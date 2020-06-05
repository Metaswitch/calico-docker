// Copyright (c) 2020 Tigera, Inc. All rights reserved.

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

package migrate

import (
	"context"
	"fmt"
	"strings"

	"github.com/docopt/docopt-go"

	"github.com/projectcalico/calicoctl/calicoctl/commands/clientmgr"
	"github.com/projectcalico/calicoctl/calicoctl/commands/constants"
	"github.com/projectcalico/libcalico-go/lib/clientv3"
	"github.com/projectcalico/libcalico-go/lib/options"
)

func Lock(args []string) error {
	doc := `Usage:
  calicoctl datastore migrate lock [--config=<CONFIG>]

Options:
  -h --help                 Show this screen.
  -c --config=<CONFIG>      Path to the file containing connection
                            configuration in YAML or JSON format.
                            [default: ` + constants.DefaultConfigPath + `]

Description:
  Lock the datastore to prepare it for migration. This prevents any new
  Calico resources from affecting the cluster but does not prevent updating
  or creating new Calico resources.
`

	parsedArgs, err := docopt.Parse(doc, args, true, "", false, false)
	if err != nil {
		return fmt.Errorf("Invalid option: 'calicoctl %s'. Use flag '--help' to read about a specific subcommand.", strings.Join(args, " "))
	}
	if len(parsedArgs) == 0 {
		return nil
	}

	cf := parsedArgs["--config"].(string)
	client, err := clientmgr.NewClient(cf)
	if err != nil {
		return err
	}

	// Ensure that the cluster information resource is initialized.
	ctx := context.Background()
	if err := client.EnsureInitialized(ctx, "", ""); err != nil {
		return fmt.Errorf("Unable to initialize cluster information for the datastore migration: %s", err)
	}

	// Get the cluster information resource
	clusterinfo, err := client.ClusterInformation().Get(ctx, "default", options.GetOptions{})
	if err != nil {
		return fmt.Errorf("Error retrieving ClusterInformation for locking: %s", err)
	}

	// Change the Datastore to not ready in order to lock it.
	f := false
	clusterinfo.Spec.DatastoreReady = &f

	// Update the cluster information resource
	_, err = client.ClusterInformation().Update(ctx, clusterinfo, options.SetOptions{})
	if err != nil {
		return fmt.Errorf("Error updating ClusterInformation for locking: %s", err)
	}

	fmt.Print("Datastore locked.\n")
	return nil
}

func checkLocked(ctx context.Context, c clientv3.Interface) (bool, error) {
	// Get the cluster information resource
	clusterinfo, err := c.ClusterInformation().Get(ctx, "default", options.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("Error retrieving ClusterInformation: %s", err)
	}

	return !*clusterinfo.Spec.DatastoreReady, nil
}
