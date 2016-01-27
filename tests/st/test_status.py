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

from test_base import TestBase
from tests.st.utils.docker_host import DockerHost
from tests.st.utils.exceptions import CommandExecError

"""
Test calicoctl status

Most of the status output is checked by the BGP tests, so this module just
contains a simple return code check.
"""


class TestStatus(TestBase):
    def test_status(self):
        """
        Test that the status command can be executed.
        """
        with DockerHost('host', dind=False, start_calico=True) as host:
            host.calicoctl("status")

    def test_status_fails(self):
        """
        Test that the status command fails when calico node is not running
        """
        with DockerHost('host', dind=False, start_calico=False) as host:
            try:
                host.calicoctl("status")
            except CommandExecError as e:
                self.assertEquals(e.returncode, 1)
                self.assertEquals(e.output,
                                  "calico-node container not running\n")
            else:
                raise AssertionError("'calicoctl status' did not exit with "
                                     "code 1 when node was not running")
