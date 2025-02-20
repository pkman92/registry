#!/bin/bash
#
# Copyright 2020 Google LLC. All Rights Reserved.
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

echo This walkthrough script demonstrates key registry operations that can be performed
echo through the API or using the automatically-generated apg command-line tool.

if ! type "jq" > /dev/null; then
  echo
  echo "Error: this script requires jq (https://stedolan.github.io/jq/)"
  exit 1
fi

echo
echo Delete everything associated with any preexisting project named "demo".
apg admin delete-project --name projects/demo

echo
echo Create a project in the registry named "demo".
apg admin create-project --project_id demo --json

echo
echo Add a API to the registry.
apg registry create-api \
    --parent projects/demo/locations/global \
    --api_id petstore \
    --api.availability GENERAL \
    --api.recommended_version "1.0.0" \
    --json

echo
echo Add a version of the API to the registry.
apg registry create-api-version \
    --parent projects/demo/locations/global/apis/petstore \
    --api_version_id 1.0.0 \
    --api_version.state "PRODUCTION" \
    --json

echo
echo Add a spec for the API version that we just added to the registry.
apg registry create-api-spec \
    --parent projects/demo/locations/global/apis/petstore/versions/1.0.0 \
    --api_spec_id openapi.yaml \
    --api_spec.contents `registry-encode-spec < testdata/openapi.yaml@r0` \
    --json

echo
echo Get the API spec.
apg registry get-api-spec \
    --name projects/demo/locations/global/apis/petstore/versions/1.0.0/specs/openapi.yaml \
    --json

echo
echo Get the contents of the API spec.
apg registry get-api-spec-contents \
    --name projects/demo/locations/global/apis/petstore/versions/1.0.0/specs/openapi.yaml \
    --json | \
    jq '.data' -r | \
    registry-decode-spec

echo
echo Update an attribute of the spec.
apg registry update-api-spec \
	--api_spec.name projects/demo/locations/global/apis/petstore/versions/1.0.0/specs/openapi.yaml \
	--api_spec.mime_type "application/x.openapi+gzip;version=3" \
    --json

echo
echo Get the modifed API spec.
apg registry get-api-spec \
    --name projects/demo/locations/global/apis/petstore/versions/1.0.0/specs/openapi.yaml \
    --json

echo
echo Update the spec to new contents.
apg registry update-api-spec \
	--api_spec.name projects/demo/locations/global/apis/petstore/versions/1.0.0/specs/openapi.yaml \
	--api_spec.contents `registry-encode-spec < testdata/openapi.yaml@r1` \
    --json

echo
echo Again update the spec to new contents.
apg registry update-api-spec \
	--api_spec.name projects/demo/locations/global/apis/petstore/versions/1.0.0/specs/openapi.yaml \
	--api_spec.contents `registry-encode-spec < testdata/openapi.yaml@r2` \
    --json

echo
echo Make a third update of the spec contents.
apg registry update-api-spec \
	--api_spec.name projects/demo/locations/global/apis/petstore/versions/1.0.0/specs/openapi.yaml \
	--api_spec.contents `registry-encode-spec < testdata/openapi.yaml@r3`

echo
echo Get the API spec.
apg registry get-api-spec \
    --name projects/demo/locations/global/apis/petstore/versions/1.0.0/specs/openapi.yaml \
    --json

echo
echo List the revisions of the spec.
apg registry list-api-spec-revisions \
    --name projects/demo/locations/global/apis/petstore/versions/1.0.0/specs/openapi.yaml \
    --json

echo
echo List just the names of the revisions of the spec.
apg registry list-api-spec-revisions \
    --name projects/demo/locations/global/apis/petstore/versions/1.0.0/specs/openapi.yaml \
    --json | \
    jq '.apiSpecs[].name' -r 

echo
echo Get the latest revision of the spec.
apg registry list-api-spec-revisions \
    --name projects/demo/locations/global/apis/petstore/versions/1.0.0/specs/openapi.yaml \
    --json | \
    jq '.apiSpecs[0].name' -r 

echo
echo Get the oldest revision of the spec.
apg registry list-api-spec-revisions \
    --name projects/demo/locations/global/apis/petstore/versions/1.0.0/specs/openapi.yaml \
    --json | \
    jq '.apiSpecs[-1].name' -r 

ORIGINAL=`apg registry list-api-spec-revisions \
    --name projects/demo/locations/global/apis/petstore/versions/1.0.0/specs/openapi.yaml \
    --json | \
    jq '.apiSpecs[-1].name' -r`

ORIGINAL_HASH=`apg registry list-api-spec-revisions \
    --name projects/demo/locations/global/apis/petstore/versions/1.0.0/specs/openapi.yaml \
    --json | \
    jq '.apiSpecs[-1].hash' -r`

echo
echo Tag a spec revision.
apg registry tag-api-spec-revision --name $ORIGINAL --tag og --json

echo
echo Get a spec by its tag.
apg registry get-api-spec \
    --name projects/demo/locations/global/apis/petstore/versions/1.0.0/specs/openapi.yaml@og \
    --json

echo
echo Print the hash of the current spec revision.
apg registry get-api-spec \
    --name projects/demo/locations/global/apis/petstore/versions/1.0.0/specs/openapi.yaml \
    --json | \
    jq '.hash' -r

echo
echo Rollback to a prior spec revision.
apg registry rollback-api-spec \
    --name projects/demo/locations/global/apis/petstore/versions/1.0.0/specs/openapi.yaml \
    --revision_id og \
    --json

echo
echo Print the hash of the current spec revision after the rollback.
apg registry get-api-spec \
    --name projects/demo/locations/global/apis/petstore/versions/1.0.0/specs/openapi.yaml \
    --json | \
    jq '.hash' -r

echo
echo Print the original hash. 
echo $ORIGINAL_HASH

echo
echo Delete a spec revision.
apg registry delete-api-spec-revision --name $ORIGINAL

ORIGINAL2=`apg registry list-api-spec-revisions \
    --name projects/demo/locations/global/apis/petstore/versions/1.0.0/specs/openapi.yaml \
    --json | \
    jq '.specs[-1].name' -r`

echo
echo Verify that the spec has changed.
echo $ORIGINAL2 should not be $ORIGINAL

echo
echo Verify that when listing specs, we only get the current revision of each spec.
apg registry list-api-specs \
    --parent projects/demo/locations/global/apis/petstore/versions/1.0.0 \
    --json

echo
echo Set some artifacts on entities in the registry.
# the contents below is the hex-encoding of "https://github.com/OAI/OpenAPI-Specification"
apg registry create-artifact \
    --parent projects/demo/locations/global/apis/petstore \
    --artifact_id source \
    --artifact.mime_type "text/plain" \
    --artifact.contents "68747470733a2f2f6769746875622e636f6d2f4f41492f4f70656e4150492d53706563696669636174696f6e0a" \
    --json

echo
echo Export a YAML summary of the demo project.
registry export yaml projects/demo > demo.yaml
cat demo.yaml
