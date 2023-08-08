# Go

## Compiling your application

Since Go 1.21, you can natively compiled Go applications to WebAssembly using the wasip1 GOOS:

```bash
GOOS=wasip1 GOARCH=wasm go build -o app.wasm <path/to/main/pkg>
```

If you prefer using TinyGo:

```bash
tinygo build -o app.wasm -target=wasi <path/to/main/pkg>
```

This will build a WebAssembly module that can be run with Timecraft.

## Running your application with Timecraft

To run your application:

```bash
timecraft run app.wasm
```

Command-line arguments can be specified after the WebAssembly module. To prevent
Timecraft from interpreting command-line options for the application, use:

```bash
timecraft run -- app.wasm arg1 arg2 ...
```

Note that environment variables passed to Timecraft are automatically passed to the
WebAssembly module.

## Networking applications

You may quickly find that the networking capabilities of `GOOS=wasip1` are limited.

We have created a library to help with common use cases, such as creating HTTP servers
and connecting to databases. See [https://github.com/stealthrocket/net](github.com/stealthrocket/net) for usage details.
