#!/bin/bash

set -e

if [ -z "$1" ]; then
    >&2 echo -e "$0 downloads pre-built WebAssembly python from github.\nProvide timecraft git sha to download."
    exit 1
fi

commit=$1

if [[ $(grep --version|grep BSD) ]]; then
    grep_match="grep -Eo" # bsd grep
else
    grep_match="grep -Po" # gnu grep
fi


echo "Downloading pre-built python.wasm from GitHub artifacts"

run_id=$(gh api \
  -H "Accept: application/vnd.github+json" \
  -H "X-GitHub-Api-Version: 2022-11-28" \
  "/repos/stealthrocket/timecraft/actions/runs?head_sha=${commit}" \
    |${grep_match} 'workflow_id":61648424,.+?"url":".+?"' \
    |${grep_match} '/[0-9]+' \
    |${grep_match} '[0-9]+')


echo "Run ID: ${run_id}"

artifact_id=$(gh api \
  -H "Accept: application/vnd.github+json" \
  -H "X-GitHub-Api-Version: 2022-11-28" \
  "/repos/stealthrocket/timecraft/actions/runs/${run_id}/artifacts" \
		  | ${grep_match} '"artifacts":[{"id":[0-9]+,' \
		  | ${grep_match} '[0-9]+')

echo "Artifact ID: ${artifact_id}"

gh api \
  -H "Accept: application/vnd.github+json" \
  -H "X-GitHub-Api-Version: 2022-11-28" \
  /repos/stealthrocket/timecraft/actions/artifacts/${artifact_id}/zip > /tmp/timecraft-python.zip

pushd cpython >/dev/null
unzip -o /tmp/timecraft-python.zip >/dev/null
popd > /dev/null

echo "Python downloaded at:"
find . -name 'python.wasm' -o -name 'python311.zip'
