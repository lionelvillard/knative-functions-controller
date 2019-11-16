#!/usr/bin/env bash

# Copyright 2019 The Knative Authors
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

ROOT=$(dirname $BASH_SOURCE[0])/..
source $ROOT/hack/lib/library.sh

TAG=$1
export KO_DOCKER_REPO=docker.io/knativefunctions
docker login -p $DOCKER_PASS -u $DOCKER_USER docker.io

cd $ROOT
mkdir -p release
ko resolve -f config -t $TAG > release/function.yaml

github::create_release $GITHUB_TOKEN lionelvillard/knative-functions-controller $TAG
github::upload_asset $GITHUB_TOKEN lionelvillard/knative-functions-controller $TAG release/function.yaml

rm -rf release