.PHONY: clean flatbuffers generate test testdata
.PRECIOUS: %.wasm

GO ?= go

testdata.go.src = $(wildcard testdata/go/*.go)
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

timecraft: go.mod flatbuffers $(timecraft.src.go)
	$(GO) build -o timecraft

clean:
	rm -f timecraft $(format.src.go) $(testdata.go.wasm)

lint:
	golangci-lint run ./...

generate: flatbuffers

flatbuffers: go.mod $(format.src.go)
	$(GO) build ./format/...

test: timecraft testdata
	$(GO) test ./...

testdata: $(testdata.go.wasm)

testdata/go/%.wasm: testdata/go/%.go
	GOARCH=wasm GOOS=wasip1 $(GO) build -o $@ $<

# We run goimports because the flatc compiler sometimes adds an unused import of
# strconv.
%_generated.go: %.fbs
	flatc --go --gen-onefile --go-namespace $(basename $(notdir $<)) --go-module-name github.com/stealthrocket/timecraft/format -o $(dir $@) $<
	goimports -w $@
