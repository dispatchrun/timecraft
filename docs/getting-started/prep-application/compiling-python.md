# Compiling your Python application to WebAssembly

Python applications aren't compiled to WebAssembly. Rather, a Python interpreter (CPython)
is compiled to WebAssembly and interprets your Python application as it would normally.

Minor patches have been made to both the CPython interpreter and standard library to ensure
that applications are able to create POSIX style sockets. This enables Python applications to
create servers and connect to databases. We intend to upstream our patches at some stage.

## Preparing Python

We provide a pre-compiled Python interpreter and patched standard library. You can download them by running

```
curl -fsSL https://timecraft.s3.amazonaws.com/python/main/python.wasm -o python.wasm
curl -fsSL https://timecraft.s3.amazonaws.com/python/main/python311.zip -o python311.zip
```

To build Python from scratch, see the instructions in the
[`./python`](https://github.com/stealthrocket/timecraft/tree/main/python) dir.

## Running your application with Timecraft

### Preparing dependencies

It's recommended that you install your Python dependencies in a virtualenv:

```console
$ python3 -m venv env
$ source ./env/bin/activate
(env) $ pip install -r /path/to/requirements.txt
```

### Running Timecraft

The Python interpreter needs to know where dependencies are installed
and where the standard library ZIP is located.

Assuming both the interpreter (`python.wasm`) and standard library (`python311.zip`)
are located in the current directory, you can pass this information to the
interpreter like so:

```
$ export PYTHONHOME=env
$ export PYTHONPATH=python311.zip:$PYTHONHOME
```

You can now run your application:

```
$ timecraft run python.wasm main.py
```
