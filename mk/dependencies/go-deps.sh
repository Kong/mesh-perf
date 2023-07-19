#!/bin/bash

set -e

OUTPUT_BIN_DIR=$1/bin

for i in \
    github.com/google/go-jsonnet/cmd/jsonnet@v0.20.0 \
    github.com/jsonnet-bundler/jsonnet-bundler/cmd/jb@v0.5.1; do
  echo "install go dep: ${i}"
  GOBIN=${OUTPUT_BIN_DIR} go install "${i}" &
done

wait
