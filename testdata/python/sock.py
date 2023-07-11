import socket

socka, sockb = socket.socketpair()

print('socka.getsockname() =', socka.getsockname())
print('sockb.getsockname() =', sockb.getsockname())
