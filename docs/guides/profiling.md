# Profiling 

Timecraft can generate CPU and memory profiles from a historical trace of execution.

Timecraft uses [wzprof](https://github.com/stealthrocket/wzprof) to profile applications
compiled to WebAssembly.
For a deep dive into the technology, see [this blog post](https://blog.stealthrocket.tech/performance-in-the-spotlight-webassembly-profiling-for-everyone/).

## Generating Profiles

Here's an example process:

```bash
$ timecraft run testdata/go/echo.wasm foo
9341c1c8-1d9b-43b5-a140-b36f866e00cf
foo
```

To generate profiles, use the `timecraft profile` command. This will generate a CPU
profile and memory profile, and write them to the registry:

```bash
$ timecraft profile 9341c1c8-1d9b-43b5-a140-b36f866e00cf
PROFILE ID    PROCESS ID                            TYPE    START   DURATION  SIZE
a53837e395c3  9341c1c8-1d9b-43b5-a140-b36f866e00cf  cpu     6s ago  5ms       37.6 KiB
219b8e22df5a  9341c1c8-1d9b-43b5-a140-b36f866e00cf  memory  6s ago  5ms       2.17 KiB
```

The `timecraft profile` command has options to control things like the
time range and sampling rate. See `timecraft profile -h`.

## Viewing Profiles

To export a profile from the registry, use `timecraft export profile`:

```bash
timecraft export profile a53837e395c3 profile.out
```

To view the profile interactively, use:

```bash
go tool pprof -http :3000 profile.out
```

## Profiling Issues

### `dot` executable not found in path

Try `brew install graphviz` or `sudo apt install graphviz`
