import socketserver


class UnixSocketHttpServer(socketserver.UnixStreamServer):
    """UnixSocketHttpServer is an HTTP server that listens
    for connections on a unix socket."""

    def get_request(self):
        """get_request gets the next request. Since the connection
        is via a unix socket and the peer address is not available,
        @ is returned as the peer address."""
        request, _ = super(UnixSocketHttpServer, self).get_request()
        return request, ["@", 0]


def start_worker(handler):
    """start_worker starts a worker to process work from the timecraft
    runtime. The provided handler should be a subclass of
    http.server.BaseHTTPRequestHandler."""
    server = UnixSocketHttpServer("@timecraft.work.sock", handler)
    server.serve_forever()
