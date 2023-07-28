#pragma once

#include <sys/types.h>

#include <wasi/api.h>

#include <__header_sys_socket.h>


#define SOMAXCONN 128


#define SO_REUSEADDR 0
#undef SO_TYPE
#define SO_TYPE 1
#define SO_ERROR 2
#define SO_DONTROUTE 3
#define SO_BROADCAST 4
#define SO_SNDBUF 5
#define SO_RCVBUF 6
#define SO_KEEPALIVE 7
#define SO_OOBINLINE 8
#define SO_LINGER 9
#define SO_RCVLOWAT 10
#define SO_RCVTIMEO 11
#define SO_SNDTIMEO 12
#define SO_ACCEPTCONN 13
#define SO_BINDTODEVICE 14


#ifdef __cplusplus
extern "C" {
#endif

int bind(int sockfd, const struct sockaddr* addr, socklen_t addrlen);
int socket(int, int, int);

int listen(int sockfd, int backlog);

int accept(int socket, struct sockaddr* restrict addr,
           socklen_t* restrict addrlen);
int accept4(int socket, struct sockaddr* restrict addr,
            socklen_t* restrict addrlen, int flags);
int connect(int sockfd, const struct sockaddr* addr, socklen_t addrlen);
int getsockopt(int socket, int level, int option_name,
               void* restrict option_value, socklen_t* restrict option_len);
int setsockopt(int sockfd, int level, int optname, const void* optval,
               socklen_t optlen);
int getpeername(int sockfd, struct sockaddr *restrict addr,
		socklen_t *restrict addrlen);
int getsockname(int sockfd, struct sockaddr* restrict addr,
                socklen_t* restrict addrlen);
ssize_t recv(int socket, void* restrict buffer, size_t length, int flags);
ssize_t send(int socket, const void* buffer, size_t length, int flags);
int shutdown(int socket, int how);

#ifdef __cplusplus
}
#endif
