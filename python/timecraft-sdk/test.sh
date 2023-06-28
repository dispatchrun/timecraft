#!/bin/bash



TIMECRAFT="${TIMECRAFT:=timecraft}"

if [ -z "${TIMECRAFT}" ]; then
    echo "Environment variable TIMECRAFT is empty. It needs to be set to the command to invoke timecraft."
    exit 1
fi

if [ -z "${PYTHON_WASM}" ]; then
    echo "Environment variable PYTHON_WASM is empty. It needs to be set to the absolute path to the python.wasm file."
    exit 1
fi

if [ -z "${PYTHON_ZIP}" ]; then
    echo "Environment variable PYTHON_ZIP is empty. It needs to be set to the absolute path to the python311.zip file."
    exit 1
fi


pip install . --upgrade --target deps/


exec ${TIMECRAFT} run \
  -e PYTHONPATH=`pwd`/deps:${PYTHON_ZIP} \
  -e PYTHONHOME=${PYTHON_ZIP} \
  -- ${PYTHON_WASM} -m unittest
