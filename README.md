# timecraft

[![Build](https://github.com/stealthrocket/timecraft/actions/workflows/build.yml/badge.svg)](https://github.com/stealthrocket/timecraft/actions/workflows/build.yml)

_The WebAssembly Time Machine_

**Timecraft** is a software runtime that executes WebAssembly modules with
sandboxing, task orchestration, and time travel capabilities.

The development of distributed systems comes with many challenges, and
satisfying the instrumentation, scale, or security requirements adds to the
difficulty. By combining a sandbox, a task orchestrator, and a time machine
in a software runtime, Timecraft intends to bring the tools and structure to
help developers on their journey to construct, test, and operate such systems.

[configuring]: https://github.com/stealthrocket/timecraft/wiki/Configuring-Timecraft
[installing]:  https://github.com/stealthrocket/timecraft/wiki/Installing-Timecraft
[preparing]:   https://github.com/stealthrocket/timecraft/wiki#preparing-your-application
[wasmedge]:    https://github.com/WasmEdge/WasmEdge
[wazero]:      https://github.com/tetratelabs/wazero
[wiki]:        https://github.com/stealthrocket/timecraft/wiki

## Getting Started

Timecraft is developed in Go on top of the [Wazero][wazero] WebAssembly
runtime.

The simplest path to installing the runtime is to use the standard installation
method of Go applications:
```
go install github.com/stealthrocket/timecraft
```
For a more detailed section on other ways to install and configure Timecraft,
see [Installing][installing] and [Configuring][configuring].

Timecraft can execute applications compiled to WebAssembly. This repository
has examples for applications written in Go and Python. At this time, the Go
programs need the new `GOOS=wasip1` port of Go, which is simpler to install
using `gotip`:
```
go install golang.org/dl/gotip@latest
gotip download
```
WebAssembly is an emerging technology which still has limitations when compared
to native programs. For a more detailed section on building applications to
WebAssembly, see [Preparing Your Application][preparing].

To try some of the examples, first compile them with this command:
```
make testdata
```

The compilation created programs with the `.wasm` extension since they are
compiled to WebAssembly. We will use the example program `get.wasm`, which
makes a GET request to the URLs passed as arguments:

```
$ timecraft run testdata/go/get.wasm https://eo3qh2ncelpc9q0.m.pipedream.net
ffe47142-c3ad-4f61-a5e9-739ebf456332
Hello, World!
```

There are few things to see here: the first line printed by Timecraft is the
unique identifier generated to represent this particular program execution and
its recording in the local Timecraft registry. The second line saying "Hello,
World!" is simply the response from this API endpoint that we sent the request
to.

The Time Machine has recorded the program execution, and can be accessed to
get insight from the program that was executed. It can reconstruct high level
context such as the HTTP requests and responses that were exchanged by the
application, for example:

```
$ timecraft trace request -v ffe47142-c3ad-4f61-a5e9-739ebf456332
2023/06/28 23:29:05.043455 HTTP 172.16.0.0:49152 > 44.206.52.165:443
> GET / HTTP/1.1
> Host: eo3qh2ncelpc9q0.m.pipedream.net
> User-Agent: Go-http-client/1.1
> Accept-Encoding: gzip
>
< HTTP/1.1 200 OK
< Date: Thu, 29 Jun 2023 06:29:04 GMT
< Content-Type: text/html; charset=utf-8
< Content-Length: 14
< Connection: keep-alive
< X-Powered-By: Express
< Access-Control-Allow-Origin: *
<
Hello, World!
```

To dive in, the `help` subcommand is a useful entrypoint to get an up to date
list of the supported capabilities, or take a look at the [Wiki][wiki] for a
walk through the documentation!

### Stability guarantees and future development

We are actively working on improving Timecraft and iterating closely with
our design partners to prioritize the development, which may from time to time
require the introduction of backward-incompatible changes to the APIs, data
formats, and configuration. As we mature the technology, we will progressively
advance towards offering more stability guarantees. The release of a v1.0 will
be the indicator that we are promising to support all the public facing
interfaces. To help us on this journey, do not heistate to reach out and
submit feedback, ideas, or code contributions!

### Contributing

Pull requests are welcome! Anything that is not a simple fix would probably
benefit from being discussed in an issue first.

Remember to be respectful and open minded!

## Time Machines

The Time Machine is a core primitive of Timecraft that brings a new step
function in scaling the development and operation of distributed systems.
Because the recording happens at the bottom-most level of interaction between
the host runtime and the guest applications, it scales with the addition of new
capabilities. For example, recording all network connections at the socket
layer means that we can track the network messages exchanged in any protocol.
Applications using protocols that cannot yet generate high-level observability
can still be executed and recorded, and capabilities can be added later on to
extract value from the records.

Decoupling the recording from the high-level machinery is the architectural
model that brings scalability. Instead of systems having to implement their
instrumentation, the separation of concerns offered by the low-level recording
capability provides a much more powerful model where extracting context from
the record does not require modification of the application code, and can
happen after the fact on records of past executions.

Because the capture happens on the critical path of the guest application, it
must be fast. The write path of the time machine must be optimized to have the
lowest possible cost. The data captured is first buffered in memory, then sent
to local storage and can be synced asynchronously to an object store like S3
for long-term durability at an affordable cost.

Over time, we anticipate that the development of most operational capabilities
could be built on top of Time Machines: time travel debugging through layers of
services, high-level telemetry, or deterministic simulations could all be
derived from the records captured from the execution of guest applications on
the runtime. The possibilities span far beyond software operations and could
even reach the realm of data analytics: data lakes of application records where
asynchronous batch processing pipelines create views tailored to provide insight
into all aspects of the products built on top of those distributed systems.

One of the main challenges of those Time Machines is facilitating access to the
scale of records that it produces. Even with effective compression to minimize
the storage footprint, they can geneate very large volumes of information.
Developing solutions to efficiently access the gold mine of data captured in the
logs is one of the responsibilities that Timecraft can fulfill on behalf of
the applications it runs.

## Task Orchestration

The industry is progressively adopting task orchestration as a way to reach
new levels of engineering and infrastructure scale. The Serverless/Microservice
model is showing its limits and the realization is settling that composing large
amounts of infrastructure pieces with container orchestrators is not viable.
The container as a unit carries too much responsibility and building the tools
necessary to successfully develop and operate distributed software systems has
to rely on primitives which are not well suited to the challenge.

Workflow orchestrators are responses to the impracticality of scaling the
development and maintenance of such systems at the infrastructure layer.
The frameworks raise the abstraction level, giving engineers composable building
blocks that they can use to detach the software logic of their distributed
applications from the infrastructure layout. Decoupling helps control the
problem scope of each layer, giving engineers a lot more runway to grow and
scale their systems by dealing with simpler logical problems at each layer
instead of having to account for the combination of possibilities created by
leaky abstractions.

In successfully enabling engineers to reach the next level of scale, those
solutions hit another physical limit: the complexity of the business logic
grows exponentially with the number of orchestrated tasks. In our experience,
this limit is attained extremely quickly after the introduction of a workflow
orchestrator; it does not prevent the system from continuing to scale and grow,
but it limits the engineers’ ability to add features and assist product
development because their time is allocated to creating the observability and
controls needed to successfully manage the platform. Time Machines are a much
more effective answer to this problem, the always-on recording and complete
view of the system execution they provide free engineering resources from the
responsibility of creating the necessary instrumentation.

## Untrusted Code Execution

The tension of limited product development capacity also creates a window of
perspective into the next barrier that businesses run into: the increased
complexity of their product logic makes it impractical to build software
accounting for and evolving with all possible use cases. This is when the
need to allow external parties to customize their product usage by injecting
small pieces of software into the platform arises. Opening up the door to
external contributors is also seen as an opportunity to lift the pressure off
of tightly locked engineering resources; however, it never works unless the
engineering team is already able to leverage this development model, then
external developers are just one of the many entities contributing to the
product.

The need to enable customization from third parties comes with extreme
challenges to controlling product safety and quality. The ability to create
products allowing the secure injection of code is becoming a differentiating
factor between companies that delight customers in the way they can be
customized to their needs and those that are collapsing under the weight of
their attempt at incorporating all possible use cases in their core product.

By combining a _time machine_ and a _task orchestrator_ with the sandboxing
compabilities of WebAssembly, Timecraft offers a novel take on the way
distributed systems can be built to address the ever more complex demand that
businesses have for software.
