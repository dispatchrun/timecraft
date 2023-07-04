import unittest

import select
import socket
import ssl
import http.client


class TestStdlib(unittest.TestCase):
    def test_socket_https(self):
        hostname = 'example.com'
        context = ssl.create_default_context()

        buf = bytearray(1024)
        view = memoryview(buf)

        req = b"GET / HTTP/1.1\r\nHost: example.com:443\r\n\r\n"
        with socket.create_connection((hostname, 443)) as sock:
            with context.wrap_socket(sock, server_hostname=hostname) as ssock:
                ssock.sendall(req)
                nbytes = ssock.recv_into(view, 1024)
                self.assertTrue(nbytes > 0)
        prefix = b"HTTP/1.1 200 OK"
        self.assertEqual(prefix, buf[:len(prefix)])

    def test_socket_https_nonblocking(self):
        hostname = 'example.com'
        context = ssl.create_default_context()

        buf = bytearray(1024)
        view = memoryview(buf)

        # Can't use with socket.create_connection because it raises when
        # connect returns EINPROGRESS.
        sock = socket.socket()
        sock.setblocking(False)
        try:
            sock.connect((hostname, 443))
        except BlockingIOError:
            pass

        req = b"GET / HTTP/1.1\r\nHost: example.com:443\r\n\r\n"
        try:
            with context.wrap_socket(sock, server_hostname=hostname) as ssock:
                poller = select.poll()
                poller.register(ssock, select.POLLOUT | select.POLLIN)
                running = True
                while running:
                    evts = poller.poll(5000)
                    for s, e in evts:
                        if e & select.POLLOUT and ssock.fileno() == s:
                            ssock.sendall(req)
                            running = False
                running = True
                while running:
                    evts = poller.poll(5000)
                    for s, e in evts:
                        if e & select.POLLIN and ssock.fileno() == s:
                            nbytes = ssock.recv_into(view, 1024)
                            self.assertTrue(nbytes > 0)
                            running = False
        finally:
            sock.close()
        prefix = b"HTTP/1.1 200 OK"
        self.assertEqual(prefix, buf[:len(prefix)])

    def test_basic_http_get(self):
        conn = http.client.HTTPConnection("neverssl.com")
        conn.request("GET", "/", headers={"Host": "neverssl.com"})
        response = conn.getresponse()
        self.assertEqual(response.status, 200)
        body = response.read()
        self.assertTrue(len(body) > 0)
        conn.close()

    def test_basic_https_get(self):
        conn = http.client.HTTPSConnection("postman-echo.com")
        conn.request("GET", "/get?foo1=bar1&foo2=bar2",
                     headers={"Host": "postman-echo.com"})
        response = conn.getresponse()
        self.assertEqual(response.status, 200)
        body = response.read()
        self.assertTrue(len(body) > 0)
        conn.close()
