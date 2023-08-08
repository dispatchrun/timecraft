# Durable execution

Timecraft includes a technical preview of a task orchestration feature.

At a high level, applications are able to submit tasks for execution, and
let the runtime figure out the best way of scheduling and executing those
tasks.

The runtime will spawn additional WebAssembly module processes to execute
tasks. These processes are completely isolated from the processes that
submitted the tasks, opening up new possibilities in terms of polyglot
applications and applications that need to run untrusted code.
Processes that are spawned to execute tasks are themselves able to
submit tasks, allowing for arbitrarily complex graphs of execution.

WebAssembly modules running in Timecraft are able to interact with the
runtime through the Timecraft SDK, to submit tasks, to wait until they're
complete, and to fetch results.

## Examples

Go:
* https://github.com/stealthrocket/timecraft/blob/main/testdata/go/task.go

Python:
* https://github.com/stealthrocket/timecraft/blob/main/testdata/python/timecraft_test.py
* https://github.com/stealthrocket/timecraft/blob/main/testdata/python/worker.py

## Durability

At this stage, tasks do not live past a restart or crash of the root process.
Timecraft V1 will leverage the record of execution to automatically resume tasks
that were still in-flight before the restart.

## Distributed Execution

At this stage, tasks are scheduled within the same Timecraft process. Timecraft
V1 will support distributed scheduling and execution, allowing applications to
make use of a cluster of resources.
