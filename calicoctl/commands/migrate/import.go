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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/docopt/docopt-go"
	log "github.com/sirupsen/logrus"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/projectcalico/calicoctl/calicoctl/commands/clientmgr"
	"github.com/projectcalico/calicoctl/calicoctl/commands/common"
	"github.com/projectcalico/calicoctl/calicoctl/commands/constants"
	"github.com/projectcalico/calicoctl/calicoctl/commands/crds"
	"github.com/projectcalico/libcalico-go/lib/apiconfig"
	apiv3 "github.com/projectcalico/libcalico-go/lib/apis/v3"
	"github.com/projectcalico/libcalico-go/lib/backend/k8s"
	client "github.com/projectcalico/libcalico-go/lib/clientv3"
	"github.com/projectcalico/libcalico-go/lib/options"
)

func Import(args []string) error {
	doc := `Usage:
  calicoctl datastore migrate import --filename=<FILENAME> [--config=<CONFIG>]

Options:
  -h --help                 Show this screen.
  -f --filename=<FILENAME>  Filename to use to import resources.  If set to
                            "-" loads from stdin.
  -c --config=<CONFIG>      Path to the file containing connection
                            configuration in YAML or JSON format.
                            [default: ` + constants.DefaultConfigPath + `]

Description:
  Import the contents of the etcdv3 datastore from the file created by the
  export command.
`

	parsedArgs, err := docopt.Parse(doc, args, true, "", false, false)
	if err != nil {
		return fmt.Errorf("Invalid option: 'calicoctl %s'. Use flag '--help' to read about a specific subcommand.", strings.Join(args, " "))
	}
	if len(parsedArgs) == 0 {
		return nil
	}

	// Get the backend client for updating cluster info and migrating IPAM.
	cf := parsedArgs["--config"].(string)
	client, err := clientmgr.NewClient(cf)
	if err != nil {
		return err
	}

	// Check that the datastore configured datastore is kubernetes
	cfg, err := clientmgr.LoadClientConfig(cf)
	if err != nil {
		log.Info("Error loading config")
		return err
	}

	if cfg.Spec.DatastoreType != apiconfig.Kubernetes {
		return fmt.Errorf("Invalid datastore type: %s to import to for datastore migration. Datastore type must be kubernetes", cfg.Spec.DatastoreType)
	}

	err = importCRDs(cfg)
	if err != nil {
		return fmt.Errorf("Error applying the CRDs necessary to begin datastore import: %s", err)
	}

	err = checkCalicoResourcesNotExist(parsedArgs, client)
	if err != nil {
		// TODO: Add something like 'calicoctl datastore migrate clean' to delete all the CRDs to wipe out the Calico resources.
		return fmt.Errorf("Datastore already has Calico resources: %s. Clear out all Calico resources by deleting all Calico CRDs.", err)
	}

	// Ensure that the cluster info resource is initialized.
	ctx := context.Background()
	if err := client.EnsureInitialized(ctx, "", ""); err != nil {
		return fmt.Errorf("Unable to initialize cluster information for the datastore migration: %s", err)
	}

	// Make sure that the datastore is locked. Since the call to EnsureInitialized
	// should initialize it to unlocked, lock it before we continue.
	locked, err := checkLocked(ctx, client)
	if err != nil {
		return fmt.Errorf("Error while checking if datastore was locked: %s", err)
	} else if !locked {
		err := Lock([]string{"datastore", "migrate", "lock", "-c", cf})
		if err != nil {
			return fmt.Errorf("Error while attempting to lock the datastore for import: %s", err)
		}
	}

	// Split file into v3 API, ClusterGUID, and IPAM components
	filename := parsedArgs["--filename"].(string)
	v3Yaml, clusterInfoJson, ipamJson, err := splitImportFile(filename)
	if err != nil {
		return fmt.Errorf("Error while reading migration file: %s\n", err)
	}

	// Apply v3 API resources
	err = updateV3Resources(cf, v3Yaml)
	if err != nil {
		return fmt.Errorf("Failed to import v3 resources: %s\n", err)
	}

	// Update the clusterinfo resource with the data from the old datastore.
	err = updateClusterInfo(ctx, client, clusterInfoJson)
	if err != nil {
		return fmt.Errorf("Failed to update cluster information: %s", err)
	}

	// Import IPAM components
	fmt.Print("Importing IPAM resources\n")
	ipam := NewMigrateIPAM(client)
	err = json.Unmarshal(ipamJson, ipam)
	if err != nil {
		return fmt.Errorf("Failed to read IPAM resources: %s\n", err)
	}
	results := ipam.PushToDatastore()

	// Handle the IPAM results
	if results.numHandled == 0 {
		if results.numResources == 0 {
			return fmt.Errorf("No IPAM resources specified in file")
		} else {
			return fmt.Errorf("Failed to import any IPAM resources: %v", results.resErrs)
		}
	} else if len(results.resErrs) == 0 {
		fmt.Printf("Successfully applied %d IPAM resource(s)\n", results.numHandled)
	} else {
		if results.numHandled != 0 && len(results.resErrs) > 0 {
			fmt.Printf("Partial success: ")
			fmt.Printf("applied the first %d out of %d resources:\n", results.numHandled, results.numResources)
		}
		return fmt.Errorf("Hit error(s): %v", results.resErrs)
	}

	fmt.Print("Datastore information successfully imported. Please refer to the datastore migration documentation for next steps.\n")

	return nil
}

func splitImportFile(filename string) ([]byte, []byte, []byte, error) {
	// Get the appropriate file to read from
	fname := filename
	if filename == "-" {
		fname = os.Stdin.Name()
	}

	b, err := ioutil.ReadFile(fname)
	if err != nil {
		return nil, nil, nil, err
	}

	split := bytes.Split(b, []byte("===\n"))
	if len(split) != 3 {
		return nil, nil, nil, fmt.Errorf("Imported file: %s is improperly formatted. Try recreating with 'calicoctl export'", fname)
	}

	// First chunk should be the v3 resource YAML.
	// Second chunk should give the cluster info resource.
	// Last chunk should be the IPAM JSON.
	return split[0], split[1], split[2], nil
}

func checkCalicoResourcesNotExist(args map[string]interface{}, c client.Interface) error {
	// Loop through all the v3 resources to see if anything is returned
	extendedV3Resources := append(allV3Resources, "clusterinfo")
	for _, r := range extendedV3Resources {
		// Skip nodes since they are backed by the Kubernetes node resource
		if r == "nodes" {
			continue
		}

		// Create mocked args in order to retrieve Get resources.
		mockArgs := map[string]interface{}{
			"<KIND>":   r,
			"<NAME>":   []string{},
			"--config": args["--config"].(string),
			"--export": false,
			"--output": "ps",
			"get":      true,
		}

		if _, ok := namespacedResources[r]; ok {
			mockArgs["--all-namespaces"] = true
		}

		// Get resources
		results := common.ExecuteConfigCommand(mockArgs, common.ActionGetOrList)

		// Loop through the result lists and see if anything exists
		for _, resource := range results.Resources {
			if meta.LenList(resource) > 0 {
				return fmt.Errorf("Found existing Calico %s resource", results.SingleKind)
			}

			if results.FileInvalid {
				return fmt.Errorf("Failed to execute command: %v", results.Err)
			} else if results.Err != nil {
				return fmt.Errorf("Failed to retrieve %s resources during datastore check: %v", resourceDisplayMap[r], results.Err)
			}
		}
	}

	// Check if any IPAM resources exist
	ipam := NewMigrateIPAM(c)
	err := ipam.PullFromDatastore()
	if err != nil {
		return fmt.Errorf("Failed to retrieve IPAM resources during datastore check: %s", err)
	}

	if !ipam.IsEmpty() {
		return fmt.Errorf("Found existing IPAM resources")
	}

	return nil
}

func updateClusterInfo(ctx context.Context, c client.Interface, clusterInfoJson []byte) error {
	// Unmarshal the etcd cluster info resource.
	migrated := apiv3.ClusterInformation{}
	err := json.Unmarshal(clusterInfoJson, &migrated)
	if err != nil {
		return fmt.Errorf("Error reading exported cluster info for migration: %s", err)
	}

	// Get the "default" cluster info resource.
	clusterinfo, err := c.ClusterInformation().Get(ctx, "default", options.GetOptions{})
	if err != nil {
		return fmt.Errorf("Error retrieving current cluster info for migration: %s", err)
	}

	// Update the calico version and cluster GUID.
	clusterinfo.Spec.ClusterGUID = migrated.Spec.ClusterGUID
	clusterinfo.Spec.CalicoVersion = migrated.Spec.CalicoVersion
	_, err = c.ClusterInformation().Update(ctx, clusterinfo, options.SetOptions{})
	if err != nil {
		return fmt.Errorf("Error updating current cluster info for migration: %s", err)
	}

	return nil
}

func updateV3Resources(cf string, data []byte) error {
	// Create tempfile so the v3 resources can be created using Apply
	tempfile, err := ioutil.TempFile("", "v3migration")
	if err != nil {
		return fmt.Errorf("Error while creating temporary v3 migration file: %s\n", err)
	}
	defer os.Remove(tempfile.Name())

	if _, err := tempfile.Write(data); err != nil {
		return fmt.Errorf("Error while writing to temporary v3 migration file: %s\n", err)
	}

	mockArgs := map[string]interface{}{
		"--config":   cf,
		"--filename": tempfile.Name(),
		"apply":      true,
	}
	err = applyV3(mockArgs)
	if err != nil {
		return fmt.Errorf("Failed to import v3 resources: %s\n", err)
	}

	return nil
}

func importCRDs(cfg *apiconfig.CalicoAPIConfig) error {
	// Start a kube client
	// Create the correct config for the clientset
	config, _, err := k8s.CreateKubernetesClientset(&cfg.Spec)
	if err != nil {
		return err
	}

	// Create the apiextensions clientset
	cs, err := clientset.NewForConfig(config)
	if err != nil {
		return err
	}
	log.Debugf("Created k8s CRD ClientSet: %+v", cs)

	// Apply the CRDs
	for _, crd := range crds.CalicoCRDs {
		_, err := cs.ApiextensionsV1beta1().CustomResourceDefinitions().Create(crd)
		if err != nil {
			if kerrors.IsAlreadyExists(err) {
				// If the CRD already exists attempt to update it.
				// Need to retrieve the current CRD first.
				currentCRD, err := cs.ApiextensionsV1beta1().CustomResourceDefinitions().Get(crd.GetObjectMeta().GetName(), v1.GetOptions{})
				if err != nil {
					return fmt.Errorf("Error retrieving existing CRD to update: %s: %s", crd.GetObjectMeta().GetName(), err)
				}

				// Use the resource version so that the current CRD can be overwritten.
				crd.GetObjectMeta().SetResourceVersion(currentCRD.GetObjectMeta().GetResourceVersion())

				// Update the CRD.
				_, err = cs.ApiextensionsV1beta1().CustomResourceDefinitions().Update(crd)
				if err != nil {
					return fmt.Errorf("Error updating CRD %s: %s", crd.GetObjectMeta().GetName(), err)
				}
			} else {
				return fmt.Errorf("Error creating CRD %s: %s", crd.GetObjectMeta().GetName(), err)
			}
		}
		log.Debugf("Applied %s CRD", crd.GetObjectMeta().GetName())
	}

	return nil
}

func applyV3(args map[string]interface{}) error {
	results := common.ExecuteConfigCommand(args, common.ActionApply)
	log.Infof("results: %+v", results)

	if results.FileInvalid {
		return fmt.Errorf("Failed to execute command: %v", results.Err)
	} else if results.NumHandled == 0 {
		if results.NumResources == 0 {
			return fmt.Errorf("No resources specified in file")
		} else if results.NumResources == 1 {
			return fmt.Errorf("Failed to apply '%s' resource: %v", results.SingleKind, results.ResErrs)
		} else if results.SingleKind != "" {
			return fmt.Errorf("Failed to apply any '%s' resources: %v", results.SingleKind, results.ResErrs)
		} else {
			return fmt.Errorf("Failed to apply any resources: %v", results.ResErrs)
		}
	} else if len(results.ResErrs) == 0 {
		if results.SingleKind != "" {
			fmt.Printf("Successfully applied %d '%s' resource(s)\n", results.NumHandled, results.SingleKind)
		} else {
			fmt.Printf("Successfully applied %d resource(s)\n", results.NumHandled)
		}
	} else {
		if results.NumHandled-len(results.ResErrs) > 0 {
			fmt.Printf("Partial success: ")
			if results.SingleKind != "" {
				fmt.Printf("applied the first %d out of %d '%s' resources:\n",
					results.NumHandled, results.NumResources, results.SingleKind)
			} else {
				fmt.Printf("applied the first %d out of %d resources:\n",
					results.NumHandled, results.NumResources)
			}
		}
		return fmt.Errorf("Hit error(s): %v", results.ResErrs)
	}

	return nil
}
