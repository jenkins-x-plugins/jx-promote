#!/bin/bash

echo "promoting the new version ${VERSION} to downstream repositories"

jx step create pr regex --regex "const JXPromoteVersion = \"(?P<version>.*)\"" --version ${VERSION} --files cmd/enhance-pipeline/cmd/enhancePipeline.go --repo hhttps://github.com/cloudbees/jx-app-cb-remote.git

jx step create pr go --name github.com/jenkins-x/jx-promote --version ${VERSION} --build "make build" --repo https://github.com/jenkins-x/jxl.git
