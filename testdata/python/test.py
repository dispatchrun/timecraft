import unittest

import http.client
import requests


class TestStdlib(unittest.TestCase):
    def test_basic_http_get(self):
        conn = http.client.HTTPConnection("neverssl.com")
        conn.request("GET", "/", headers={"Host": "neverssl.com"})
        response = conn.getresponse()
        print(response.status, response.reason)
        self.assertEqual(response.status, 200)
        body = response.read()
        self.assertTrue(len(body) > 0)
        conn.close()

    def test_basic_https_get(self):
        conn = http.client.HTTPSConnection("postman-echo.com")
        conn.request("GET", "/get?foo1=bar1&foo2=bar2", headers={"Host": "postman-echo.com"})
        response = conn.getresponse()
        print(response.status, response.reason)
        self.assertEqual(response.status, 200)
        body = response.read()
        self.assertTrue(len(body) > 0)
        conn.close()


class TestRequests(unittest.TestCase):
    def test_basic_http_get(self):
        r = requests.get("http://neverssl.com")
        self.assertTrue(len(r.headers) > 0)
        self.assertEqual(r.status_code, 200)
        self.assertTrue(len(r.text) > 0)

    @unittest.skip("skip until test.py is setup to run in a harness")
    def test_basic_unix_post(self):
        r = requests.post("http+unix://\0timecraft.sock/timecraft.server.v1.TimecraftService/Version", json={})
        self.assertEqual(r.status_code, 200)
        self.assertTrue(len(r.text)>0)

    def test_echo_https_get(self):
        r = requests.get('https://postman-echo.com/get?foo1=bar1&foo2=bar2')
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
        r = requests.post('https://postman-echo.com/post', data="This is expected to be sent back as part of response body.")
        self.assertEqual(r.status_code, 200)
        b = r.json()
        b["headers"].pop("x-amzn-trace-id", None)
        b["headers"].pop("cookie", None)
        b["headers"].pop("content-length", None)
        b["headers"].pop("transfer-encoding", None)

        self.assertDictEqual(b, {
            'args': {},
            'data': 'This is expected to be sent back as part of response body.',
            'files': {},
            'form': {},
            'headers': {'accept': '*/*',
                        'accept-encoding': 'gzip, deflate',
                        'content-type': 'application/json',
                        'host': 'postman-echo.com',
                        'user-agent': 'python-requests/2.31.0',
                        'x-forwarded-port': '443',
                        'x-forwarded-proto': 'https'},
            'json': None,
            'url': 'https://postman-echo.com/post'})
