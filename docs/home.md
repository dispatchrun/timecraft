---
label: "Home"
sidebar_position: 1
sidebar_label: "Home"
slug: /
link:
    type: generated-index
    title: "Home"
---

# Timecraft docs

## What is Timecraft?

Timecraft is an application runtime designed to build and run production grade distributed systems. The runtime is at the core of the Stealth Rocket platform.
With Timecraft, you can compose and run resilient distributed systems, lower the overall platform complexity and build the next generation of products.

Timecraft leverages WebAssembly as an userland kernel to introspect and record the execution of your applications. With those records, Timecraft provide advanced
debugging mechanisms but also high-level primitives to help developers focus on their applications logic.

## Roadmap

We are preparing the Timecraft public roadmap and will be publish it soon on Github. Stay tuned!


## Quickstart

Install Timecraft and run your first application locally.

```bash
curl -sL https://timecraft.dev/install.sh | bash
```

Compile your application to WebAssembly and run it:

```bash
timecraft run app.wasm
```

Timecraft will run and record the execution of your application. The first log line will be the unique ID of the Log:
```bash
cd0b69bb-3706-407e-9159-e1368e7bf774
```

Replay, trace or profile the execution:

```bash
timecraft replay cd0b69bb-3706-407e-9159-e1368e7bf774

timecraft trace cd0b69bb-3706-407e-9159-e1368e7bf774

timecraft profile cd0b69bb-3706-407e-9159-e1368e7bf774
```

Next, start building resiliant application using the built-in decentralized orchestrator.

:::info
For more details, check out our [Getting Started guide](https://docs.timecraft.dev/getting-started/installation).
:::


## Community

Come join the Timecraft community! We are happy to help and hear about your use cases.

- [Github issues](https://github.com/stealthrocket/timecraft/issues): Open an issue to discuss about challenges or technical details
- [Discord](https://stealthrocket.tech/discord): Join us on Discord!
- [Twitter](https://twitter.com/_stealthrocket): Follow us and Twitter. We regularly post updates about what we are cooking!
- [steathrocket.tech](https://stealthrocket.tech): Check our website. We, Stealth Rocket, are building the next gen distributed systems platform!
