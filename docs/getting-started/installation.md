---
sidebar_position: 2
---
# Installing Timecraft

## Supported Systems

Timecraft runs on macOS and Linux.

## Installing Go

Timecraft requires [Go](https://go.dev/).

macOS users using [homebrew](https://brew.sh/) can install Go using:

```bash
brew install go
```

Ubuntu users can install Go with:

```bash
sudo apt install golang
```

For other Linux systems, please see the [Go Releases](https://go.dev/dl/) page for installation instructions.

## Installing Timecraft

To install Timecraft, run:

```bash
go install github.com/stealthrocket/timecraft@latest
```

Timecraft should now be available. To check that installation was successful,
run:

```bash
timecraft help
```

## Installation Issues

### Cannot find timecraft command

If the `timecraft` command cannot be found, make sure `$GOPATH/bin` is in
your search path:

```bash
export PATH="$GOPATH/bin:$PATH"
```

If `$GOPATH` has not been set, try:

```bash
go env | grep GOPATH
```
