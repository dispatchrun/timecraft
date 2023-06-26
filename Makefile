.PHONY: clean flatbuffers generate test testdata
.PRECIOUS: %.wasm

GO ?= go

ifeq ($(GITHUB_BRANCH_NAME),)
	branch := $(shell git rev-parse --abbrev-ref HEAD)-
else
	branch := $(GITHUB_BRANCH_NAME)-
endif
commit_timestamp := $(shell git show --no-patch --format=%ct)-
ifeq ($(GITHUB_SHA),)
	commit := $(shell git rev-parse --short=8 HEAD)
else
	commit := $(shell echo $(GITHUB_SHA) | cut -c1-8)
endif
container.version ?= $(if $(RELEASE_TAG),$(RELEASE_TAG),$(shell git describe --tags || echo '$(subst /,-,$(branch))$(commit_timestamp)$(commit)'))
container.image ?= ghcr.io/stealthrocket/timecraft

testdata.go.src = \
	$(wildcard testdata/go/*.go) \
	$(wildcard testdata/go/test/*.go)
testdata.go.wasm = $(testdata.go.src:.go=.wasm)

format.src.fbs = \
	$(wildcard format/*/*.fbs)
format.src.go = \
	$(format.src.fbs:.fbs=_generated.go)

timecraft.src.go = \
	$(format.src.go) \
	$(wildcard *.go) \
	$(wildcard */*.go) \
	$(wildcard */*/*.go) \
	$(wildcard */*/*/*.go)

timecraft: go.mod $(timecraft.src.go)
	$(GO) build -o timecraft

clean:
	rm -f timecraft $(format.src.go) $(testdata.go.wasm)

lint:
	golangci-lint run ./...
	buf lint

fmt:
	$(GO) fmt ./...

generate: flatbuffers
	buf generate

flatbuffers: go.mod $(format.src.go)
	$(GO) build ./format/...

test: testdata wasi-testsuite
	$(GO) test ./...

testdata: $(testdata.go.wasm)

%_test.wasm: %_test.go
	GOARCH=wasm GOOS=wasip1 $(GO) test -c -o $@ $<

%.wasm: %.go
	GOARCH=wasm GOOS=wasip1 $(GO) build -o $@ $<

wasi-testsuite: timecraft testdata/wasi-testsuite
	python3 testdata/wasi-testsuite/test-runner/wasi_test_runner.py \
		-t testdata/wasi-testsuite/tests/assemblyscript/testsuite \
		   testdata/wasi-testsuite/tests/c/testsuite \
		   testdata/wasi-testsuite/tests/rust/testsuite \
		-r testdata/adapter.py
	@rm -rf testdata/wasi-testsuite/tests/rust/testsuite/fs-tests.dir/*.cleanup

testdata/wasi-testsuite: testdata/wasi-testsuite/.git

testdata/wasi-testsuite/.git: .gitmodules
	git submodule update --init --recursive -- testdata/wasi-testsuite

# We run goimports because the flatc compiler sometimes adds an unused import of
# strconv.
%_generated.go: %.fbs
	flatc --go --gen-onefile --go-namespace $(basename $(notdir $<)) --go-module-name github.com/stealthrocket/timecraft/format -o $(dir $@) $<
	goimports -w $@

container-build: 
	docker build -f Dockerfile -t $(container.image):$(container.version) .

container-push:
	docker push $(container.image):$(container.version)
