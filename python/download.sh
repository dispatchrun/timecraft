#!/bin/bash

set -e

if [ -z "$1" ]; then
    >&2 echo -e "$0 downloads pre-built WebAssembly python from s3.\nProvide timecraft git sha to download."
    exit 1
fi

commit=$1

curl https://timecraft.s3.amazonaws.com/python/${commit}/python.wasm > cpython/python.wasm
curl https://timecraft.s3.amazonaws.com/python/${commit}/python311.zip > cpython/usr/local/lib/python311.zip

echo "Python downloaded at:"
find . -name 'python.wasm' -o -name 'python311.zip'
