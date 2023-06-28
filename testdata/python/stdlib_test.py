import unittest

import http.client


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
        conn.request("GET", "/get?foo1=bar1&foo2=bar2",
                     headers={"Host": "postman-echo.com"})
        response = conn.getresponse()
        print(response.status, response.reason)
        self.assertEqual(response.status, 200)
        body = response.read()
        self.assertTrue(len(body) > 0)
        conn.close()
