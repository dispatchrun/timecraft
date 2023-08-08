# Record & Replay

## Record

Timecraft records a trace of execution to a log, and stores the log and the WebAssembly module that was executed in its local registry.

Here's an example (`hello.go`):

```go showLineNumbers
package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}
```

```bash
$ GOOS=wasip1 GOARCH=wasm go build -o hello.wasm hello.go
$ timecraft run hello.wasm
7fdf35be-42e5-43ed-8417-8bc56e4cefd0
Hello, World!
```

The first line written to stderr is an identifier for the process.
Each time you run your application, a new identifier will be generated.
The identifier can be used to replay or analyze execution at a later stage.

## Replay

To replay execution, use:

```bash
$ timecraft replay 7fdf35be-42e5-43ed-8417-8bc56e4cefd0
Hello, World!
```

The same WebAssembly module was executed, however the copy stored in the
registry was used.

Rather than make real system calls, the replay is sandboxed and
side-effect free. It leverages the deterministic nature of WebAssembly
module execution to achieve exactly the same execution path as the
original.

## Using the Registry

Timecraft stores resources, such as process logs and WebAssembly modules, in a local registry.

To list resource types:

```bash
timecraft get
```

### Processes

To show recently executed processes:

```bash
$ timecraft get process
PROCESS ID                            START    SIZE
7fdf35be-42e5-43ed-8417-8bc56e4cefd0  17m ago  2.43 KiB
```

To show details about a process and its trace of execution:

```bash
$ timecraft describe process 7fdf35be-42e5-43ed-8417-8bc56e4cefd0
ID:      7fdf35be-42e5-43ed-8417-8bc56e4cefd0
Start:   16m ago, Thu, 29 Jun 2023 16:33:32 AEST
Runtime: timecraft (devel 36225ccf69b258491deb7f764f9b1d25296fe0a4)
Modules:
  sha256:9aa43ada4d6394d6df8bbb32d8481fb00d2bc967049a465f01c5c3baf00703e0: (none) (1.94 MiB)
Args:
  hello.wasm
Env:
  ...
Records: 23, 1 batch(es), 1 segment(s), 2.36 KiB/5.44 KiB +72 B (compression: 56.65%)
---
SEGMENT  RECORDS  BATCHES  DURATION  SIZE      UNCOMPRESSED SIZE  COMPRESSED SIZE  COMPRESSION RATIO
0        23       1        4ms       2.43 KiB  5.44 KiB           2.36 KiB         56.65%
```

### Modules

To show WebAssembly modules that were recently executed:

```bash
$ timecraft get module
MODULE ID     MODULE NAME  SIZE
9aa43ada4d63  hello.wasm   1.94 MiB
```

To show details about a WebAssembly module:

```bash
$ timecraft describe module 9aa43ada4d63
ID:   sha256:9aa43ada4d6394d6df8bbb32d8481fb00d2bc967049a465f01c5c3baf00703e0
Name: hello.wasm
Size: 1.94 MiB
...
```

To export a WebAssembly module:

```bash
timecraft export module 9aa43ada4d63 hello2.wasm
```
