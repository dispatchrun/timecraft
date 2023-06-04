package wasicall

import (
	"testing"

	"github.com/stealthrocket/timecraft/internal/assert"
	"github.com/stealthrocket/wasi-go"
)

func TestEncodeIOVecsPrefixSameSize(t *testing.T) {
	v := wasi.IOVec("hello world")

	encoded := encodeIOVecsPrefix(nil, []wasi.IOVec{v}, 11)
	assert.Equal(t, len(encoded), 4+4+11)

	decoded, leftover, err := decodeIOVecs(encoded, nil)
	assert.OK(t, err)
	assert.Equal(t, len(leftover), 0)
	assert.DeepEqual(t, decoded, []wasi.IOVec{v})
}

func TestEncodeIOVecsPrefixShortSize(t *testing.T) {
	v := wasi.IOVec("hello world")

	encoded := encodeIOVecsPrefix(nil, []wasi.IOVec{v}, 5)
	assert.Equal(t, len(encoded), 4+4+5)

	decoded, leftover, err := decodeIOVecs(encoded, nil)
	assert.OK(t, err)
	assert.Equal(t, len(leftover), 0)
	assert.DeepEqual(t, decoded, []wasi.IOVec{v[:5]})
}

func TestEncodeIOVecsPrefixZeroSize(t *testing.T) {
	v := wasi.IOVec("hello world")

	encoded := encodeIOVecsPrefix(nil, []wasi.IOVec{v}, 0)
	assert.Equal(t, len(encoded), 4)

	decoded, leftover, err := decodeIOVecs(encoded, nil)
	assert.OK(t, err)
	assert.Equal(t, len(leftover), 0)
	assert.DeepEqual(t, decoded, ([]wasi.IOVec)(nil))
}

func TestEncodeIOVecsPrefixNegativeSize(t *testing.T) {
	v := wasi.IOVec("hello world")

	encoded := encodeIOVecsPrefix(nil, []wasi.IOVec{v}, ^wasi.Size(0))
	assert.Equal(t, len(encoded), 4)

	decoded, leftover, err := decodeIOVecs(encoded, nil)
	assert.OK(t, err)
	assert.Equal(t, len(leftover), 0)
	assert.DeepEqual(t, decoded, ([]wasi.IOVec)(nil))
}
