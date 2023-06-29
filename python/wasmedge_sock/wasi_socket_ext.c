
/* Wasmedge v2 implementation as a static library.
 *
 * Base on the work of:
 * vmware-labs see LICENSE-vmware
 * hangedfish see LICENSE-hangedfish
 */

#include <byteswap.h>
#include <errno.h>
#include <memory.h>
#include <netinet/in.h>
#include <netinet/tcp.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/param.h>
#include <sys/un.h>

#include "include/netdb.h"
#include "include/sys/socket.h"

// #define WASMEDGE_SOCKET_DEBUG

#ifdef WASMEDGE_SOCKET_DEBUG
#define WSEDEBUG(fmt, ...) fprintf(stderr, fmt __VA_OPT__(, ) __VA_ARGS__)
#else
#define WSEDEBUG(fmt, ...)
#endif

/*
 * WASMEDGE V2 SOCKET API
 *
 * Constants start with WE_ to avoid confusion with posix constants.
 */

typedef int32_t we_protocol_family;

#define WE_UNSPECIFIED_FAMILY 0
#define WE_INET_FAMILY 1
#define WE_INET6_FAMILY 2
#define WE_UNIX_FAMILY 3

typedef int32_t we_protocol;

#define WE_IP_PROTOCOL 0
#define WE_TCP_PROTOCOL 1
#define WE_UDP_PROTOCOL 2

typedef int32_t we_socket_type;

#define WE_ANY_SOCKET 0
#define WE_DATAGRAM_SOCKET 1
#define WE_STREAM_SOCKET 2

typedef int32_t we_socket_option_level;

#define WE_SOCKET_LEVEL 0
#define WE_TCP_LEVEL 6

typedef int32_t we_socket_option;

// socket options
#define WE_REUSE_ADDRESS 0
#define WE_QUERY_SOCKET_TYPE 1
#define WE_QUERY_SOCKET_ERROR 2
#define WE_DONT_ROUTE 3
#define WE_BROADCAST 4
#define WE_SEND_BUFFER_SIZE 5
#define WE_RECV_BUFER_SIZE 6
#define WE_KEEP_ALIVE 7
#define WE_OOB_INLINE 8
#define WE_LINGER 9
#define WE_RECV_LOW_WATERMARK 10
#define WE_RECV_TIMEOUT 11
#define WE_SEND_TIMEOUT 12
#define WE_QUERY_ACCEPT_CONNECTIONS 13
#define WE_BIND_TO_DEVICE 14
// tcp level options
#define WE_TCP_NO_DELAY 15

typedef struct {
  uint16_t family;
  union {
    uint8_t inet_addr[4];
    uint8_t inet6_addr[16];
    uint8_t unix_addr[126];
  };
} we_address_buffer;

typedef struct {
  we_address_buffer* buf;
  uint32_t len;
} we_address;

typedef struct {
  uint32_t family;
  uint32_t data_len;
  char* data;
  char padding[4];
} we_sock_addr;

typedef struct _we_sock_address_info {
  uint16_t flags;
  uint8_t family;
  uint8_t socket_type;
  uint32_t protocol;
  uint32_t address_length;
  we_sock_addr* address;
  char* canonical_name;
  uint32_t canonical_name_length;
  struct _we_sock_address_info* next;
} we_sock_address_info;

typedef struct {
  we_sock_address_info info;
  we_sock_addr addr;
  char data[26];
  char canonname[30];

} we_sock_addr_result;

typedef struct iovec_read {
  uint8_t* buf;
  uint32_t size;
} iovec_read_t;

typedef struct iovec_write {
  uint8_t* buf;
  uint32_t size;
} iovec_write_t;

#define SOL_RESERVED 0x74696d65

int32_t __import_wasip1_sock_open(we_protocol_family family,
                                  we_socket_type sock_type, int32_t* fd)
    __attribute__((__import_module__("wasi_snapshot_preview1"),
                   __import_name__("sock_open")));

int32_t __import_wasip1_sock_bind(int32_t fd, we_address* addr, uint32_t port)
    __attribute__((__import_module__("wasi_snapshot_preview1"),
                   __import_name__("sock_bind")));

uint32_t __import_wasip1_sock_listen(uint32_t fd, uint32_t backlog)
    __attribute__((__import_module__("wasi_snapshot_preview1"),
                   __import_name__("sock_listen")));

int32_t __import_wasip1_sock_accept(uint32_t fd, uint32_t flags, uint32_t* fd2)
    __attribute__((__import_module__("wasi_snapshot_preview1"),
                   __import_name__("sock_accept")));

int32_t __import_wasip1_sock_connect(uint32_t fd, we_address* addr,
                                     uint32_t port)
    __attribute__((__import_module__("wasi_snapshot_preview1"),
                   __import_name__("sock_connect")));

int32_t __import_wasip1_sock_recv(uint32_t fd, iovec_read_t* buf,
                                  uint32_t buf_count, uint16_t flags,
                                  uint32_t* recv_len, uint32_t* oflags)
    __attribute__((__import_module__("wasi_snapshot_preview1"),
                   __import_name__("sock_recv")));

int32_t __import_wasip1_sock_send(uint32_t fd, iovec_write_t buf,
                                  uint32_t buf_count, uint16_t flags,
                                  uint32_t* send_len)
    __attribute__((__import_module__("wasi_snapshot_preview1"),
                   __import_name__("sock_send")));

int32_t __import_wasip1_sock_shutdown(uint32_t fd, uint8_t flags)
    __attribute__((__import_module__("wasi_snapshot_preview1"),
                   __import_name__("sock_shutdown")));

int32_t __import_wasip1_sock_getaddrinfo(uint8_t* name, uint32_t name_len,
                                         uint8_t* service, uint32_t service_len,
                                         we_sock_address_info* hints,
                                         we_sock_address_info** respp,
                                         uint32_t max_res_len,
                                         uint32_t* res_lenp)
    __attribute__((__import_module__("wasi_snapshot_preview1"),
                   __import_name__("sock_getaddrinfo")));

int32_t __import_wasip1_sock_getlocaladdr(uint32_t fd, we_address* addr,
                                          uint32_t* port)
    __attribute__((__import_module__("wasi_snapshot_preview1"),
                   __import_name__("sock_getlocaladdr")));

int32_t __import_wasip1_sock_getpeeraddr(uint32_t fd, we_address* addr,
                                         uint32_t* port)
    __attribute__((__import_module__("wasi_snapshot_preview1"),
                   __import_name__("sock_getpeeraddr")));

int32_t __import_wasip1_sock_getsockopt(uint32_t fd,
                                        we_socket_option_level level,
                                        we_socket_option name, int32_t* flag,
                                        int32_t flag_size)
    __attribute__((__import_module__("wasi_snapshot_preview1"),
                   __import_name__("sock_getsockopt")));

int32_t __import_wasip1_sock_setsockopt(uint32_t fd,
                                        we_socket_option_level level,
                                        we_socket_option name, int32_t* value,
                                        uint32_t value_size)
    __attribute__((__import_module__("wasi_snapshot_preview1"),
                   __import_name__("sock_setsockopt")));

uint16_t ntohs(uint16_t n) {
  union {
    int i;
    char c;
  } u = {1};
  return u.c ? bswap_16(n) : n;
}

/*
 * CONVERSION FUNCTIONS
 *
 * utw_* functions convert unix enums into equivalent wasi enums.
 * wtu_* functions convert wasi enums into equivalent unix enums.
 *
 * All those functions panic on unknown or unhandled values.
 */

int wtu_protocol_family(we_protocol_family fam) {
  switch (fam) {
    case WE_UNSPECIFIED_FAMILY:
      return AF_UNSPEC;
    case WE_INET_FAMILY:
      return AF_INET;
    case WE_INET6_FAMILY:
      return AF_INET6;
    case WE_UNIX_FAMILY:
      return AF_UNIX;
  }
  printf("!! unsupported wasm socket family %d\n", fam);
  abort();
}

int wtu_sock_type(we_socket_type t) {
  switch (t) {
    case WE_ANY_SOCKET:
      return 0;
    case WE_DATAGRAM_SOCKET:
      return SOCK_DGRAM;
    case WE_STREAM_SOCKET:
      return SOCK_STREAM;
  }
  printf("!! unsupported socket type %d\n", t);
  abort();
}

int wtu_protocol(we_protocol p) {
  switch (p) {
    case WE_IP_PROTOCOL:
      return IPPROTO_IP;
    case WE_TCP_PROTOCOL:
      return IPPROTO_TCP;
    case WE_UDP_PROTOCOL:
      return IPPROTO_UDP;
  }
  printf("!! unsupported wasi socket protocol %d\n", p);
  abort();
}

we_protocol_family utw_sock_domain(int domain) {
  switch (domain) {
    case AF_UNSPEC:
      return WE_UNSPECIFIED_FAMILY;
    case AF_INET:
      return WE_INET_FAMILY;
    case AF_INET6:
      return WE_INET6_FAMILY;
    case AF_UNIX:
      return WE_UNIX_FAMILY;
  }
  printf("!! unsupported unix socket domain %d\n", domain);
  abort();
}

we_socket_type utw_sock_type(int type) {
  // Clear the SOCK_NONBLOCK and SOCK_CLOEXEC flags by keeping only the first
  // byte of type.
  type &= 0xFF;
  switch (type) {
    case 0:
      return WE_ANY_SOCKET;
    case SOCK_STREAM:
      return WE_STREAM_SOCKET;
    case SOCK_DGRAM:
      return WE_DATAGRAM_SOCKET;
  }
  printf("!! unsupported socket type %d\n", type);
  abort();
}

we_protocol utw_sock_proto(int proto) {
  switch (proto) {
    case IPPROTO_IP:
      return WE_IP_PROTOCOL;
    case IPPROTO_TCP:
      return WE_TCP_PROTOCOL;
    case IPPROTO_UDP:
      return WE_UDP_PROTOCOL;
  }
  printf("!! unsupported posix socket protocol %d\n", proto);
  abort();
}

we_socket_option_level utw_sock_option_level(int level) {
  switch (level) {
    case SOL_SOCKET:
      return WE_SOCKET_LEVEL;
    case IPPROTO_TCP:
      return WE_TCP_LEVEL;
    case SOL_RESERVED:
      return SOL_RESERVED;
    default:
      printf("!! unsupported socket option level: %d\n", level);
      abort();
  }
}

we_socket_option utw_sock_option(int level, int name) {
  if (level == SOL_RESERVED) {
    return name;
  }

  if (level == IPPROTO_TCP) {
    switch (name) {
      case TCP_NODELAY:
        return WE_TCP_NO_DELAY;
      default:
        printf("!! unsupported tcp option: %d\n", name);
        abort();
    }
  }

  switch (name) {
    case SO_REUSEADDR:
      return WE_REUSE_ADDRESS;
    case SO_TYPE:
      return WE_QUERY_SOCKET_TYPE;
    case SO_ERROR:
      return WE_QUERY_SOCKET_ERROR;
    case SO_DONTROUTE:
      return WE_DONT_ROUTE;
    case SO_BROADCAST:
      return WE_BROADCAST;
    case SO_SNDBUF:
      return WE_SEND_BUFFER_SIZE;
    case SO_RCVBUF:
      return WE_RECV_BUFER_SIZE;
    case SO_KEEPALIVE:
      return WE_KEEP_ALIVE;
    case SO_OOBINLINE:
      return WE_OOB_INLINE;
    case SO_LINGER:
      return WE_LINGER;
    case SO_RCVLOWAT:
      return WE_RECV_LOW_WATERMARK;
    case SO_RCVTIMEO:
      return WE_RECV_TIMEOUT;
    case SO_SNDTIMEO:
      return WE_SEND_TIMEOUT;
    case SO_ACCEPTCONN:
      return WE_QUERY_ACCEPT_CONNECTIONS;
    case SO_BINDTODEVICE:
      return WE_BIND_TO_DEVICE;
    default:
      printf("!! unsupported socket option: %d\n", name);
      abort();
  }
}

// Fill out a wasmedge address buffer from a unix sockaddr. It returns a
// we_address that points to this buffer ready to pass to host functions.
we_address sockaddr_to_we_address(we_address_buffer* out,
                                  const struct sockaddr* addr) {
  memset(out, 0, sizeof(we_address_buffer));
  switch (addr->sa_family) {
    case AF_INET: {
      struct sockaddr_in* sin = (struct sockaddr_in*)addr;
      out->family = WE_INET_FAMILY;
      memcpy(out->inet_addr, &sin->sin_addr, 4);
    } break;
    case AF_INET6: {
      struct sockaddr_in6* sin6 = (struct sockaddr_in6*)addr;
      out->family = WE_INET6_FAMILY;
      memcpy(out->inet_addr, &sin6->sin6_addr, 16);
    } break;
    case AF_UNIX: {
      struct sockaddr_un* sun = (struct sockaddr_un*)addr;
      out->family = WE_UNIX_FAMILY;
      memcpy(&out->unix_addr, &sun->sun_path, 108);
      // hack for abstract unix sockets
      if (out->unix_addr[0] == 0) {
        out->unix_addr[0] = '@';
      }
    } break;
    default:
      printf("!! unsupported family: %d\n", addr->sa_family);
      abort();
  }

  we_address out_addr = {.buf = out, .len = 128};
  return out_addr;
}

// Retrieves the port from a sockadrr and converts it to little endian.
uint16_t sockaddr_port(const struct sockaddr* addr) {
  switch (addr->sa_family) {
    case AF_INET:
      return ntohs(((struct sockaddr_in*)addr)->sin_port);
    case AF_INET6:
      return ntohs(((struct sockaddr_in6*)addr)->sin6_port);
    case AF_UNIX:
      return 0;
    default:
      printf("!! unsupported family in connect: %d\n", addr->sa_family);
      abort();
  }
}

/*
 * SYS/SOCKET LIBC IMPLEMENTATION
 */

int socket(int domain, int type, int _protocol) {
  WSEDEBUG("WSME| socket domain=%d type=%d _protocol=%d\n", domain, type,
           _protocol);
  int fd;
  we_protocol_family af = utw_sock_domain(domain);
  we_socket_type st = utw_sock_type(type);
  int res = __import_wasip1_sock_open((int8_t)af, (int8_t)st, &fd);
  if (0 != res) {
    errno = res;
    WSEDEBUG("WSME| socket err: %s \n", strerror(errno));
    return -1;
  }
  WSEDEBUG("WSME| socket() returning: %d \n", fd);
  return fd;
}

int bind(int fd, const struct sockaddr* addr, socklen_t len) {
  WSEDEBUG("WSME| bind[%d]: calling bind on sa_data=[", __LINE__);
  for (int i = 0; i < len; ++i) {
    WSEDEBUG("'%d', ", (short)addr->sa_data[i]);
  }
  WSEDEBUG("]\n");

  we_address_buffer we_addr_buf;
  we_address we_addr = sockaddr_to_we_address(&we_addr_buf, addr);
  uint16_t port = sockaddr_port(addr);

  int res = __import_wasip1_sock_bind(fd, &we_addr, port);
  if (res != 0) {
    errno = res;
    return -1;
  }
  return res;
}

int connect(int fd, const struct sockaddr* addr, socklen_t len) {
  WSEDEBUG("WSME| connect[%d]: calling connect on %d addr=%p len=%d\n",
           __LINE__, fd, addr, len);

  we_address_buffer we_addr_buf;
  we_address we_addr = sockaddr_to_we_address(&we_addr_buf, addr);
  uint16_t port = sockaddr_port(addr);

  int res = __import_wasip1_sock_connect(fd, &we_addr, port);
  if (res != 0) {
    WSEDEBUG("WSME| connect(): %d\n", res);
    errno = res;
    return -1;
  }
  return 0;
}

int accept4(int socket, struct sockaddr* restrict addr,
            socklen_t* restrict addrlen, int flags) {
  WSEDEBUG("WSME| accept4[%d]: fd=%d flags=%d\n", __LINE__, socket, flags);

  uint32_t ret = -1;

  // TODO: non blocking flag?
  int32_t err = __import_wasip1_sock_accept(socket, 0, &ret);
  if (err != 0) {
    errno = err;
    return -1;
  }

  memset(addr, 0, *addrlen);
  addr->sa_family = AF_UNSPEC;
  *addrlen = sizeof(struct sockaddr);

  return ret;
}

int accept(int socket, struct sockaddr* restrict addr,
           socklen_t* restrict addrlen) {
  return accept4(socket, addr, addrlen, 0);
}

ssize_t recv(int socket, void* restrict buffer, size_t length, int flags) {
  WSEDEBUG("WSME| recv[%d]: fd=%d buflen=%zu\n", __LINE__, socket, length);

  iovec_read_t iov = {.buf = buffer, .size = length};
  uint32_t ro_datalen;
  uint32_t ro_flags;  // nothing to do with this?

  int err =
      __import_wasip1_sock_recv(socket, &iov, 1, flags, &ro_datalen, &ro_flags);
  if (err != 0) {
    errno = err;
    return -1;
  }

  return ro_datalen;
}

ssize_t send(int socket, const void* buffer, size_t length, int flags) {
  WSEDEBUG("WSME| send[%d]: fd=%d buflen=%zu\n", __LINE__, socket, length);

  iovec_write_t io = {.buf = (uint8_t*)(buffer), .size = length};
  uint32_t send_len;
  int32_t err = __import_wasip1_sock_send(socket, io, 1, 0, &send_len);
  if (err != 0) {
    errno = err;
    return -1;
  }
  return send_len;
}

int shutdown(int socket, int how) {
  WSEDEBUG("WSME| shutdown[%d]: fd=%d\n", __LINE__, socket);

  int32_t err = __import_wasip1_sock_shutdown(socket, (uint8_t)(how));
  if (err != 0) {
    errno = err;
    return -1;
  }
  return err;
}

int listen(int fd, int backlog) {
  backlog = MIN(backlog, SOMAXCONN);
  return __import_wasip1_sock_listen(fd, backlog);
}

int getsockopt(int sockfd, int level, int optname, void* restrict optval,
               socklen_t* restrict optlen) {
  WSEDEBUG("WSME| getsockopt[%d]: fd=%d level=%d name=%d option_len=%d\n",
           __LINE__, sockfd, level, optname, *optlen);

  we_socket_option_level wlevel = utw_sock_option_level(level);
  we_socket_option wname = utw_sock_option(level, optname);

  if (*optlen < 4) {
    // wasmedge v2 only supports int32 options.
    return EINVAL;
  }

  int32_t* val = (int32_t*)(optval);
  int32_t len = 4;
  *optlen = len;

  int res = __import_wasip1_sock_getsockopt(sockfd, wlevel, wname, val, len);

  if (res != 0) {
    errno = res;
    return -1;
  }

  // When getsockopt attemps to query socket type, we need to convert back the
  // wasi socket type to the unix socket type.
  if (optname == SO_TYPE) {
    *val = wtu_sock_type(*val);
  }

  return 0;
}

int setsockopt(int fd, int level, int optname, const void* optval,
               socklen_t optlen) {
  WSEDEBUG("WSME| setsockopt[%d]: fd=%d level=%d optname=%d optlen=%d\n",
           __LINE__, fd, level, optname, optlen);

  we_socket_option wname = utw_sock_option(level, optname);
  we_socket_option_level wlevel = utw_sock_option_level(level);

  WSEDEBUG("WSME| setsockopt[%d]: fd=%d wasm_level=%d wasm_optname=%d\n",
           __LINE__, fd, wlevel, wname);

  int res = __import_wasip1_sock_setsockopt(fd, wlevel, wname, (int32_t*)optval,
                                            (uint32_t)optlen);
  if (res != 0) {
    if ((level == IPPROTO_TCP && optname == TCP_NODELAY) || (level == SOL_SOCKET && optname == SO_REUSEADDR)) {
      // we don't support those yet, and urllib3 crashes if this option fails.
      return 0;
    }
    WSEDEBUG(
        "WSME| setsockopt[%d]: ERROR %d (fd=%d level=%d optname=%d "
        "optlen=%d)\n",
        __LINE__, res, fd, level, optname, optlen);
    errno = res;
    return -1;
  }
  return 0;
}

struct servent* getservbyname(const char* name, const char* prots) {
  WSEDEBUG("WSME| getservbyname[%d]: name=%s\n", __LINE__, name);
  return NULL;
}

int getnameinfo(const struct sockaddr* restrict addr, socklen_t addrlen,
                char* restrict host, socklen_t hostlen, char* restrict serv,
                socklen_t servlen, int flags) {
  // TODO
  WSEDEBUG("WSME| getnameinfo[%d]: \n", __LINE__);
  abort();
  return EAI_FAIL;
}

struct hostent* gethostbyname(const char* name) {
  // TODO implement with getaddrinfo
  WSEDEBUG("WSME| gethostbyname[%d]: \n", __LINE__);
  return NULL;
}

struct hostent* gethostbyaddr(const void* addr, socklen_t len, int type) {
  // TODO not sure how to write this
  return NULL;
}

int gethostname(char* name, size_t len) {
  if (len == 0) {
    return 0;
  }

  const char* myname = "localhost";
  size_t n = MIN(len, sizeof(myname)) - 1;
  memcpy(name, myname, n);
  name[n] = 0;

  return n;
}

int getpeername(int sockfd, struct sockaddr* restrict addr,
                socklen_t* restrict addrlen) {
  WSEDEBUG("WASME| getpeername[%d]: fd:'%d' addr:'%p' size:'%d' \n", __LINE__,
           sockfd, addr, *addrlen);

  uint32_t port;
  we_address_buffer buf;
  we_address rsa = {.buf = &buf, .len = sizeof(buf)};
  int32_t res = __import_wasip1_sock_getpeeraddr(sockfd, &rsa, &port);
  if (res != 0) {
    WSEDEBUG("WASME| bad getpeeraddr %d => %d\n", sockfd, res);
    errno = res;
    return -1;
  }

  // Intermediate output address to make truncation easier, for the cost of an
  // extra copy. Explicitely 0 initialized for inet6 that doesn't fill all the
  // fields.
  struct sockaddr_storage storage = {0};
  socklen_t size;

  switch (buf.family) {
    case WE_INET_FAMILY: {
      size = sizeof(struct sockaddr_in);
      struct sockaddr_in* sin = (struct sockaddr_in*)(&storage);
      sin->sin_family = AF_INET;
      sin->sin_port = port;
      memcpy(&sin->sin_addr, &buf.inet_addr, 4);
    } break;
    case WE_INET6_FAMILY: {
      size = sizeof(struct sockaddr_in6);
      struct sockaddr_in6* sin6 = (struct sockaddr_in6*)(&storage);
      sin6->sin6_family = AF_INET6;
      sin6->sin6_port = port;
      memcpy(&sin6->sin6_addr, &buf.inet6_addr, 16);
    } break;
    case WE_UNIX_FAMILY: {
      struct sockaddr_un* sun = (struct sockaddr_un*)(&storage);
      size = sizeof(struct sockaddr_un);
      sun->sun_family = AF_UNIX;
      memcpy(&sun->sun_path, &buf.unix_addr, 126);
    } break;
    default:
      printf("!! unsupported family: %d\n", buf.family);
      abort();
  }

  memcpy(addr, &storage, MIN(*addrlen, size));
  *addrlen = size;

  return 0;
}

int getsockname(int fd, struct sockaddr* restrict addr,
                socklen_t* restrict addrlen) {
  // addrlen contains the size of the struct pointed at by addr
  // it needs to be returned with the actual size of the address

  if (addr == NULL || addrlen == NULL) {
    errno = EFAULT;
    return -1;
  }

  uint32_t port;
  we_address_buffer raw;
  we_address waddr = {
      .buf = &raw,
      .len = sizeof(raw),
  };

  int32_t err = __import_wasip1_sock_getlocaladdr(fd, &waddr, &port);
  if (0 != err) {
    errno = err;
    return -1;
  }
  socklen_t max = *addrlen;

  switch (raw.family) {
    case WE_INET_FAMILY: {
      *addrlen = sizeof(struct sockaddr_in);
      if (max < *addrlen) {
        // TODO: should just truncate the address instead of panic
        printf("!!! getsockname buffer too small (%d/%d)\n", max, *addrlen);
        abort();
      }
      struct sockaddr_in* out = (struct sockaddr_in*)(addr);
      out->sin_family = AF_INET;
      out->sin_port = port;
      out->sin_addr.s_addr = *(in_addr_t*)(&raw.inet_addr);  // copy 4 bytes
    } break;
    case WE_INET6_FAMILY: {
      *addrlen = sizeof(struct sockaddr_in6);
      if (max < *addrlen) {
        // TODO: should just truncate the address instead of panic
        printf("!!! getsockname buffer too small (%d/%d)\n", max, *addrlen);
        abort();
      }
      struct sockaddr_in6* out = (struct sockaddr_in6*)(addr);
      out->sin6_family = AF_INET6;
      out->sin6_port = port;
      out->sin6_addr = *(struct in6_addr*)(&raw.inet6_addr);  // copy 16 bytes
    } break;
    case WE_UNIX_FAMILY: {
      *addrlen = sizeof(struct sockaddr_un);
      if (max < *addrlen) {
        // TODO: should just truncate the address instead of panic
        printf("!!! getsockname buffer too small (%d/%d)\n", max, *addrlen);
        abort();
      }
      struct sockaddr_un* out = (struct sockaddr_un*)addr;
      out->sun_family = AF_UNIX;
      memcpy(&out->sun_path, &raw.unix_addr, 108);
    } break;
    default:
      printf("!!! unsupported family in getsockname: %d\n", raw.family);
      abort();
  }

  return 0;
}

static struct addrinfo* we_sock_address_info_to_addrinfo(
    we_sock_address_info* infos, uint32_t n) {
  WSEDEBUG("WSME| convert_wasi_addrinfo_to_addrinfo[%d]: \n", __LINE__);
  // TODO: we could potentially save memory by doing a pre-pass on wasi_addrinfo
  // to detect how many addresses are needed. Also if sizes work out we could
  // reuse the allocation of wasi_addrinfo instead.
  struct addrinfo* out = (struct addrinfo*)calloc(
      n, sizeof(struct addrinfo) + sizeof(struct sockaddr_storage));
  struct sockaddr_storage* addresses = (struct sockaddr_storage*)(&out[n]);

  we_sock_address_info* info = infos;
  for (size_t i = 0; i < n; i++) {
    out[i] = (struct addrinfo){
        .ai_flags = (int)info->flags,
        .ai_family = wtu_protocol_family(info->family),
        .ai_socktype = wtu_sock_type(info->socket_type),
        .ai_protocol = wtu_protocol(info->protocol),
        .ai_addrlen = 0,
        .ai_addr = NULL,

        // not supported by wasi-go yet
        .ai_canonname = NULL,
        .ai_canonnamelen = 0,

        .ai_next = NULL,
    };

    if (info->address != NULL) {
      we_sock_addr* waddr = info->address;
      switch (waddr->family) {
        case WE_INET_FAMILY: {
          struct sockaddr_in* addr = (struct sockaddr_in*)(&addresses[i]);
          addr->sin_family = AF_INET;
          addr->sin_port = *((uint16_t*)(&waddr->data[0]));
          memcpy(&addr->sin_addr, waddr->data + 2, sizeof(struct in_addr));

          out[i].ai_addr = (struct sockaddr*)(&addresses[i]);
          out[i].ai_addrlen = sizeof(struct sockaddr_in);
          out[i].ai_family = AF_INET;
        } break;
        case WE_INET6_FAMILY: {
          struct sockaddr_in6* addr = (struct sockaddr_in6*)(&addresses[i]);
          addr->sin6_family = AF_INET6;
          addr->sin6_port = *((uint16_t*)(&waddr->data[0]));
          memcpy(&addr->sin6_addr, waddr->data + 2, sizeof(struct in6_addr));

          out[i].ai_addr = (struct sockaddr*)(&addresses[i]);
          out[i].ai_addrlen = sizeof(struct sockaddr_in6);
          out[i].ai_family = AF_INET6;
        } break;
        default:
          printf("!!! unknown family: %d\n", waddr->family);
          abort();
      }
    }
    if (i > 0) {
      out[i - 1].ai_next = &out[i];
    }
    info = info->next;
  }
  return out;
}

int getaddrinfo(const char* restrict host, const char* restrict serv,
                const struct addrinfo* restrict hint,
                struct addrinfo** restrict res) {
  WSEDEBUG("WSME| getaddrinfo[%d]: host:'%s' serv:'%s' hint:'%p' \n", __LINE__,
           host, serv, hint);

  we_sock_address_info hints = {
      .flags = 0,  // TODO: hint->ai_flags,
      .family = utw_sock_domain(hint->ai_family),
      .socket_type = utw_sock_type(hint->ai_socktype),
      .protocol = utw_sock_proto(hint->ai_protocol),
      .address_length = 0,
      .address = 0,
      .canonical_name = 0,
      .canonical_name_length = 0,
      .next = NULL,
  };

#define MAX_RESULTS 16

  we_sock_addr_result* results =
      (we_sock_addr_result*)calloc(MAX_RESULTS, sizeof(we_sock_addr_result));

  for (size_t i = 0; i < MAX_RESULTS; i++) {
    results[i].info.address_length = sizeof(we_sock_addr);
    results[i].info.address = &results[i].addr;
    results[i].info.canonical_name = results[i].canonname;
    results[i].info.canonical_name_length = sizeof(results[0].canonname);
    results[i].info.family = hints.family;
    results[i].info.socket_type = hints.socket_type;
    results[i].info.protocol = hints.protocol;

    results[i].addr.data_len = sizeof(results[0].data);
    results[i].addr.data = results[i].data;

    if (i > 0) {
      results[i - 1].info.next = &results[i].info;
    }
  }

  uint32_t res_len = 0;
  int rc = __import_wasip1_sock_getaddrinfo(
      (uint8_t*)host, strlen(host), (uint8_t*)serv, strlen(serv), &hints,
      ((we_sock_address_info**)(&results)), MAX_RESULTS, &res_len);

  if (0 != rc) {
    errno = rc;
    free((void*)results);
    return -1;
  }

  *res =
      we_sock_address_info_to_addrinfo((we_sock_address_info*)results, res_len);
  free(results);
  return 0;
}

void freeaddrinfo(struct addrinfo* p) {
  WSEDEBUG("WSME| freeaddrinfo[%d]: \n", __LINE__);
  free(p);
}

#undef h_errno
int h_errno;
// TODO: actually fill h_errno
int* __h_errno_location(void) {
  // TODO: thread safety
  return &h_errno;
}
