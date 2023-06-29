.PHONY: clean flatbuffers generate test testdata docker-build docker-buildx
.PRECIOUS: %.wasm

SHELL := /bin/bash

GO ?= go

PYTHONWASM_BUILD = python/cpython/python.wasm
PYTHONZIP_BUILD = python/cpython/usr/local/lib/python311.zip
PYTHON       ?= python3
PYTHONMAJOR  ?= 3
PYTHONMINOR  ?= 11
PYTHONPREFIX ?= testdata/python
PYTHONWASM   ?= $(PYTHONWASM_BUILD)
PYTHONZIP    ?= $(PYTHONZIP_BUILD)
VIRTUALENV   ?= $(PYTHONPREFIX)/env
PYTHONHOME   ?= $(VIRTUALENV)
PYTHONPATH   ?= $(PYTHONZIP):$(PYTHONHOME)

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

testdata.py.src = \
	$(wildcard testdata/python/*_test.py)

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

timecraft.sdk.src.go = \
	$(wildcard sdk/*.go sdk/go/timecraft/*.go)

timecraft.sdk.src.py = \
	$(wildcard sdk/python/src/timecraft/*.py)

timecraft.sdk.venv.py = \
	$(VIRTUALENV)/lib/python$(PYTHONMAJOR).$(PYTHONMINOR)/site-packages/timecraft

timecraft = ./timecraft

$(timecraft): go.mod $(timecraft.src.go)
	$(GO) build -o $@

clean:
	rm -f $(timecraft) $(format.src.go) $(testdata.go.wasm)

lint:
	golangci-lint run ./...
	buf lint

fmt:
	$(GO) fmt ./...

generate: flatbuffers
	buf generate

flatbuffers: go.mod $(format.src.go)
	$(GO) build ./format/...

test: go_test py_test wasi-testsuite

go_test: testdata
	$(GO) test ./...

testdata: $(testdata.go.wasm)

%_test.wasm: %_test.go go.mod $(timecraft.sdk.src.go)
	GOARCH=wasm GOOS=wasip1 $(GO) test -c -o $@ $<

%.wasm: %.go go.mod $(timecraft.sdk.src.go)
	GOARCH=wasm GOOS=wasip1 $(GO) build -o $@ $<

wasi-testsuite: timecraft testdata/wasi-testsuite
	python3 testdata/wasi-testsuite/test-runner/wasi_test_runner.py \
		-t testdata/wasi-testsuite/tests/assemblyscript/testsuite \
		   testdata/wasi-testsuite/tests/c/testsuite \
		   testdata/wasi-testsuite/tests/rust/testsuite \
		-r testdata/adapter.py
	@rm -rf testdata/wasi-testsuite/tests/rust/testsuite/fs-tests.dir/*.cleanup

py_test: $(timecraft) $(timecraft.sdk.venv.py) $(testdata.py.src) $(PYTHONWASM) $(PYTHONZIP)
	@if [ -f "$(PYTHONWASM)" ] && [ -f "$(PYTHONZIP)" ]; then \
		$(timecraft) run --env PYTHONPATH=$(PYTHONPATH) --env PYTHONHOME=$(PYTHONHOME) -- $(PYTHONWASM) -m unittest $(testdata.py.src); \
	else \
		echo "skipping Python tests (could not find $(PYTHONWASM) and $(PYTHONZIP))"; \
	fi

$(PYTHONZIP_BUILD) $(PYTHONWASM_BUILD):
	$(MAKE) -C python download || $(MAKE) -C python build-docker

$(timecraft.sdk.venv.py): $(VIRTUALENV)/bin/activate $(timecraft.sdk.src.py)
	source $(VIRTUALENV)/bin/activate; pip install sdk/python

$(VIRTUALENV)/bin/activate:
	$(PYTHON) -m venv $(VIRTUALENV)

testdata/wasi-testsuite: testdata/wasi-testsuite/.git

testdata/wasi-testsuite/.git: .gitmodules
	git submodule update --init --recursive -- testdata/wasi-testsuite

# We run goimports because the flatc compiler sometimes adds an unused import of
# strconv.
%_generated.go: %.fbs
	flatc --go --gen-onefile --go-namespace $(basename $(notdir $<)) --go-module-name github.com/stealthrocket/timecraft/format -o $(dir $@) $<
	goimports -w $@

docker-build:
	docker build -f Dockerfile -t $(container.image):$(container.version) .

docker-buildx:
	docker buildx build --push --platform=linux/amd64,linux/arm64 -f Dockerfile -t $(container.image):$(container.version) .
