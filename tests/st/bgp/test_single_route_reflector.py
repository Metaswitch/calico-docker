# Copyright (c) 2015-2016 Tigera, Inc. All rights reserved.
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
from nose.plugins.attrib import attr

from tests.st.test_base import TestBase
from tests.st.utils.docker_host import DockerHost
from tests.st.utils.route_reflector import RouteReflectorCluster

from .peer import create_bgp_peer, ADDITIONAL_DOCKER_OPTIONS


class TestSingleRouteReflector(TestBase):

    @attr('slow')
    def test_single_route_reflector(self):
        """
        Run a multi-host test using a single route reflector and global
        peering.
        """
        with DockerHost('host1',
                        additional_docker_options=ADDITIONAL_DOCKER_OPTIONS) as host1, \
             DockerHost('host2',
                        additional_docker_options=ADDITIONAL_DOCKER_OPTIONS) as host2, \
             RouteReflectorCluster(1, 1) as rrc:

            # Set the default AS number - as this is used by the RR mesh, and
            # turn off the node-to-node mesh (do this from any host).
            host1.calicoctl("config set asNumber 64514")
            host1.calicoctl("config set nodeToNodeMesh off")

            # Create a workload on each host in the same network.
            network1 = host1.create_network("subnet1", subnet=subnet1,
                                            driver="calico",
                                            ipam_driver="calico-ipam")
            workload_host1 = host1.create_workload("workload1",
                                                   network=network1)
            workload_host2 = host2.create_workload("workload2",
                                                   network=network1)

            # Allow network to converge (which it won't)
            try:
                workload_host1.assert_can_ping(workload_host2.ip, retries=5)
            except AssertionError:
                pass
            else:
                raise AssertionError("Hosts can ping each other")

            # Set global config telling all calico nodes to peer with the
            # route reflector.  This can be run from either host.
            rg = rrc.get_redundancy_group()
            assert len(rg) == 1
            create_bgp_peer(host1, "global", rg[0].ip, 64514)

            # Allow network to converge (which it now will).
            workload_host1.assert_can_ping(workload_host2.ip, retries=10)

            # And check connectivity in both directions.
            self.assert_ip_connectivity(workload_list=[workload_host1,
                                                       workload_host2],
                                        ip_pass_list=[workload_host1.ip,
                                                      workload_host2.ip])
