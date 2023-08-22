#!/bin/bash

set -x

dst_dir=${DST_DIR:-build}
version=${PERF_TEST_MESH_VERSION}

if [ "${version}" = "preview" ]; then
  # This is a pretty ugly hack because there is no way to specify the name of a resulting Kong Mesh package.
  # We have to fetch it to temporary directoy 'tmp_subdir' and move to 'kong-mesh-preview' afterwards.
  tmp_subdir="${dst_dir}/tmp_subdir"
  rm -rf "${tmp_subdir}"
  mkdir "${tmp_subdir}"
  (cd "${tmp_subdir}" && curl -L https://docs.konghq.com/mesh/installer.sh | VERSION="${version}" sh -)
  mkdir ${dst_dir}/kong-mesh-preview
  mv ${tmp_subdir}/*/* ${dst_dir}/kong-mesh-preview
  rm -rf ${tmp_subdir}
else
  (cd "${dst_dir}" && curl -L https://docs.konghq.com/mesh/installer.sh | VERSION="${version}" sh -)
fi
