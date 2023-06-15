.PHONY: clean flatbuffers generate test testdata
.PRECIOUS: %.wasm

GO ?= go

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
