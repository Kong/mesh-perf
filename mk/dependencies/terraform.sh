#!/bin/bash

set -e

OUTPUT_DIR=$1/bin
VERSION="1.5.7"
curl --fail --location -s -o /tmp/terraform.zip https://releases.hashicorp.com/terraform/${VERSION}/terraform_${VERSION}_${OS}_${ARCH}.zip
unzip -o /tmp/terraform.zip -d $OUTPUT_DIR
