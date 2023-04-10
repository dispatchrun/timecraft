package timecraft

import (
	"context"

	. "github.com/stealthrocket/wazergo/types"
)

func (m *Module) httpClientCreate(
	ctx context.Context,
	enableCompression Bool,
	enableKeepAlive Bool,
	maxIdleConns Int32,
	maxConnsPerHost Duration,
	idleConnTimeout Duration,
	responseHeaderTimeout Duration,
	expectContinueTimeout Duration,
	proxyConnectHeader Bytes,
	maxResponseHeaderBytes Int64,
	writeBufferSize Int32,
	readBufferSize Int32,
	clientID Pointer[Int64],
) Errno {
	return errNotImplemented
}

func (m *Module) httpClientShutdown(
	ctx context.Context,
	clientID Int64,
) Errno {
	return errNotImplemented
}

func (m *Module) httpClientExchangeCreate(
	ctx context.Context,
	clientID Int64,
	timeout Duration,
	exchangeID Pointer[Int64],
) Errno {
	return errNotImplemented
}

func (m *Module) httpClientExchangeWriteRequestHeader(
	ctx context.Context,
	exchangeID Int64,
	method Bytes,
	url Bytes,
	header Bytes,
	contentLength Int64,
) Errno {
	return errNotImplemented
}

func (m *Module) httpClientExchangeWriteRequestBody(
	ctx context.Context,
	exchangeID Int64,
	data Bytes,
	nwritten Pointer[Int32],
) Errno {
	return errNotImplemented
}

func (m *Module) httpClientExchangeWriteRequestTrailer(
	ctx context.Context,
	exchangeID Int64,
	trailer Bytes,
) Errno {
	return errNotImplemented
}

func (m *Module) httpClientExchangeSendRequest(
	ctx context.Context,
	exchangeID Int64,
) Errno {
	return errNotImplemented
}

func (m *Module) httpClientExchangeReadResponseHeader(
	ctx context.Context,
	exchangeID Int64,
	protoMajor Pointer[Int32],
	protoMinor Pointer[Int32],
	statusCode Pointer[Int32],
	status Bytes,
	header Bytes,
	contentLength Pointer[Int64],
) Errno {
	return errNotImplemented
}

func (m *Module) httpClientExchangeReadResponseBody(
	ctx context.Context,
	exchangeID Int64,
	data Bytes,
	nread Pointer[Int32],
) Errno {
	return errNotImplemented
}

func (m *Module) httpClientExchangeReadResponseTrailer(
	ctx context.Context,
	exchangeID Int64,
	trailer Bytes,
) Errno {
	return errNotImplemented
}

func (m *Module) httpClientExchangeClose(
	ctx context.Context,
	exchangeID Int64,
) Errno {
	return errNotImplemented
}

func (m *Module) httpServerCreate(
	ctx context.Context,
	addr Bytes,
	readTimeout Duration,
	readHeaderTimeout Duration,
	writeTimeout Duration,
	idleTimeout Duration,
	maxHeaderBytes Int64,
	serverID Pointer[Int64],
) Errno {
	return errNotImplemented
}

func (m *Module) httpServerShutdown(
	ctx context.Context,
	serverID Int64,
) Errno {
	return errNotImplemented
}

func (m *Module) httpServerExchangeCreate(
	ctx context.Context,
	serverID Int64,
	exchangeID Pointer[Int64],
) Errno {
	return errNotImplemented
}

func (m *Module) httpServerExchangeReadRequestHeader(
	ctx context.Context,
	exchangeID Int64,
	method Bytes,
	url Bytes,
	header Bytes,
	contentLength Pointer[Int64],
) Errno {
	return errNotImplemented
}

func (m *Module) httpServerExchangeReadRequestBody(
	ctx context.Context,
	exchangeID Int64,
	data Bytes,
	nread Pointer[Int32],
) Errno {
	return errNotImplemented
}

func (m *Module) httpServerExchangeReadRequestTrailer(
	ctx context.Context,
	exchangeID Int64,
	trailer Bytes,
) Errno {
	return errNotImplemented
}

func (m *Module) httpServerExchangeWriteResponseHeader(
	ctx context.Context,
	exchangeID Int64,
	statusCode Int32,
	status Bytes,
	header Bytes,
	contentLength Int64,
) Errno {
	return errNotImplemented
}

func (m *Module) httpServerExchangeWriteResponseBody(
	ctx context.Context,
	exchangeID Int64,
	data Bytes,
	nwritten Pointer[Int32],
) Errno {
	return errNotImplemented
}

func (m *Module) httpServerExchangeWriteResponseTrailer(
	ctx context.Context,
	exchangeID Int64,
	trailer Bytes,
) Errno {
	return errNotImplemented
}

func (m *Module) httpServerExchangeClose(
	ctx context.Context,
	exchangeID Int64,
) Errno {
	return errNotImplemented
}
