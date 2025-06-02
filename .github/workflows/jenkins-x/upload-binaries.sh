#!/bin/bash

echo "HOME is $HOME"
echo current git configuration

# See https://github.com/actions/checkout/issues/766
git config --global --add safe.directory "$GITHUB_WORKSPACE"

echo "setting git user"

git config --global user.name jenkins-x-bot-test
git config --global user.email "jenkins-x@googlegroups.com"

export BRANCH=$(git rev-parse --abbrev-ref HEAD)
export BUILDDATE=$(date)
export REV=$(git rev-parse HEAD)
export GOVERSION="1.22.3"
export ROOTPACKAGE="github.com/$REPOSITORY"

goreleaser release
