# timecraft host-TLS implementation

import logging
from collections import namedtuple
from enum import _simple_enum
from enum import Enum as _Enum, IntEnum as _IntEnum, IntFlag as _IntFlag
from socket import socket, SOCK_STREAM, create_connection
from socket import SOL_SOCKET, SO_TYPE, _GLOBAL_DEFAULT_TIMEOUT
import socket as _socket
import errno

log = logging.getLogger('ssl')


class SSLError(OSError):
    """An error occurred in the SSL implementation."""


class SSLWantReadError(SSLError):
    """Non-blocking SSL socket needs to read more data
    before the requested operation can be completed."""


class SSLSyscallError(SSLError):
    """System error when attempting SSL operation."""


# Copied from my machine
OPENSSL_VERSION="OpenSSL 3.0.5 5 Jul 2022"
OPENSSL_VERSION_INFO=(3, 0, 0, 5, 0)
OPENSSL_VERSION_NUMBER=805306448

# Copied from _ssl.c
CERT_NONE=0
CERT_OPTIONAL=1
CERT_REQUIRED=2

PROTOCOL_TLS=2
PROTOCOL_TLS_CLIENT=0x10

# https://wiki.openssl.org/index.php/List_of_SSL_OP_Flags
OP_NO_COMPRESSION=1<<17
OP_NO_TICKET=1<<14
OP_NO_SSLv2=0
OP_NO_SSLv3=1<<25

# Copied from ssl.py
HAS_NEVER_CHECK_COMMON_NAME=True


@_simple_enum(_IntEnum)
class TLSVersion:
    MINIMUM_SUPPORTED = -2
    # https://github.com/openssl/openssl/blob/master/include/openssl/prov_ssl.h#LL23C42-L23C48
    SSLv3 = 0x0300
    TLSv1 = 0x0301
    TLSv1_1 = 0x0302
    TLSv1_2 = 0x0303
    TLSv1_3 = 0x0304
    MAXIMUM_SUPPORTED = -1


class _ASN1Object(namedtuple("_ASN1Object", "nid shortname longname oid")):
    """ASN.1 object identifier lookup
    """
    # __slots__ = ()

    def __new__(cls, oid):
        if oid == '1.3.6.1.5.5.7.3.1':
            return super().__new__(cls, nid=129, shortname='serverAuth', longname='TLS Web Server Authentication', oid=oid)
        elif oid == '1.3.6.1.5.5.7.3.2':
            return super().__new__(cls, nid=130, shortname='clientAuth', longname='TLS Web Client Authentication', oid=oid)
        else:
            raise NotImplementedError(f"ASN1Object does not support oid={oid}")
    
    @classmethod
    def fromnid(cls, nid):
        """Create _ASN1Object from OpenSSL numeric ID
        """
        raise NotImplementedError("can't create new ASN1Object from nid")

    @classmethod
    def fromname(cls, name):
        """Create _ASN1Object from short name, long name or OID
        """
        raise NotImplementedError("can't create new ASN1Object from name")


class Purpose(_ASN1Object, _Enum):
    """SSLContext purpose flags with X509v3 Extended Key Usage objects
    """
    SERVER_AUTH = '1.3.6.1.5.5.7.3.1'
    CLIENT_AUTH = '1.3.6.1.5.5.7.3.2'
    
    
class SSLContext():
    # TODO

    def __init__(self, protocol=None, *args, **kwargs):
        self._options = 0


    @property
    def options(self):
        return self._options

    @options.setter
    def options(self, value):
        self._options |= value

    def load_verify_locations(self, cafile=None, capath=None, cadata=None):
        log.warning("load_verify_locations does nothing")

    def set_alpn_protocols(self, alpn_protocols):
        log.warning("set_alpn_protocols does nothing")

    def wrap_socket(self, sock, server_side=False,
                    do_handshake_on_connect=True,
                    suppress_ragged_eofs=True,
                    server_hostname=None, session=None):
        return SSLSocket._create(
            sock=sock,
            server_side=server_side,
            do_handshake_on_connect=do_handshake_on_connect,
            suppress_ragged_eofs=suppress_ragged_eofs,
            server_hostname=server_hostname,
            context=self,
            session=session
        )
    
    def _encode_hostname(self, hostname):
        if hostname is None:
            return None
        elif isinstance(hostname, str):
            return hostname.encode('idna').decode('ascii')
        else:
            return hostname.decode('ascii')

    @property
    def post_handshake_auth(self):
        return None


class SSLSocket(socket):
    def __init__(self, *args, **kwargs):
        raise TypeError(
            f"{self.__class__.__name__} does not have a public "
            f"constructor. Instances are returned by "
            f"SSLContext.wrap_socket()."
        )

    @classmethod
    def _create(cls, sock, server_side=False, do_handshake_on_connect=True,
                suppress_ragged_eofs=True, server_hostname=None,
                context=None, session=None):
        r = sock.getsockopt(SOL_SOCKET, SO_TYPE)
        if r != SOCK_STREAM:
            raise NotImplementedError(f"only stream sockets are supported, not {r}")
        if server_side:
            raise NotImplementedError(f"only client-side ssl is supported")
        if context.check_hostname and not server_hostname:
            raise ValueError("check_hostname requires server_hostname")

        kwargs = dict(
            family=sock.family, type=sock.type, proto=sock.proto,
            fileno=sock.fileno()
        )
        self = cls.__new__(cls, **kwargs)
        super(SSLSocket, self).__init__(**kwargs)
        self.settimeout(sock.gettimeout())
        sock.detach()

        self.server_hostname = context._encode_hostname(server_hostname)

        self.setsockopt(0x74696d65, 1, self.server_hostname.encode("ascii"))

        return self

    def dup(self):
        raise NotImplementedError("Can't dup() %s instances" %
                                  self.__class__.__name__)

    def read(self, len=1024, buffer=None):
        """Read up to LEN bytes and return them.
        Return zero-length string on EOF."""
        raise NotImplementedError(f"tried to call not implemented read({len}, {buffer})")

    def write(self, data):
        """Write DATA to the underlying SSL channel.  Returns
        number of bytes of DATA actually transmitted."""
        raise NotImplementedError(f"tried to call not implemented write({data})")

    def getpeercert(self, binary_form=False):
        raise NotImplementedError(f"tried to call not implemented getpeercert")

    def selected_npn_protocol(self):
        raise NotImplementedError(f"tried to call not implemented selected_npn_protocol")

    def selected_alpn_protocol(self):
        raise NotImplementedError(f"tried to call not implemented selected_alpn_protocol")

    def cipher(self):
        raise NotImplementedError(f"tried to call not implemented cipher")

    def shared_ciphers(self):
        raise NotImplementedError(f"tried to call not implemented shared_ciphers")

    def compression(self):
        raise NotImplementedError(f"tried to call not implemented compression")

    def send(self, data, flags=0):
        raise NotImplementedError(f"tried to call not implemented send({data}, {flags})")

    def sendto(self, data, flags_or_addr, addr=None):
        raise NotImplementedError(f"tried to call not implemented sendto({data}, {flags_or_addr}, {addr})")

    def sendmsg(self, *args, **kwargs):
        # Ensure programs don't send data unencrypted if they try to
        # use this method.
        raise NotImplementedError("sendmsg not allowed on instances of %s" %
                                  self.__class__)

    def sendall(self, data, flags=0):
        return super().sendall(data, flags)

    def sendfile(self, file, offset=0, count=None):
        """Send a file, possibly by using os.sendfile() if this is a
        clear-text socket.  Return the total number of bytes sent.
        """
        raise NotImplementedError(f"tried to call not implemented sendfile({file}, {offset}, {count})")

    def recv(self, buflen=1024, flags=0):
        raise NotImplementedError(f"tried to call not implemented recv({buflen}, {flags})")

    def recv_into(self, buffer, nbytes=None, flags=0):
        if nbytes:
            return super().recv_into(buffer, nbytes, flags)
        else:
            return super().recv_into(buffer, flags)

    def recvfrom(self, buflen=1024, flags=0):
        raise NotImplementedError(f"tried to call not implemented recvfrom({buflen}, {flags})")

    def recvfrom_into(self, buffer, nbytes=None, flags=0):
        raise NotImplementedError(f"tried to call not implemented recvfrom_into({buffer}, {buflen}, {flags})")

    def recvmsg(self, *args, **kwargs):
        raise NotImplementedError("recvmsg not allowed on instances of %s" %
                                  self.__class__)

    def recvmsg_into(self, *args, **kwargs):
        raise NotImplementedError("recvmsg_into not allowed on instances of "
                                  "%s" % self.__class__)

    def pending(self):
        raise NotImplementedError(f"tried to call not implemented pending")

    def shutdown(self, how):
        raise NotImplementedError(f"tried to call not implemented shutdown")

    def unwrap(self):
        raise NotImplementedError(f"tried to call not implemented unwrap")

    def verify_client_post_handshake(self):
        raise NotImplementedError(f"tried to call not implemented verify_client_post_handshake")

    def _real_close(self):
        return super()._real_close()

    def do_handshake(self, block=False):
        raise NotImplementedError("host should be in charge of performing handshake")

    def connect(self, addr):
        """Connects to remote ADDR, and then wraps the connection in
        an SSL channel."""
        raise NotImplementedError(f"tried to call not implemented connect({addr})")

    def connect_ex(self, addr):
        """Connects to remote ADDR, and then wraps the connection in
        an SSL channel."""
        raise NotImplementedError(f"tried to call not implemented connect_ex({addr})")

    def accept(self):
        """Accepts a new connection from a remote client, and returns
        a tuple containing that new connection wrapped with a server-side
        SSL channel, and the address of the remote client."""

        return NotImplementedError("htls does not support accept()")

    def get_channel_binding(self, cb_type="tls-unique"):
        raise NotImplementedError(f"tried to call not implemented get_channel_binding({cb_type})")

    def version(self):
        raise NotImplementedError(f"tried to call not implemented version()")


def create_default_context(purpose=Purpose.SERVER_AUTH, *, cafile=None,
                           capath=None, cadata=None):
    """Create a SSLContext object with default settings.

    NOTE: The protocol and settings may change anytime without prior
          deprecation. The values represent a fair balance between maximum
          compatibility and security.
    """
    if not isinstance(purpose, _ASN1Object):
        raise TypeError(purpose)

    # SSLContext sets OP_NO_SSLv2, OP_NO_SSLv3, OP_NO_COMPRESSION,
    # OP_CIPHER_SERVER_PREFERENCE, OP_SINGLE_DH_USE and OP_SINGLE_ECDH_USE
    # by default.
    if purpose == Purpose.SERVER_AUTH:
        # verify certs and host name in client mode
        context = SSLContext(PROTOCOL_TLS_CLIENT)
        context.verify_mode = CERT_REQUIRED
        context.check_hostname = True
    elif purpose == Purpose.CLIENT_AUTH:
        context = SSLContext(PROTOCOL_TLS_SERVER)
    else:
        raise ValueError(purpose)

    if cafile or capath or cadata:
        log.warning("cafile, capath, cadata arguments do nothing")
    elif context.verify_mode != CERT_NONE:
        # no explicit cafile, capath or cadata but the verify mode is
        # CERT_OPTIONAL or CERT_REQUIRED. Let's try to load default system
        # root CA certificates for the given purpose. This may fail silently.
        log.warning("certificates are not explicitely loaded")
    # OpenSSL 1.1.1 keylog file
    if hasattr(context, 'keylog_filename'):
        log.warning("keylog_filename / SSLKEYLOGFILE do nothing")
    return context

        
# Used by http.client if no context is explicitly passed.
_create_default_https_context = create_default_context
