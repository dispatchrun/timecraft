# Execution tracing

Timecraft can generate traces from the record of execution. These traces weren't emitted
during execution, they are generated after the fact.

## System Call Tracing

To show a trace of [WASI](https://wasi.dev/) system calls made by the WebAssembly module, use:

```console
$ timecraft run --trace <MODULE>
```

This is similar to the [`strace`](https://man7.org/linux/man-pages/man1/strace.1.html) tool
on Linux.

To show a trace of WASI system calls made by a process that was running in the past, use:

```console
$ timecraft replay --trace <ID>
```

## Network Tracing

Timecraft can trace network activity.

Here's an example application that makes an HTTP request:

```
$ timecraft run testdata/go/get.wasm https://eo3qh2ncelpc9q0.m.pipedream.net
b5770968-2ec7-4307-b613-61e8e9b2f9d2
Hello, World!
```

To trace low-level networking activity, use:

```
$ timecraft trace network b5770968-2ec7-4307-b613-61e8e9b2f9d2
2023/06/30 08:13:20.463329 TCP 172.16.0.0:49152 > 44.194.85.187:443: CONN EINPROGRESS
2023/06/30 08:13:20.947931 TCP 172.16.0.0:49152 > 44.194.85.187:443: SEND OK 112
2023/06/30 08:13:24.326929 TCP 44.194.85.187:443 > 172.16.0.0:49152: RECV OK 209
```

To show bytes transmitted, use `-v`:

```
$ timecraft trace network -v b5770968-2ec7-4307-b613-61e8e9b2f9d2
[23] 2023/06/30 08:13:20.463329 TCP 172.16.0.0:49152 > 44.194.85.187:443: CONN EINPROGRESS
[40] 2023/06/30 08:13:20.947931 TCP 172.16.0.0:49152 > 44.194.85.187:443: SEND OK 112

00000000  47 45 54 20 2f 20 48 54  54 50 2f 31 2e 31 0d 0a  |GET / HTTP/1.1..|
00000010  48 6f 73 74 3a 20 65 6f  33 71 68 32 6e 63 65 6c  |Host: eo3qh2ncel|
00000020  70 63 39 71 30 2e 6d 2e  70 69 70 65 64 72 65 61  |pc9q0.m.pipedrea|
00000030  6d 2e 6e 65 74 0d 0a 55  73 65 72 2d 41 67 65 6e  |m.net..User-Agen|
00000040  74 3a 20 47 6f 2d 68 74  74 70 2d 63 6c 69 65 6e  |t: Go-http-clien|
00000050  74 2f 31 2e 31 0d 0a 41  63 63 65 70 74 2d 45 6e  |t/1.1..Accept-En|
00000060  63 6f 64 69 6e 67 3a 20  67 7a 69 70 0d 0a 0d 0a  |coding: gzip....|

[45] 2023/06/30 08:13:24.326929 TCP 44.194.85.187:443 > 172.16.0.0:49152: RECV OK 209

00000000  48 54 54 50 2f 31 2e 31  20 32 30 30 20 4f 4b 0d  |HTTP/1.1 200 OK.|
00000010  0a 44 61 74 65 3a 20 54  68 75 2c 20 32 39 20 4a  |.Date: Thu, 29 J|
00000020  75 6e 20 32 30 32 33 20  32 32 3a 31 33 3a 32 34  |un 2023 22:13:24|
00000030  20 47 4d 54 0d 0a 43 6f  6e 74 65 6e 74 2d 54 79  | GMT..Content-Ty|
00000040  70 65 3a 20 74 65 78 74  2f 68 74 6d 6c 3b 20 63  |pe: text/html; c|
00000050  68 61 72 73 65 74 3d 75  74 66 2d 38 0d 0a 43 6f  |harset=utf-8..Co|
00000060  6e 74 65 6e 74 2d 4c 65  6e 67 74 68 3a 20 31 34  |ntent-Length: 14|
00000070  0d 0a 43 6f 6e 6e 65 63  74 69 6f 6e 3a 20 6b 65  |..Connection: ke|
00000080  65 70 2d 61 6c 69 76 65  0d 0a 58 2d 50 6f 77 65  |ep-alive..X-Powe|
00000090  72 65 64 2d 42 79 3a 20  45 78 70 72 65 73 73 0d  |red-By: Express.|
000000a0  0a 41 63 63 65 73 73 2d  43 6f 6e 74 72 6f 6c 2d  |.Access-Control-|
000000b0  41 6c 6c 6f 77 2d 4f 72  69 67 69 6e 3a 20 2a 0d  |Allow-Origin: *.|
000000c0  0a 0d 0a 48 65 6c 6c 6f  2c 20 57 6f 72 6c 64 21  |...Hello, World!|
000000d0  0a                                                |.|
```

To trace HTTP requests, use:

```
$ timecraft trace request b5770968-2ec7-4307-b613-61e8e9b2f9d2
2023/06/30 08:13:24.326929 HTTP 172.16.0.0:49152 > 44.194.85.187:443: GET / => 200 OK
```

To show HTTP headers/body, use `-v`:

```
$ timecraft trace request -v b5770968-2ec7-4307-b613-61e8e9b2f9d2
2023/06/30 08:13:24.326929 HTTP 172.16.0.0:49152 > 44.194.85.187:443
> GET / HTTP/1.1
> Host: eo3qh2ncelpc9q0.m.pipedream.net
> User-Agent: Go-http-client/1.1
> Accept-Encoding: gzip
>
< HTTP/1.1 200 OK
< Date: Thu, 29 Jun 2023 22:13:24 GMT
< Content-Type: text/html; charset=utf-8
< Content-Length: 14
< Connection: keep-alive
< X-Powered-By: Express
< Access-Control-Allow-Origin: *
<
Hello, World!
```

## Additional Capabilities

Additional tracing capabilities are planned, see https://github.com/stealthrocket/timecraft/issues?q=is%3Aissue+is%3Aopen+trace.
