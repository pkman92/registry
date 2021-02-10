#!/bin/sh
#
# Copyright 2021 Google LLC. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

if ! [ -x "$(command -v kubectl)" ] || ! [ -x "$(command -v gcloud)" ]; then
  echo 'ERROR: This script requires `kubectl` and `gcloud`. Please install to continue.' >&2; return
fi

# Uses deployments/gke/service.yaml to create an external load balancer if no
# service configuration file is specified.
SERVICE_CONFIG=$1
if [ -z "${SERVICE_CONFIG}" ]; then
  SERVICE_CONFIG=deployments/gke/service.yaml
fi

gcloud config set project ${REGISTRY_PROJECT_IDENTIFIER}

# Enables Kubernetes Engine API.
gcloud services enable container.googleapis.com

# Creates the cluster `registry-backend` if not exists.
if [[ $(gcloud container clusters list --zone=us-central1-a --filter name=registry-backend --uri) ]]; then
  echo "Cluster 'registry-backend' exists. "
else
  gcloud container clusters create registry-backend --zone us-central1-a --scopes=datastore,sql-admin,gke-default
fi

# Authenticates to the cluster.
gcloud container clusters get-credentials registry-backend --zone us-central1-a

# Creates a deployment.
envsubst < deployments/gke/deployment.yaml | kubectl apply -f -

# Creates a service to expose the deployment.
kubectl apply -f "${SERVICE_CONFIG}"