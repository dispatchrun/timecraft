# timecraft-python

CPython running on WebAssembly in timecraft.

Everything in one place to iterate fast.


## Run

Assuming at the root of this repository, after building.

### Interpreter

```
timecraft run \
  -e PYTHONPATH=`pwd`/deps:`pwd`/python/usr/local/lib/python311.zip \
  -e PYTHONHOME=`pwd`/python/usr/local/lib/python311.zip \
  -- `pwd`/python/python.wasm
```

### Script

```
timecraft run \
  -e PYTHONPATH=`pwd`/deps:`pwd`/python/usr/local/lib/python311.zip \
  -e PYTHONHOME=`pwd`/python/usr/local/lib/python311.zip \
  -- `pwd`/python/python.wasm \
  ./script.py arg1 arg2 etc
```

### Dependencies

To install Python dependencies for the script you want to run, use the following
command (you need [pip installed][pip] in your host python):

```
python3 -m pip install \
  --target deps/ \
  --only-binary :all: --implementation py --abi none --platform any --python-version "3.11" \
  whateverpackage
```

This will store the package (if available) in `./deps`, which is provided using
`PYTHONPATH` to wasm.

### Test

You can run the timecraft-python test suite with:

```
./test.sh
```

You can override the timecraft command by setting the environment variable
`TIMECRAFT`. For example:

```
TIMECRAFT="go run ../timecraft" ./test.sh
```

## Download

Instead of building it yourself, you can download the artifact of the latest
successful build on github:

Go to [Actions][actions] -> Click on green build -> Click on artifact.

Unzip the downloaded file, then adjust the paths of the examples above.


[actions]: https://github.com/stealthrocket/timecraft-python/actions


## Build

```
./build.sh
```

Produces:

* `/python/python.wasm` - the python wasm module.
* `/python/usr/local/lib/python311.zip` - the python standard library.

### Requirements

* Clang 9+ (tested with 15.0.7 on ubuntu)
* If necessary, [`libclang_rt.builtins-wasm32.a`][libclang].
* `wasm-opt` on your `PATH`. ([download][wasm-opt]).
* `python3.11` on your `PATH`.

or

* Docker

If you don't want to install everything locally, you can build a docker image
that contains those dependencies:

```
docker build -t builder .
docker run -v `pwd`:/src -w /src builder ./build.sh
```

### Configuration

The `build.sh` script creates a `./build` directory where it stores extra
dependencies in.

Clang and LLVM are expected to be on the path. Otherwise, use the following
environment variables:

```
export CC=/usr/lib/llvm-15/bin/clang
export AR=/usr/lib/llvm-15/bin/llvm-ar
export NM=/usr/lib/llvm-15/bin/llvm-nm
export RL=/usr/lib/llvm-15/bin/llvm-ranlib
```

If you want to iterate faster, disable the `wasm-opt` pass by providing any
value to `NO_OPT` when building.

When making any change in configuration, it is possible you may have to run
`make dist-clean` in `/python`.

### Debugging

* Provide `NO_OPT` to `build.sh` to disable `wasm-opt`.
* Provide `DEBUG` to `build.sh` to enable `--pydebug`.
* Provide `PYTHONTRACEMALLOC=1` to `timecraft run` to trace Python malloc calls.

Example of all those options when running tests:

```
NO_OPT=true DEBUG=true ./build.sh && \
timecraft run \
  -e PYTHONPATH=`pwd`/deps:`pwd`/python/usr/local/lib/python311.zip \
  -e PYTHONHOME=`pwd`/python/usr/local/lib/python311.zip \
  -e PYTHONTRACEMALLOC=1 \
  -- `pwd`/python/python.wasm ./test.py -v
```

## Layout

* `/python` - source of CPython 3.11.3
* `/timecraft-sdk` - Timecraft SDK
* `/wasi-libc` - source of wasi-libc 19
* `/wasmedge_sock` - libc socket library using wasmedge extension

### `/python`

Contains a copy of the CPython source code, plus the modified `ssl` module to
perform hTLS.

hTLS is a mechanism that offloads TLS operations from the guest (CPython) to the
host. It is similar to kTLS, in that the guest flags a socket to require TLS
encryption using socket option. Contrary to kTLS, hTLS has the host perform
handshake. Only timecraft supports this operation at the moment.

On the guest side, we provide a modified `ssl` module. It implements the minimum
interface to pretend to do TLS directly, but uses hTLS under the hood. Not all
operations may be available, but it works transparently for any HTTP mechanism
(`http`, `urllib3`, etc.) At the moment only client-side TLS is supported.

### `/timecraft-sdk`

Timecraft SDK is the python library intended to be used by Python guests running
inside timecraft to access its capabilities from the inside.

See [timecraft-sdk/README.md](timecraft-sdk/README.md).

### `/wasi-libc`

Copy of the [wasi-libc 19][wasi-libc] source code to make it easy to drop debug
`printf()`s when needed. Also contains changes to `chdir.c` and `getcwd.c` to
initialize the current working directory from the host's `PWD` environment
variable. It allows us to do invoke the python wasm module with a relative path
to a python script instead of requiring an absolute path.

We also add the field `sun_path` to `struct sockaddr_un`.

### `/wasmedge_sock`

Static library implementation of posix sockets based on the wasmedge interface.
Initially from [vmware][vmware]. Should be contributed back to them at some
point.

[pip]: https://pip.pypa.io/en/stable/installation/
[wasm-opt]: https://github.com/WebAssembly/binaryen/releases/tag/version_113
[libclang]: https://github.com/stealthrocket/wasi-go/blob/main/CONTRIBUTING.md
[wasi-libc]: https://github.com/WebAssembly/wasi-libc/tree/a1c7c2c7a4b2813c6f67bd2ef6e0f430d31cebad
[vmware]: https://github.com/vmware-labs/webassembly-language-runtimes/tree/f9547e0f4e3bb798d60ba6ed10a6eb47861ebdc4/libs/wasmedge_sock
