"""
See timecraft_test.py => TestTimecraft.test_spawn
"""

from http.server import HTTPServer, BaseHTTPRequestHandler


class Handler(BaseHTTPRequestHandler):

    def do_GET(self):
        self.send_response(200)
        self.end_headers()
        self.wfile.flush()

    def log_message(self, format, *args) -> None:
        pass


if __name__ == "__main__":
    HTTPServer(("0.0.0.0", 3000), Handler).serve_forever()
