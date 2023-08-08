---
sidebar_position: 1
---

# Architecture

Timecraft is an application runtime leverage WebAssembly as main sandboxing system. On top of it, Timecraft implements a networking and filesystem sandbox allowing
operators to get full control of their applications' interactions with the rest of the world.


:::info
We are activelly working on bringing a native sandboxing system to Timecraft. While we believe that WebAssembly is the future of cloud computing and distributed systems, we might want to leverage your existing development workflow while still benefit from the Timecraft features. The native sandbox will be based on cAdvisor.
:::


## Timecraft internals

Internally, Timecraft is composed of the following:

- A WebAssembly runtime 
- A decentralized scheduler
- A log of execution

### WebAssembly runtime

Timecraft leverags Wazero for its WebAssembly runtime and enhances it with a couple of goodies.

The WASI layer implements a [complete, non-blocking, network stack](https://github.com/stealthrocket/wasi-go/blob/main/imports/wasi_snapshot_preview1/wasmedge.go) based on the WasmEdge API, allowing developers to run a full application in WASM. While we are currently using the WasmEdge API, we are also looking at implementing [WASIX](http://wasix.org/) to provide wider compatibility with the WebAssembly ecosystem.

The Timecraft WASI layer is currently fully compatible with Go 1.21 (to be released later this month) and Python 3.11. Note that Python Asyncio is also supported.


### Decentralized scheduler

Timecraft ships with a built-in decentralized orchestrator enabling durable execution at scale. The orchestrator is still in active development but fully functional.
Developers can programatically create graph of execution of actors, Timecraft will ensure those actors go to completion not matter the infrastructure event (instance restart, network partitions, etc).

The interaction with the scheduler goes through the [Timecraft SDK](https://github.com/stealthrocket/timecraft/tree/main/sdk) currently available for Golang and Python. More languages will be supported as we move forward.

:::info
More details about the durable execution can be found in the dedicated guides: [Durable execution](/guides/durable-execution).
:::

### Log of execution

Each Timecraft instance automatically record the entire execution of all application and actors it schedules. Coupled with the WebAssembly determinism, [The Log](/architecture/log) enables the Time Machine capabilities of the system:
- Deterministic simulator
- Profiling of past executions
- Interactive debugging (coming soon)
- Telemetry injection / Execution rehydratation (coming soon)

# Time Machine

The Time Machine is a core primitive of Timecraft that brings a new step function in scaling the development and operation of distributed systems. Because the recording happens at the bottom-most level of interaction between the host runtime and the guest applications, it scales with the addition of new capabilities. For example, recording all network connections at the socket layer means that we can track the network messages exchanged in any protocol. Applications using protocols that cannot yet generate high-level observability can still be executed and recorded, and capabilities can be added later on to extract value from the records.

Decoupling the recording from the high-level machinery is the architectural model that brings scalability. Instead of systems having to implement their instrumentation, the separation of concerns offered by the low-level recording capability provides a much more powerful model where extracting context from the record does not require modification of the application code, and can happen after the fact on records of past executions.

Because the capture happens on the critical path of the guest application, it must be fast. The write path of the time machine must be optimized to have the lowest possible cost. The data captured is first buffered in memory, then sent to local storage and can be synced asynchronously to an object store like S3 for long-term durability at an affordable cost.

Over time, we anticipate that the development of most operational capabilities could be built on top of Time Machines: time travel debugging through layers of services, high-level telemetry, or deterministic simulations could all be derived from the records captured from the execution of guest applications on the runtime. The possibilities span far beyond software operations and could even reach the realm of data analytics: data lakes of application records where asynchronous batch processing pipelines create views tailored to provide insight into all aspects of the products built on top of those distributed systems.

One of the main challenges of those Time Machines is facilitating access to the scale of records that it produces. Even with effective compression to minimize the storage footprint, they can geneate very large volumes of information. Developing solutions to efficiently access the gold mine of data captured in the logs is one of the responsibilities that Timecraft can fulfill on behalf of the applications it runs.
