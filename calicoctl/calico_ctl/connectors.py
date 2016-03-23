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
import os
import sys
import docker
import docker.errors

from pycalico.ipam import IPAMClient
from pycalico.datastore import (ETCD_AUTHORITY_ENV, ETCD_AUTHORITY_DEFAULT,
                                ETCD_SCHEME_ENV, ETCD_SCHEME_DEFAULT,
                                ETCD_KEY_FILE_ENV, ETCD_CERT_FILE_ENV,
                                ETCD_CA_CERT_FILE_ENV, DataStoreError)
from utils import DOCKER_VERSION
from utils import print_paragraph
from pycalico.util import validate_hostname_port

try:
    client = IPAMClient()
except DataStoreError as e:
    print_paragraph(e.message)
    sys.exit(1)

DOCKER_URL = os.getenv("DOCKER_HOST", "unix://var/run/docker.sock")
docker_client = docker.Client(version=DOCKER_VERSION, base_url=DOCKER_URL)
