package sandbox

import (
	"testing"

	"github.com/stealthrocket/timecraft/internal/assert"
	"github.com/stealthrocket/wasi-go"
)

func TestSocketBuffer(t *testing.T) {
	tests := []struct {
		scenario string
		function func(*testing.T)
	}{
		{
			scenario: "reading from an empty socket buffer",
			function: testSocketBufferReadEmpty,
		},

		{
			scenario: "data written to a socket buffer can be read back",
			function: testSocketBufferWriteAndRead,
		},

		{
			scenario: "data written to a socket buffer can be read back in chunks",
			function: testSocketBufferWriteAndReadChunks,
		},

		{
			scenario: "data written to a socket buffer can be read back into a vector",
			function: testSocketBufferWriteAndReadVector,
		},

		{
			scenario: "cannot write a message larger than the socket buffer",
			function: testSocketBufferWriteMessageTooLarge,
		},

		{
			scenario: "cannot write when the socket buffer is full",
			function: testSocketBufferWriteFull,
		},

		{
			scenario: "source address and packet association is retained",
			function: testSocketBufferAssociateAddressAndData,
		},
	}

	for _, test := range tests {
		t.Run(test.scenario, test.function)
	}
}

func testSocketBufferReadEmpty(t *testing.T) {
	s := newSocketBuffer[ipv4](nil, 0)
	b := make([]byte, 32)

	n, flags, _, errno := s.recv([]wasi.IOVec{b}, 0)
	assert.Equal(t, n, ^wasi.Size(0))
	assert.Equal(t, flags, 0)
	assert.Equal(t, errno, wasi.EAGAIN)
}

func testSocketBufferWriteAndRead(t *testing.T) {
	s := newSocketBuffer[ipv4](nil, 64)
	b := make([]byte, 32)

	addr1 := ipv4{
		addr: [4]byte{127, 0, 0, 1},
		port: 4242,
	}

	n, errno := s.send([]wasi.IOVec{[]byte("Hello World!")}, addr1)
	assert.Equal(t, n, 12)
	assert.Equal(t, errno, wasi.ESUCCESS)

	n, flags, addr2, errno := s.recv([]wasi.IOVec{b}, 0)
	assert.Equal(t, n, 12)
	assert.Equal(t, flags, 0)
	assert.Equal(t, addr2, addr1)
	assert.Equal(t, errno, wasi.ESUCCESS)
	assert.Equal(t, string(b[:n]), "Hello World!")

	n, flags, addr3, errno := s.recv([]wasi.IOVec{b}, 0)
	assert.Equal(t, n, ^wasi.Size(0))
	assert.Equal(t, flags, 0)
	assert.Equal(t, addr3, ipv4{})
	assert.Equal(t, errno, wasi.EAGAIN)
}

func testSocketBufferWriteAndReadChunks(t *testing.T) {
	s := newSocketBuffer[ipv4](nil, 64)
	b := make([]byte, 32)

	addr1 := ipv4{
		addr: [4]byte{127, 0, 0, 1},
		port: 4242,
	}

	n, errno := s.send([]wasi.IOVec{[]byte("Hello")}, addr1)
	assert.Equal(t, n, 5)
	assert.Equal(t, errno, wasi.ESUCCESS)

	n, errno = s.send([]wasi.IOVec{[]byte(" ")}, addr1)
	assert.Equal(t, n, 1)
	assert.Equal(t, errno, wasi.ESUCCESS)

	n, errno = s.send([]wasi.IOVec{[]byte("World!")}, addr1)
	assert.Equal(t, n, 6)
	assert.Equal(t, errno, wasi.ESUCCESS)

	for _, c := range []byte("Hello World!") {
		n, flags, addr2, errno := s.recv([]wasi.IOVec{b[:1]}, 0)
		assert.Equal(t, n, 1)
		assert.Equal(t, flags, 0)
		assert.Equal(t, addr2, addr1)
		assert.Equal(t, errno, wasi.ESUCCESS)
		assert.Equal(t, string(b[:n]), string([]byte{c}))
	}

	n, flags, addr3, errno := s.recv([]wasi.IOVec{b}, 0)
	assert.Equal(t, n, ^wasi.Size(0))
	assert.Equal(t, flags, 0)
	assert.Equal(t, addr3, ipv4{})
	assert.Equal(t, errno, wasi.EAGAIN)
}

func testSocketBufferWriteAndReadVector(t *testing.T) {
	s := newSocketBuffer[ipv4](nil, 64)
	b := make([]byte, 32)

	addr1 := ipv4{
		addr: [4]byte{127, 0, 0, 1},
		port: 4242,
	}

	iov1 := []wasi.IOVec{
		[]byte("Hello"),
		[]byte(" "),
		[]byte("World!"),
	}

	n, errno := s.send(iov1, addr1)
	assert.Equal(t, n, 12)
	assert.Equal(t, errno, wasi.ESUCCESS)

	iov2 := []wasi.IOVec{
		make([]byte, 4),
		make([]byte, 4),
		make([]byte, 3),
		make([]byte, 3),
	}

	n, flags, addr2, errno := s.recv(iov2, 0)
	assert.Equal(t, n, 12)
	assert.Equal(t, flags, 0)
	assert.Equal(t, addr2, addr1)
	assert.Equal(t, errno, wasi.ESUCCESS)

	b = b[:0]
	b = append(b, iov2[0]...)     // "Hell"
	b = append(b, iov2[1]...)     // "o Wo"
	b = append(b, iov2[2]...)     // "rld"
	b = append(b, iov2[3][:1]...) // "!"
	assert.Equal(t, string(b), "Hello World!")

	n, flags, addr3, errno := s.recv([]wasi.IOVec{b}, 0)
	assert.Equal(t, n, ^wasi.Size(0))
	assert.Equal(t, flags, 0)
	assert.Equal(t, addr3, ipv4{})
	assert.Equal(t, errno, wasi.EAGAIN)
}

func testSocketBufferWriteMessageTooLarge(t *testing.T) {
	s := newSocketBuffer[ipv4](nil, 10)

	addr := ipv4{
		addr: [4]byte{127, 0, 0, 1},
		port: 4242,
	}

	n, errno := s.sendmsg([]wasi.IOVec{[]byte("Hello, World!")}, addr)
	assert.Equal(t, n, ^wasi.Size(0))
	assert.Equal(t, errno, wasi.EMSGSIZE)
}

func testSocketBufferWriteFull(t *testing.T) {
	s := newSocketBuffer[ipv4](nil, 11)

	addr := ipv4{
		addr: [4]byte{127, 0, 0, 1},
		port: 4242,
	}

	n, errno := s.sendmsg([]wasi.IOVec{[]byte("hello world")}, addr)
	assert.Equal(t, n, 11)
	assert.Equal(t, errno, wasi.ESUCCESS)

	n, errno = s.sendmsg([]wasi.IOVec{[]byte("!")}, addr)
	assert.Equal(t, n, ^wasi.Size(0))
	assert.Equal(t, errno, wasi.EAGAIN)
}

func testSocketBufferAssociateAddressAndData(t *testing.T) {
	s := newSocketBuffer[ipv4](nil, 100)
	b := make([]byte, 32)

	addr1 := ipv4{
		addr: [4]byte{127, 0, 0, 1},
		port: 4242,
	}

	addr2 := ipv4{
		addr: [4]byte{127, 0, 0, 1},
		port: 8484,
	}

	n, errno := s.sendmsg([]wasi.IOVec{[]byte("hello, world!")}, addr1)
	assert.Equal(t, n, 13)
	assert.Equal(t, errno, wasi.ESUCCESS)

	n, errno = s.sendmsg([]wasi.IOVec{[]byte("how are you?")}, addr2)
	assert.Equal(t, n, 12)
	assert.Equal(t, errno, wasi.ESUCCESS)

	n, flags, src1, errno := s.recvmsg([]wasi.IOVec{b}, 0)
	assert.Equal(t, n, 13)
	assert.Equal(t, flags, 0)
	assert.Equal(t, src1, addr1)
	assert.Equal(t, errno, wasi.ESUCCESS)
	assert.Equal(t, string(b[:n]), "hello, world!")

	n, flags, src2, errno := s.recvmsg([]wasi.IOVec{b}, 0)
	assert.Equal(t, n, 12)
	assert.Equal(t, flags, 0)
	assert.Equal(t, src2, addr2)
	assert.Equal(t, errno, wasi.ESUCCESS)
	assert.Equal(t, string(b[:n]), "how are you?")
}
