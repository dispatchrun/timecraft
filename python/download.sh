#!/bin/bash

set -ex

if [ -z "$1" ]; then
    >&2 echo -e "$0 downloads pre-built WebAssembly python from s3.\nProvide timecraft git sha to download."
    exit 1
fi

commit=$1

python_wasm_out="cpython/python.wasm"
python_zip_out="cpython/usr/local/lib/python311.zip"

mkdir -p cpython/usr/local/lib/
curl --fail-with-body https://timecraft.s3.amazonaws.com/python/${commit}/python.wasm > ${python_wasm_out}
curl --fail-with-body https://timecraft.s3.amazonaws.com/python/${commit}/python311.zip > ${python_zip_out}

echo "Python downloaded at:"
echo ${python_wasm_out}
echo ${python_zip_out}

