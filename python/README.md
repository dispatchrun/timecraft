# timecraft/python

CPython running on WebAssembly in timecraft.


## Run

Assuming in the directory of this readme, after building.

### Interpreter

```
timecraft run \
  -e PYTHONPATH=`pwd`/deps:`pwd`/cpython/usr/local/lib/python311.zip \
  -e PYTHONHOME=`pwd`/cpython/usr/local/lib/python311.zip \
  -- `pwd`/cpython/python.wasm
```

### Script

```
timecraft run \
  -e PYTHONPATH=`pwd`/deps:`pwd`/cpython/usr/local/lib/python311.zip \
  -e PYTHONHOME=`pwd`/cpython/usr/local/lib/python311.zip \
  -- `pwd`/cpython/python.wasm \
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

## Download

Instead of building it yourself, you can download the artifact of the latest
successful build on github:

Go to [Actions][actions] -> Click on green build -> Click on artifact.

Unzip the downloaded file, then adjust the paths of the examples above.


[actions]: https://github.com/stealthrocket/timecraft/actions


## Build

```
./build.sh
```

Produces:

* `cpython/python.wasm` - the python wasm module.
* `cpython/usr/local/lib/python311.zip` - the python standard library.

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

* `/cpython` - https://github.com/stealthrocket/cpython/tree/dev
* `/wasi-libc` - https://github.com/stealthrocket/wasi-libc/tree/dev
* `/wasmedge_sock` - libc socket library using wasmedge extension

### `/wasmedge_sock`

Static library implementation of posix sockets based on the wasmedge interface.
Initially from [vmware][vmware]. Heavily modified. Should be contributed back to
them at some point.

[pip]: https://pip.pypa.io/en/stable/installation/
[wasm-opt]: https://github.com/WebAssembly/binaryen/releases/tag/version_113
[libclang]: https://github.com/stealthrocket/wasi-go/blob/main/CONTRIBUTING.md
[vmware]: https://github.com/vmware-labs/webassembly-language-runtimes/tree/f9547e0f4e3bb798d60ba6ed10a6eb47861ebdc4/libs/wasmedge_sock
