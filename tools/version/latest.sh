#!/bin/bash

if [ ! -x `which gh` ]; then
  echo "You must have github's gh client installed to install a preview version"
  exit 1
fi

REPO_ORG=kong
REPO_PREFIX=kong-mesh
BRANCH=master

JQCMD='.data.repository.ref.target.history.nodes | map(select(.statusCheckRollup.state == "SUCCESS")) | first | .oid'
PREVIEW_COMMIT=`gh api graphql  -f owner=${REPO_ORG} -f repo=${REPO_PREFIX} -f branch=${BRANCH} --jq "${JQCMD}" -F query='
query($owner: String!, $repo: String!, $branch: String!) {
  repository(owner: $owner, name: $repo) {
    ref(qualifiedName: $branch) {
      target {
        ... on Commit {
          history(first: 10) {
            nodes {
              oid
              statusCheckRollup {
                state
              }
            }
          }
        }
      }
    }
  }
}
'`

PREVIEW_COMMIT=$(echo $PREVIEW_COMMIT | cut -c -9)
VERSION=0.0.0-preview.v${PREVIEW_COMMIT}

printf "${VERSION}"
