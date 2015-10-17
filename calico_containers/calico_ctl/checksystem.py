# Copyright 2015 Metaswitch Networks
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
"""
Usage:
  calicoctl checksystem [--fix] [--libnetwork]

Description:
  Check for incompatibilities between calico and the host system

Options:
  --fix  Allow calicoctl to attempt to correct any issues detected on the host
  --libnetwork  Check for the correct docker version for libnetwork deployments
"""
import sys
import re
import sh

import docker
from requests import ConnectionError

from utils import DOCKER_VERSION, DOCKER_LIBNETWORK_VERSION
from utils import enforce_root
from connectors import docker_client


def checksystem(arguments):
    """
    Main dispatcher for checksystem commands. Calls the corresponding helper
    function. checksystem only has one main function, so we call that function
    directly.

    :param arguments: A dictionary of arguments already processed through
    this file's docstring with docopt
    :return: None
    """
    check_system(fix=arguments["--fix"],
                 quit_if_error=True,
                 libnetwork=arguments["--libnetwork"])


def check_system(fix=False, quit_if_error=False, libnetwork=False):
    """
    Checks that the system is setup correctly. fix==True, this command will
    attempt to fix any issues it encounters. If any fixes fail, it will
    exit(1). Fix will automatically be set to True if the user specifies --fix
    at the command line.

    :param fix: if True, try to fix any system dependency issues that are
    detected.
    :param quit_if_error: if True, quit with error code 1 if any issues are
    detected, or if any fixes are unsuccesful.
    :return: True if all system dependencies are in the proper state, False if
    they are not. This function will sys.exit(1) instead of returning false if
    quit_if_error == True
    """
    # modprobe requires root privileges.
    enforce_root()

    kernel_ok = _check_kernel_modules(fix)
    docker_ok = _check_docker_version(libnetwork)

    system_ok = kernel_ok and docker_ok

    if quit_if_error and not system_ok:
        sys.exit(1)

    return system_ok


def module_loaded(module):
    """
    Checks if the specified kernel-module has been loaded.
    :param module: Name of the module to check
    :return: True if the module is loaded, False if not.
    """
    return any(s.startswith(module) for s in open("/proc/modules").readlines())


def normalize_version(version):
    """
    This function convers a string representation of a version into
    a list of integer values.
    e.g.:   "1.5.10" => [1, 5, 10]
    http://stackoverflow.com/questions/1714027/version-number-comparison
    """
    return [int(x) for x in re.sub(r'(\.0+)*$', '', version).split(".")]


def _check_kernel_modules(fix):
    """
    Check system kernel modules
    :param fix: if True, try to fix any system dependency issues that are
    detected.
    :return: True if kernel modules are ok.
    """
    modprobe = sh.Command._create('modprobe')
    system_ok = True
    try:
        ip6tables = sh.Command._create('ip6tables')
        ip6tables("-L")
    except:
        if fix:
            try:
                modprobe('ip6_tables')
            except sh.ErrorReturnCode:
                print >> sys.stderr, "ERROR: Could not enable ip6_tables."
                system_ok = False
        else:
            print >> sys.stderr, "WARNING: Unable to detect the ip6_tables " \
                                 "module. Load with `modprobe ip6_tables`"
            system_ok = False

    for module in ["xt_set"]:
        if not module_loaded(module):
            if fix:
                try:
                    modprobe(module)
                except sh.ErrorReturnCode:
                    print >> sys.stderr, "ERROR: Could not enable %s." % module
                    system_ok = False
            else:
                print >> sys.stderr, "WARNING: Unable to detect the %s " \
                                     "module. Load with `modprobe %s`" % \
                                     (module, module)
                system_ok = False
    return system_ok


def _check_docker_version(libnetwork=False):
    """
    Check the Docker version is supported.

    :return: True if Docker version is OK.
    """
    system_ok = True

    # Set correct docker version
    version = DOCKER_VERSION if not libnetwork else DOCKER_LIBNETWORK_VERSION

    # Check docker version compatability
    try:
        info = docker_client.version()
    except ConnectionError:
        print >> sys.stderr, "ERROR: Docker daemon not running."
        system_ok = False
    except docker.errors.APIError:
        print >> sys.stderr, "ERROR: Docker server must support Docker " \
                             "Remote API v%s or greater." % version
        system_ok = False
    else:
        api_version = normalize_version(info['ApiVersion'])
        # Check that API Version is above the minimum supported version
        if cmp(api_version, normalize_version(version)) < 0:
            if libnetwork:
                print >> sys.stderr, "ERROR: Docker Version does not support Libnetwork."

            print >> sys.stderr, "ERROR: Docker server must support Docker " \
                                 "Remote API v%s or greater." % version
            system_ok = False

    return system_ok
