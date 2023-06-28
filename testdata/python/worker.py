"""
See timecraft_test.py => TestTimecraft.test_tasks
"""

import http.server

import timecraft


client = timecraft.Client()


class Handler(http.server.BaseHTTPRequestHandler):

    def do_POST(self):
        try:
            content_length = int(self.headers.get("Content-Length"))
            body = self.rfile.read(content_length)
            if self.path[1:] != body.decode("utf-8"):
                raise RuntimeError("unexpected body")

            if self.headers["X-Foo"] != "bar":
                raise RuntimeError("expected X-Foo header")

            self.send_response(200)

            for k, v in self.headers.items():
                if k.startswith("X-Timecraft"):
                    self.send_header(k, v)
            self.end_headers()

            self.wfile.write(body)

        except Exception as e:
            self.send_response(500)
            self.end_headers()
            self.wfile.write(str(e).encode("utf-8"))

        finally:
            self.wfile.flush()

    def log_message(self, format, *args) -> None:
        pass


if __name__ == "__main__":
    timecraft.start_worker(Handler)
