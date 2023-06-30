#!/bin/bash

set -e

if [ -z "$1" ]; then
    >&2 echo -e "$0 downloads pre-built WebAssembly python from s3.\nProvide timecraft git sha to download."
    exit 1
fi

commit=$1

python_wasm_out="cpython/python.wasm"
python_zip_out="cpython/usr/local/lib/python311.zip"

mkdir -p cpython/usr/local/lib/
curl https://timecraft.s3.amazonaws.com/python/${commit}/python.wasm > ${python_wasm_out}
curl https://timecraft.s3.amazonaws.com/python/${commit}/python311.zip > ${python_zip_out}

echo "Python downloaded at:"
echo ${python_wasm_out}
od -t x1 -N 8 ${python_wasm_out}
echo ${python_zip_out}
od -t x1 -N 8 ${python_zip_out}
