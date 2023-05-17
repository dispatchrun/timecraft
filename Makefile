.PHONY: clean flatbuffers generate test
.PRECIOUS: %.wasm

format.src.fbs = \
	$(wildcard format/*/*.fbs)
format.src.go = \
	$(format.src.fbs:.fbs=_generated.go)

timecraft.src.go = \
	$(format.src.go) \
	$(wildcard *.go) \
	$(wildcard cmd/*.go) \
	$(wildcard internal/*/*.go)

timecraft: go.mod $(timecraft.src.go)
	go build -o timecraft

clean:
	rm -f timecraft $(format.src.go)

generate: flatbuffers

flatbuffers: go.mod $(format.src.go)
	go build ./format/...

test: flatbuffers
	go test -v ./...

# We run goimports because the flatc compiler sometimes adds an unused import of
# strconv.
%_generated.go: %.fbs
	flatc --go --gen-onefile --go-namespace $(basename $(notdir $<)) --go-module-name github.com/stealthrocket/timecraft/format -o $(dir $@) $<
	goimports -w $@
