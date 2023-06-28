from http.server import HTTPServer
from .client import Client


def start_worker(handler):
    """start_worker starts a worker to process work from the timecraft
    runtime. The provided handler should be a subclass of
    http.server.BaseHTTPRequestHandler."""
    client = Client()
    logger = client.logger()
    logger.info("starting worker")

    server = HTTPServer(("0.0.0.0", 3000), handler)
    server.serve_forever()
