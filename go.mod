module github.com/stealthrocket/timecraft

go 1.20

require (
	github.com/bufbuild/connect-go v1.8.0
	github.com/containerd/go-runc v1.0.0
	github.com/davecgh/go-spew v1.1.1
	github.com/google/flatbuffers v23.5.26+incompatible
	github.com/google/go-cmp v0.5.9
	github.com/google/pprof v0.0.0-20230705174524-200ffdc848b8
	github.com/google/uuid v1.3.0
	github.com/klauspost/compress v1.16.6
	github.com/planetscale/vtprotobuf v0.4.0
	github.com/stealthrocket/net v0.2.1
	github.com/stealthrocket/wasi-go v0.7.5
	github.com/stealthrocket/wazergo v0.19.1
	github.com/stealthrocket/wzprof v0.2.0
	github.com/tetratelabs/wazero v1.3.0
	golang.org/x/exp v0.0.0-20230713183714-613f0c0eb8a1
	golang.org/x/net v0.11.0
	golang.org/x/sync v0.3.0
	golang.org/x/sys v0.9.0
	golang.org/x/time v0.3.0
	google.golang.org/protobuf v1.31.0
	gopkg.in/yaml.v3 v3.0.1
	gvisor.dev/gvisor v0.0.0-20230720202042-c2a0ef354117
)

require (
	github.com/containerd/console v1.0.1 // indirect
	github.com/containerd/containerd v1.4.13 // indirect
	github.com/opencontainers/runtime-spec v1.1.0-rc.1 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/sirupsen/logrus v1.8.1 // indirect
)

replace gvisor.dev/gvisor => ../gvisor
