// Copyright (c) 2019,2021 Tigera, Inc. All rights reserved.

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package fv_test

import (
	"context"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	. "github.com/onsi/gomega"

	log "github.com/sirupsen/logrus"

	v3 "github.com/projectcalico/api/pkg/apis/projectcalico/v3"
	. "github.com/projectcalico/calicoctl/v3/tests/fv/utils"
	"github.com/projectcalico/libcalico-go/lib/apiconfig"
	libapi "github.com/projectcalico/libcalico-go/lib/apis/v3"
	"github.com/projectcalico/libcalico-go/lib/clientv3"
	"github.com/projectcalico/libcalico-go/lib/ipam"
	"github.com/projectcalico/libcalico-go/lib/logutils"
	cnet "github.com/projectcalico/libcalico-go/lib/net"
	"github.com/projectcalico/libcalico-go/lib/options"
)

func init() {
	log.AddHook(logutils.ContextHook{})
	log.SetFormatter(&logutils.Formatter{})
}

func TestIPAM(t *testing.T) {
	RegisterTestingT(t)

	ctx := context.Background()

	// Create a Calico client.
	config := apiconfig.NewCalicoAPIConfig()
	config.Spec.DatastoreType = "etcdv3"
	config.Spec.EtcdEndpoints = "http://127.0.0.1:2379"
	client, err := clientv3.New(*config)
	Expect(err).NotTo(HaveOccurred())

	// Create an IPv4 pool.
	pool := v3.NewIPPool()
	pool.Name = "ipam-test-v4"
	pool.Spec.CIDR = "10.65.0.0/16"
	_, err = client.IPPools().Create(ctx, pool, options.SetOptions{})
	Expect(err).NotTo(HaveOccurred())

	// Create an IPv6 pool.
	pool = v3.NewIPPool()
	pool.Name = "ipam-test-v6"
	pool.Spec.CIDR = "fd5f:abcd:64::0/48"
	_, err = client.IPPools().Create(ctx, pool, options.SetOptions{})
	Expect(err).NotTo(HaveOccurred())

	// Create a Node resource for this host.
	node := libapi.NewNode()
	node.Name, err = os.Hostname()
	Expect(err).NotTo(HaveOccurred())
	_, err = client.Nodes().Create(ctx, node, options.SetOptions{})
	Expect(err).NotTo(HaveOccurred())

	// ipam show with specific unallocated IP.
	out := Calicoctl(false, "ipam", "show", "--ip=10.65.0.2")
	Expect(out).To(ContainSubstring("10.65.0.2 is not assigned"))

	// ipam show, with no allocations yet.
	out = Calicoctl(false, "ipam", "show")
	Expect(out).To(ContainSubstring("IPS IN USE"))

	// Assign some IPs.
	v4, v6, err := client.IPAM().AutoAssign(ctx, ipam.AutoAssignArgs{
		Num4:  5,
		Num6:  7,
		Attrs: map[string]string{"note": "reserved by ipam_test.go"},
	})
	Expect(err).NotTo(HaveOccurred())

	// ipam show, pools only.
	out = Calicoctl(false, "ipam", "show")
	Expect(out).To(ContainSubstring("IPS IN USE"))
	Expect(out).To(ContainSubstring("10.65.0.0/16"))
	Expect(out).To(ContainSubstring("5 (0%)"))
	Expect(out).To(ContainSubstring("65531 (100%)"))
	Expect(out).To(ContainSubstring("fd5f:abcd:64::/48"))

	// ipam show, including blocks.
	out = Calicoctl(false, "ipam", "show", "--show-blocks")
	Expect(out).To(ContainSubstring("Block"))
	Expect(out).To(ContainSubstring("5 (8%)"))
	Expect(out).To(ContainSubstring("59 (92%)"))

	// Find out the allocation block.
	var allocatedIP string
	r, err := regexp.Compile(`(10\.65\.[0-9]+\.)([0-9]+)/26`)
	Expect(err).NotTo(HaveOccurred())
	for _, line := range strings.Split(out, "\n") {
		sm := r.FindStringSubmatch(line)
		if len(sm) > 0 {
			ordinalBase, err := strconv.Atoi(sm[2])
			Expect(err).NotTo(HaveOccurred())
			allocatedIP = sm[1] + strconv.Itoa(ordinalBase+2)
			break
		}
	}
	Expect(allocatedIP).NotTo(BeEmpty())

	// ipam show with specific IP that is now allocated.
	out = Calicoctl(false, "ipam", "show", "--ip="+allocatedIP)
	Expect(out).To(ContainSubstring(allocatedIP + " is in use"))
	Expect(out).To(ContainSubstring("Attributes:"))
	Expect(out).To(ContainSubstring("note: reserved by ipam_test.go"))

	// ipam show with an invalid IP.
	out, err = CalicoctlMayFail(false, "ipam", "show", "--ip=10.240.0.300")
	Expect(err).To(HaveOccurred())
	Expect(out).To(ContainSubstring("invalid IP address"))

	// Create a pool with blocksize 29, so we can easily allocate
	// an entire block.
	pool = v3.NewIPPool()
	pool.Name = "ipam-test-v4-b29"
	pool.Spec.CIDR = "10.66.0.0/16"
	pool.Spec.BlockSize = 29
	_, err = client.IPPools().Create(ctx, pool, options.SetOptions{})
	Expect(err).NotTo(HaveOccurred())

	// Allocate more than one block's worth (8) of IPs from that
	// pool.
	// Assign some IPs.
	v4More, v6More, err := client.IPAM().AutoAssign(ctx, ipam.AutoAssignArgs{
		Num4:      11,
		IPv4Pools: []cnet.IPNet{cnet.MustParseNetwork(pool.Spec.CIDR)},
	})
	Expect(err).NotTo(HaveOccurred())

	// ipam show, including blocks.
	//
	// Example output here:
	// +----------+-------------------------------------------+------------+------------+-------------------+
	// | GROUPING |                   CIDR                    | IPS TOTAL  | IPS IN USE |     IPS FREE      |
	// +----------+-------------------------------------------+------------+------------+-------------------+
	// | IP Pool  | 10.65.0.0/16                              |      65536 | 5 (0%)     | 65531 (100%)      |
	// | Block    | 10.65.79.0/26                             |         64 | 5 (8%)     | 59 (92%)          |
	// | IP Pool  | 10.66.0.0/16                              |      65536 | 11 (0%)    | 65525 (100%)      |
	// | Block    | 10.66.137.224/29                          |          8 | 8 (100%)   | 0 (0%)            |
	// | Block    | 10.66.137.232/29                          |          8 | 3 (38%)    | 5 (62%)           |
	// | IP Pool  | fd5f:abcd:64::/48                         | 1.2089e+24 | 7 (0%)     | 1.2089e+24 (100%) |
	// | Block    | fd5f:abcd:64:4f2c:ec1b:27b9:1989:77c0/122 |         64 | 7 (11%)    | 57 (89%)          |
	// +----------+-------------------------------------------+------------+------------+-------------------+
	outLines := strings.Split(Calicoctl(false, "ipam", "show", "--show-blocks"), "\n")
	Expect(outLines).To(ContainElement(And(ContainSubstring("Block"), ContainSubstring("10.66"), ContainSubstring("8 (100%)"), ContainSubstring("0 (0%)"))))
	Expect(outLines).To(ContainElement(And(ContainSubstring("IP Pool"), ContainSubstring("fd5f"), ContainSubstring("7 (0%)"))))

	// Clean up resources
	cidrs := append(v4, v4More...)
	cidrs = append(cidrs, v6...)
	cidrs = append(cidrs, v6More...)
	nodename, err := os.Hostname()
	Expect(err).NotTo(HaveOccurred())
	var ips []cnet.IP
	for _, cidr := range cidrs {
		err = client.IPAM().ReleaseAffinity(ctx, cidr, nodename, false)
		Expect(err).NotTo(HaveOccurred())
		ip := cnet.ParseIP(cidr.IP.String())
		ips = append(ips, *ip)
	}
	// Release the IPs
	_, err = client.IPAM().ReleaseIPs(ctx, ips)
	Expect(err).NotTo(HaveOccurred())

	_, err = client.IPPools().Delete(ctx, "ipam-test-v4", options.DeleteOptions{})
	Expect(err).NotTo(HaveOccurred())
	_, err = client.IPPools().Delete(ctx, "ipam-test-v6", options.DeleteOptions{})
	Expect(err).NotTo(HaveOccurred())
	_, err = client.Nodes().Delete(ctx, nodename, options.DeleteOptions{})
	Expect(err).NotTo(HaveOccurred())
	_, err = client.IPPools().Delete(ctx, "ipam-test-v4-b29", options.DeleteOptions{})
	Expect(err).NotTo(HaveOccurred())
}
