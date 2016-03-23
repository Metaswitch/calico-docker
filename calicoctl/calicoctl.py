#!/usr/bin/env python

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

# Two usage strings - 1 for linux and 1 for other OSes (e.g. Mac and Windows) which can't run the full set of commands.

linux_usage = """Override the host:port of the ETCD server by setting the environment variable:
ETCD_ENDPOINTS [example: http://1.2.3.4:2379,http://1.2.3.5:2379]
or, if ETCD_ENDPOINTS is left unset:
ETCD_AUTHORITY [default: 127.0.0.1:2379]

Usage: calicoctl <command> [<args>...]

    status            Print current status information
    node              Configure the main calico/node container and establish Calico networking
    container         Configure containers and their addresses
    profile           Configure endpoint profiles
    endpoint          Configure the endpoints assigned to existing containers
    pool              Configure ip-pools
    bgp               Configure global bgp
    ipam              Configure IP address management
    checksystem       Check for incompatibilities on the host system
    diags             Save diagnostic information
    version           Display the version of calicoctl
    config            Configure low-level component configuration

See 'calicoctl <command> --help' to read about a specific subcommand.
"""

otheros_usage = """Override the host:port of the ETCD server by setting the environment variable:
ETCD_ENDPOINTS [example: http://1.2.3.4:2379,http://1.2.3.5:2379]
or, if ETCD_ENDPOINTS is left unset:
ETCD_AUTHORITY [default: 127.0.0.1:2379]

Usage: calicoctl <command> [<args>...]

    profile           Configure endpoint profiles
    endpoint          Configure the endpoints assigned to existing containers
    pool              Configure ip-pools
    bgp               Configure global bgp
    ipam              Configure IP address management
    version           Display the version of calicoctl
    config            Configure low-level component configuration

See 'calicoctl <command> --help' to read about a specific subcommand.
"""

import signal
import sys
import traceback
from sys import platform as _platform

from docopt import docopt
from pycalico.datastore_errors import DataStoreError

import calico_ctl.bgp
import calico_ctl.checksystem
import calico_ctl.config
import calico_ctl.container
import calico_ctl.diags
import calico_ctl.endpoint
import calico_ctl.ipam
import calico_ctl.node
import calico_ctl.pool
import calico_ctl.profile
import calico_ctl.status
import calico_ctl.version
from calico_ctl.utils import print_paragraph


def keyboard_interrupt_handler(signal, frame):
    print('Aborted command.')
    sys.exit(0)

signal.signal(signal.SIGINT, keyboard_interrupt_handler)


if __name__ == '__main__':
    """
    Calicoctl interprets the first sys.argv (after the file name) as a submodule.
    Calicoctl works on the assumption that each subcommand will have a python
    file sharing its name in the calico_ctl/ directory. This file should
    also have a function sharing that same name which accepts a single
    argument - a docopt processed input dictionary.

    Example:
      calico_ctl/node.py has a function called node(arguments)
    """
    if _platform == "linux" or _platform == "linux2":
        docstring = linux_usage
    else:
        docstring = otheros_usage

    # If no arguments were provided in the function call, add the help flag
    # to trigger the main help message
    if len(sys.argv) == 1:
        docopt(docstring, options_first=True, argv=['--help'])
        sys.exit(1)

    # Run command through initial docopt processing to determine subcommand
    command_args = docopt(docstring, options_first=True)

    # Group the additional args together and forward them along
    argv = [command_args['<command>']] + command_args['<args>']

    # Dispatch the appropriate subcommand
    try:
        try:
            command = command_args['<command>']

            # Look for a python file in the calico_ctl module which
            # shares the same name as the input command
            command_module = getattr(calico_ctl, command)

            # The command module should have a function the same name as the
            # command.  This protects against trying to run an invalid command that
            # happens to have the same name as a non-command module.
            if not hasattr(command_module, command):
                docopt(docstring, options_first=True, argv=['--help'])
                sys.exit(1)

            # docopt the arguments through that module's docstring
            arguments = docopt(command_module.__doc__, argv=argv)

            # Call the dispatch function in that module which should also have
            # the same name
            cmd = getattr(command_module, command)
        except AttributeError:
            # Unrecognized submodule. Show main help message
            docopt(docstring, options_first=True, argv=['--help'])
            sys.exit(1)
        else:
            cmd(arguments)
    except SystemExit:
        raise
    except DataStoreError as e:
        print_paragraph(e.message)
        sys.exit(1)
    except BaseException as e:
        print "{0}: {1}\n".format(type(e).__name__, e)
        traceback.print_exc()
        sys.exit(1)
