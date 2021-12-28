#!/bin/bash

set -e -o pipefail

if [ "$DISABLE_LINTER" == "true" ]
then
  exit 0
fi

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

if ! [ -x "$(command -v golangci-lint)" ]; then
        echo "Looks like golangci-lint isn't installed, to run \'make lint\' please install it."
        exit 127
fi

export GO111MODULE=on
golangci-lint run \
  --verbose \
  --build-tags build
