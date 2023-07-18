#!/bin/bash

set -e

meta=$(find "${PROM_SNAPSHOT_PATH}" -name "meta.json")

minTime=$(jq '.minTime' ${meta})
maxTime=$(jq '.maxTime' ${meta})

echo "http://localhost:3000/d/z6C1v-NGk/perf-test?from=${minTime}&to=${maxTime}"
