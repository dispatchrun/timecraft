# Compiling your Go application to WebAssembly

## Preparing Go

Go 1.21, due to be released in August 2023, will be able to natively compile Go applications to WebAssembly.

```console
$ GOOS=wasip1 GOARCH=wasm go build ...
```

Since Go 1.21 has not been released yet, you can use [`gotip`](https://pkg.go.dev/golang.org/dl/gotip) to test these features before release:

```console
$ go install golang.org/dl/gotip@latest
$ gotip download
```

```console
$ GOOS=wasip1 GOARCH=wasm gotip build ...
```

## Compiling your application

Instead of using `go build`, use:

```console
$ GOOS=wasip1 GOARCH=wasm gotip build -o app.wasm <path/to/main/pkg>
```

This will build a WebAssembly module that can be run with Timecraft.

## Running your application with Timecraft

To run your application:

```console
$ timecraft run app.wasm
```

Command-line arguments can be specified after the WebAssembly module. To prevent
Timecraft from interpreting command-line options for the application, use:

```console
$ timecraft run -- app.wasm arg1 arg2 ...
```

Note that environment variables passed to Timecraft are automatically passed to the
WebAssembly module.

## Networking applications

You may quickly find that the networking capabilities of `GOOS=wasip1` are limited.

We have created a library to help with common use cases, such as creating HTTP servers
and connecting to databases. See https://github.com/stealthrocket/net for usage details.
