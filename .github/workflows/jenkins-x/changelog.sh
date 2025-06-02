#!/bin/bash -x

git config --global --add safe.directory /github/workspace

export PULL_BASE_SHA=$(git rev-parse HEAD)

git add * || true
git commit -a -m "chore: release $VERSION" --allow-empty

# See https://github.com/actions/checkout/issues/766
jx changelog create --verbose --version=$VERSION --rev=$PULL_BASE_SHA --output-markdown=changelog.md
