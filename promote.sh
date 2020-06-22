#!/bin/bash

echo "promoting the new version ${VERSION} to downstream repositories"

jx step create pr regex --regex 'const JXPromoteVersion = "(?P<version>.*)"' --version ${VERSION} --files cmd/enhance-pipeline/cmd/enhancePipeline.go --repo https://github.com/cloudbees/jx-app-cb-remote.git

jx step create pr regex --regex 'version: (.*)' --version ${VERSION} --files docker/gcr.io/jenkinsxio-labs-private/jx-promote.yml --repo https://github.com/jenkins-x/jxr-versions.git
