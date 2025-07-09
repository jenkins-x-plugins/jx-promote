#!/bin/bash -x

# See https://github.com/actions/checkout/issues/766
git config --global --add safe.directory /github/workspace

git add * && git commit -a -m "chore: release $VERSION" || echo No change

export PULL_BASE_SHA=$(git rev-parse HEAD)

jx changelog create --verbose --version=$VERSION --rev=$PULL_BASE_SHA --output-markdown=changelog.md
