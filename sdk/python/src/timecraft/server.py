from http.server import HTTPServer
from .client import Client


def serve_forever(address, handler):
    """serve_forever starts a worker to process work from the timecraft
    runtime. The address argument should be of the same type as the first
    argument passed to http.server.HTTPServer (e.g. a tuple with the IP and
    port to listen on). The handler should be a subclass of
    http.server.BaseHTTPRequestHandler."""
    client = Client()
    client.logger.info("starting worker")

    server = HTTPServer(address, handler)
    server.serve_forever()
