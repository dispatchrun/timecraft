import unittest

import requests


class TestRequests(unittest.TestCase):
    def test_basic_http_get(self):
        r = requests.get("http://neverssl.com")
        self.assertTrue(len(r.headers) > 0)
        self.assertEqual(r.status_code, 200)
        self.assertTrue(len(r.text) > 0)

    def test_timecraft_runtime_endpoint(self):
        u = "http://0.0.0.0:3001/timecraft.server.v1.TimecraftService/Version"
        r = requests.post(u, json={})
        self.assertEqual(r.status_code, 200)
        self.assertTrue(len(r.text) > 0)

    def test_echo_https_get(self):
        r = requests.get("https://postman-echo.com/get?foo1=bar1&foo2=bar2")
        self.assertEqual(r.status_code, 200)
        b = r.json()
        b["headers"].pop("x-amzn-trace-id", None)
        b["headers"].pop("cookie", None)
        self.assertDictEqual(b, {
            "args": {
                "foo1": "bar1",
                "foo2": "bar2"
            },
            "headers": {
                "x-forwarded-proto": "https",
                "x-forwarded-port": "443",
                "host": "postman-echo.com",
                "user-agent": "python-requests/2.31.0",
                "accept": "*/*",
                "accept-encoding": "gzip, deflate",
            },
            "url": "https://postman-echo.com/get?foo1=bar1&foo2=bar2"
        })

    def test_echo_https_post_body_string(self):
        data = "This is expected to be sent back as part of response body."
        r = requests.post("https://postman-echo.com/post", data=data)
        self.assertEqual(r.status_code, 200)
        b = r.json()
        b["headers"].pop("x-amzn-trace-id", None)
        b["headers"].pop("cookie", None)
        b["headers"].pop("content-length", None)
        b["headers"].pop("transfer-encoding", None)

        self.assertDictEqual(b, {
            "args": {},
            "data": data,
            "files": {},
            "form": {},
            "headers": {"accept": "*/*",
                        "accept-encoding": "gzip, deflate",
                        "content-type": "application/json",
                        "host": "postman-echo.com",
                        "user-agent": "python-requests/2.31.0",
                        "x-forwarded-port": "443",
                        "x-forwarded-proto": "https"},
            "json": None,
            "url": "https://postman-echo.com/post"})
