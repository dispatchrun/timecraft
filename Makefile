.PHONY: clean flatbuffers generate test testdata
.PRECIOUS: %.wasm

pkg.src.go = \
	$(wildcard pkg/*/*.go)

format.src.fbs = \
	$(wildcard pkg/format/*/*.fbs)
format.src.go = \
	$(format.src.fbs:.fbs=_generated.go)

timecraft.src.go = \
	$(format.src.go) \
	$(pkg.src.go) \
	$(wildcard *.go) \
	$(wildcard cmd/*.go)

timecraft.testdata.go = \
	$(wildcard pkg/timecraft/testdata/*_test.go)
timecraft.testdata.wasm = \
	$(timecraft.testdata.go:_test.go=_test.wasm)

timecraft: go.mod $(timecraft.src.go)
	go build -o timecraft

clean:
	rm -f timecraft $(timecraft.testdata.wasm) $(format.src.go)

generate: flatbuffers

flatbuffers: go.mod $(format.src.go)
	go build ./pkg/format/...

test: flatbuffers testdata
	go test -v ./...

testdata: $(timecraft.testdata.wasm)

%_test.wasm: %_test.go
	GOROOT=$(PWD)/../go GOARCH=wasm GOOS=wasip1 ../go/bin/go test -tags timecraft -c -o $@ $<

# We run goimports because the flatc compiler sometimes adds an unused import of
# strconv.
%_generated.go: %.fbs
	flatc --go --gen-onefile --go-namespace $(basename $(notdir $<)) --go-module-name github.com/stealthrocket/timecraft/pkg/format -o $(dir $@) $<
	goimports -w $@
