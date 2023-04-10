.PHONY: clean test testdata
.PRECIOUS: %.wasm

pkg.src.go = \
	$(wildcard pkg/*/*.go)

timecraft.src.go = $(pkg.src.go) \
	$(wildcard *.go) \
	$(wildcard cmd/*.go)

timecraft.testdata.go = \
	$(wildcard pkg/timecraft/testdata/*_test.go)
timecraft.testdata.wasm = \
	$(timecraft.testdata.go:_test.go=_test.wasm)

timecraft: go.mod $(timecraft.src.go)
	go build -o timecraft

clean:
	rm -f timecraft $(timecraft.testdata.wasm)

test: testdata
	go test -v ./...

testdata: $(timecraft.testdata.wasm)

%_test.wasm: %_test.go
	GOROOT=$(PWD)/../go GOARCH=wasm GOOS=wasip1 ../go/bin/go test -tags timecraft -c -o $@ $<
