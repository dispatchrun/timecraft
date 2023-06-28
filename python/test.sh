#!/bin/bash

TIMECRAFT="${TIMECRAFT:=timecraft}"

python3.11 -m pip install --target deps/ --only-binary :all: --implementation py --abi none --platform any requests

exec ${TIMECRAFT} run \
  -e PYTHONPATH=`pwd`/deps:`pwd`/python/usr/local/lib/python311.zip \
  -e PYTHONHOME=`pwd`/python/usr/local/lib/python311.zip \
--sockets wasmedgev2 -- `pwd`/python/python.wasm ./test.py -v
