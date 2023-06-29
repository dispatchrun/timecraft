#!/bin/bash

set -ex

unset WASI_SDK_PATH

export CC="${CC:=`which clang-15`}"
export AR="${AR:=`which llvm-ar-15`}"
export NM="${NM:=`which llvm-nm-15`}"
export RL="${RL:=`which llvm-ranlib-15`}"

echo "=== building wasi-libc"
pushd ./wasi-libc/
make -j`nproc`
popd

export SYSROOT=`pwd`/wasi-libc/sysroot


# build wasmedge static library against wasi-libc
WASMEDGE_DEBUG=""
if [ ! -z "${DEBUG}" ]; then
    WASMEDGE_DEBUG="-DWASMEDGE_SOCKET_DEBUG"
fi

WASMEDGE_LIB="`pwd`/wasmedge_sock/build"
WASMEDGE_INCLUDE="`pwd`/wasmedge_sock/include"
mkdir -p "${WASMEDGE_LIB}"
pushd "${WASMEDGE_LIB}"
rm -f *.a *.o
${CC} -Wall -Werror -O0 -MD -MT -MF -c --target=wasm32-unknown-wasi --sysroot=${SYSROOT} -I`pwd`/../include ${WASMEDGE_DEBUG} ../wasi_socket_ext.c -fPIC -o libwasmedge.o
${AR} qc libwasmedge_sock.a libwasmedge.o
${RL} libwasmedge_sock.a
popd

export MYBUILD="`pwd`/build"

# download pre-build libz
LIBZ="${MYBUILD}/libz"
mkdir -p "${LIBZ}"
pushd "${LIBZ}"
if [ ! -f "lib/wasm32-wasi/libz.a" ]; then
    wget "https://github.com/vmware-labs/webassembly-language-runtimes/releases/download/libs%2Fzlib%2F1.2.13%2B20230310-c46e363/libz-1.2.13-wasi-sdk-19.0.tar.gz"
    tar xvf libz-1.2.13-wasi-sdk-19.0.tar.gz
fi
popd
export LIBZ_LIB="${LIBZ}/lib/wasm32-wasi"
export LIBZ_INCLUDE="${LIBZ}/include"

# download pre-build libuuid
LIBUUID="${MYBUILD}/libuuid"
mkdir -p "${LIBUUID}"
pushd "${LIBUUID}"
if [ ! -f "lib/wasm32-wasi/libuuid.a" ]; then
    wget "https://github.com/vmware-labs/webassembly-language-runtimes/releases/download/libs%2Flibuuid%2F1.0.3%2B20230310-c46e363/libuuid-1.0.3-wasi-sdk-19.0.tar.gz"
    tar xvf libuuid-1.0.3-wasi-sdk-19.0.tar.gz
fi
popd
export LIBUUID_LIB="${LIBUUID}/lib/wasm32-wasi"
export LIBUUID_INCLUDE="${LIBUUID}/include"


# build python against wasi-libc
echo "=== building python 3.11.3"
PYSOURCE="`pwd`/cpython"
pushd "${PYSOURCE}"

export CFLAGS_CONFIG="${CFLAGS_CONFIG} -Wno-int-conversion"
export CFLAGS="${CFLAGS_CONFIG} ${CFLAGS_DEPENDENCIES} ${CFLAGS}"
export LDFLAGS="${LDFLAGS_DEPENDENCIES} ${LDFLAGS}"

export CFLAGS="-I${LIBZ_INCLUDE} ${CFLAGS}"
export LDFLAGS="-L${LIBZ_LIB} ${LDFLAGS}"

export CFLAGS="-I${LIBUUID_INCLUDE} ${CFLAGS}"
export LDFLAGS="-L${LIBUUID_LIB} ${LDFLAGS}"

export CFLAGS="-I${WASMEDGE_INCLUDE} --sysroot=${SYSROOT} ${CFLAGS}"
export LDFLAGS="-lwasmedge_sock -L${WASMEDGE_LIB} ${LDFLAGS}"
export PYTHON_WASM_CONFIGURE="--with-build-python=python3.11"

if [ ! -z "${DEBUG}" ]; then
    export CFLAGS="-DWASMEDGE_SOCKET_DEBUG ${CFLAGS}"
    export PYTHON_WASM_CONFIGURE="${PYTHON_WASM_CONFIGURE} --with-pydebug"
fi

export CC="${CC} --sysroot=${SYSROOT} --target=wasm32-wasi"
CONFIG_SITE=./Tools/wasm/config.site-wasm32-wasi ./configure -C --host=wasm32-wasi --build=$(./config.guess) ${PYTHON_WASM_CONFIGURE}
export MAKE_TARGETS='python.wasm wasm_stdlib'
make -j`nproc` ${MAKE_TARGETS}

if [ -z "${NO_OPT}" ]; then
    wasm-opt -O2 -o python-optimized.wasm python.wasm
    mv python-optimized.wasm python.wasm
fi

pushd Lib/
zip -ur ../usr/local/lib/python311.zip ssl.py
popd

popd


echo "=*= DONE =*="
