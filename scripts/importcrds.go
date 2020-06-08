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

package main

import (
	"io/ioutil"
	"os"
	"strconv"
	"strings"
)

const (
	crdPrefix  = "crd.projectcalico.org_"
	fileSuffix = ".yaml"
)

// Reads all CRD files that are downloaded from libcalico-go
// and encodes them as strings literals in calicoctl/commands/crds/crds.go
func main() {
	fs, _ := ioutil.ReadDir("../../../config/crd")
	out, _ := os.Create("../../commands/crds/crds.go")
	out.Write([]byte(`// Copyright (c) 2020 Tigera, Inc. All rights reserved.

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

`))
	out.Write([]byte("package crds \n\n//DO NOT CHANGE. This is a generated file. In order to update, run `make gen-crds`.\n\nconst (\n"))
	for _, f := range fs {
		if strings.HasSuffix(f.Name(), fileSuffix) && strings.HasPrefix(f.Name(), crdPrefix) {
			fname := strings.TrimPrefix(f.Name(), crdPrefix)
			name := strings.TrimSuffix(fname, fileSuffix)
			out.Write([]byte("\t" + name + " = "))
			b, _ := ioutil.ReadFile("../../../config/crd/" + f.Name())
			fstr := strconv.Quote(string(b))
			out.Write([]byte(fstr))
			out.Write([]byte("\n"))
		}
	}
	out.Write([]byte(")\n"))
}
