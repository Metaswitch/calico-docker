// Copyright (c) 2016 Tigera, Inc. All rights reserved.

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

package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/docopt/docopt-go"
	"github.com/projectcalico/calico-containers/calicoctl/commands/constants"
	"github.com/projectcalico/calico-containers/calicoctl/commands/node"
)

// Node function is a switch to node related sub-commands
func Node(args []string) error {
	var err error
	doc := constants.DatastoreIntro + `Usage:
  calicoctl node <command> [<args>...]

    status       View the current status of a Calico node.
    diags        Gather a diagnostics bundle for a Calico node.
    checksystem  Verify the compute host is able to run a Calico node instance.

Options:
  -h --help      Show this screen.

Description:
  Node specific commands for calicoctl.  These commands must be run directly on
  the compute host running the Calico node instance.
  
  See 'calicoctl node <command> --help' to read about a specific subcommand.
`
	arguments, err := docopt.Parse(doc, args, true, "", true, false)
	if err != nil {
		return err
	}
	if arguments["<command>"] == nil {
		return nil
	}

	command := arguments["<command>"].(string)
	args = append([]string{"node", command}, arguments["<args>"].([]string)...)

	switch command {
	case "status":
		err = node.Status(args)
	case "diags":
		err = node.Diags(args)
	case "checksystem":
		err = node.Checksystem(args)
	default:
		fmt.Println(doc)
	}

	if err != nil {
		fmt.Printf("Error executing command. Invalid option: 'calicoctl %s'. Use flag '--help' to read about a specific subcommand.\n", strings.Join(args, " "))
		os.Exit(1)
	}

	return nil
}
