import ctypes
import pathlib


try:
    _parent_dir = pathlib.Path(__file__).parent.resolve()
    _lib = ctypes.CDLL(_parent_dir / "libclntlib.so")
except OSError as e:
    print(f"Error loading shared library: {e}")
    print("Please ensure the C++ library is compiled and the path is correct.")
    raise RuntimeError("SigmaOS library not loaded.")


class Qid(ctypes.Structure):
    _fields_ = [
        ("path", ctypes.c_uint64),
        ("version", ctypes.c_uint32),
        ("type", ctypes.c_uint8),
    ]


class Stat(ctypes.Structure):
    _fields_ = [
        ("type", ctypes.c_uint16),
        ("dev", ctypes.c_uint32),
        ("qid", Qid),
        ("mode", ctypes.c_uint32),
        ("atime", ctypes.c_uint32),
        ("mtime", ctypes.c_uint32),
        ("length", ctypes.c_uint64),
        ("name", ctypes.c_char_p),
        ("uid", ctypes.c_char_p),
        ("gid", ctypes.c_char_p),
        ("muid", ctypes.c_char_p),
    ]


def _define_prototypes():
    """Set the argument and return types for all C functions."""

    # void init_socket()
    _lib.init_socket.restype = None

    # void close_fd_stub(int fd)
    _lib.close_fd_stub.argtypes = [ctypes.c_int]
    _lib.close_fd_stub.restype = None

    # CTstatProto* stat_stub(char* pn)
    _lib.stat_stub.argtypes = [ctypes.c_char_p]
    _lib.stat_stub.restype = ctypes.POINTER(Stat)

    # int create_stub(char* pn, uint32_t perm, uint32_t mode)
    _lib.create_stub.argtypes = [ctypes.c_char_p, ctypes.c_uint32, ctypes.c_uint32]
    _lib.create_stub.restype = ctypes.c_int

    # int open_stub(char* pn, uint32_t mode, bool wait)
    _lib.open_stub.argtypes = [ctypes.c_char_p, ctypes.c_uint32, ctypes.c_bool]
    _lib.open_stub.restype = ctypes.c_int

    # void rename_stub(char* src, char* dst)
    _lib.rename_stub.argtypes = [ctypes.c_char_p, ctypes.c_char_p]
    _lib.rename_stub.restype = None

    # void remove_stub(char* pn)
    _lib.remove_stub.argtypes = [ctypes.c_char_p]
    _lib.remove_stub.restype = None

    # char* get_file_stub(char* pn)
    _lib.get_file_stub.argtypes = [ctypes.c_char_p]
    _lib.get_file_stub.restype = ctypes.c_char_p

    # uint32_t put_file_stub(...)
    _lib.put_file_stub.argtypes = [ctypes.c_char_p, ctypes.c_uint32, ctypes.c_uint32, ctypes.c_char_p, ctypes.c_uint64, ctypes.c_uint64]
    _lib.put_file_stub.restype = ctypes.c_uint32

    # uint32_t read_stub(int fd, char* b)
    _lib.read_stub.argtypes = [ctypes.c_int, ctypes.c_char_p]
    _lib.read_stub.restype = ctypes.c_uint32

    # uint32_t pread_stub(int fd, char* b, uint64_t o)
    _lib.pread_stub.argtypes = [ctypes.c_int, ctypes.c_char_p, ctypes.c_uint64]
    _lib.pread_stub.restype = ctypes.c_uint32

    # uint32_t write_stub(int fd, char* b)
    _lib.write_stub.argtypes = [ctypes.c_int, ctypes.c_char_p]
    _lib.write_stub.restype = ctypes.c_uint32

    # void seek_stub(int fd, uint64_t o)
    _lib.seek_stub.argtypes = [ctypes.c_int, ctypes.c_uint64]
    _lib.seek_stub.restype = None

    # uint64_t clnt_id_stub()
    _lib.clnt_id_stub.restype = ctypes.c_uint64

    # void started()
    _lib.started.restype = None

    # void exited(uint32_t status, char* msg)
    _lib.exited.argtypes = [ctypes.c_uint32, ctypes.c_char_p]
    _lib.exited.restype = None

    # void wait_evict()
    _lib.wait_evict.restype = None

    # Memory freeing functions
    _lib.free_stat.argtypes = [ctypes.POINTER(Stat)]
    _lib.free_stat.restype = None
    _lib.free_string.argtypes = [ctypes.c_char_p]
    _lib.free_string.restype = None

_define_prototypes()
_lib.init_socket()

# === File and Directory Operations ===

def open(path: str, mode: int, wait: bool = False) -> int:
    """Opens a file and returns its file descriptor."""
    path_bytes = path.encode("utf-8")
    fd = _lib.open_stub(path_bytes, mode, wait)
    if fd == -1:
        raise IOError(f"Failed to open file: {path}")
    return fd

def close(fd: int):
    """Closes an open file descriptor."""
    _lib.close_fd_stub(fd)

def create(path: str, perm: int, mode: int) -> int:
    """Creates a new file and returns its file descriptor."""
    path_bytes = path.encode("utf-8")
    fd = _lib.create_stub(path_bytes, perm, mode)
    if fd == -1:
        raise IOError(f"Failed to create file: {path}")
    return fd

def stat(path: str) -> dict | None:
    """Retrieves file status information."""
    path_bytes = path.encode("utf-8")
    stat_ptr = _lib.stat_stub(path_bytes)
    if not stat_ptr:
        return None
    try:
        s = stat_ptr.contents
        return {
            "type": s.type, "dev": s.dev,
            "qid": {"path": s.qid.path, "version": s.qid.version, "type": s.qid.type},
            "mode": s.mode, "atime": s.atime, "mtime": s.mtime, "length": s.length,
            "name": s.name.decode("utf-8"), "uid": s.uid.decode("utf-8"),
            "gid": s.gid.decode("utf-8"), "muid": s.muid.decode("utf-8"),
        }
    finally:
        _lib.free_stat(stat_ptr)

def rename(src_path: str, dst_path: str):
    """Renames or moves a file."""
    src_bytes = src_path.encode("utf-8")
    dst_bytes = dst_path.encode("utf-8")
    _lib.rename_stub(src_bytes, dst_bytes)

def remove(path: str):
    """Removes a file."""
    path_bytes = path.encode("utf-8")
    _lib.remove_stub(path_bytes)

# === File Content I/O ===

def get_file(path: str) -> str | None:
    """Reads the entire content of a file."""
    path_bytes = path.encode("utf-8")
    content_ptr = _lib.get_file_stub(path_bytes)
    if not content_ptr:
        return None
    try:
        return ctypes.string_at(content_ptr).decode("utf-8")
    finally:
        _lib.free_string(content_ptr)

def put_file(path: str, perm: int, mode: int, data: str | bytes, offset: int = 0, lease_id: int = 0) -> int:
    """Writes data to a file, creating it if necessary."""
    path_bytes = path.encode("utf-8")
    data_bytes = data.encode("utf-8") if isinstance(data, str) else data
    bytes_written = _lib.put_file_stub(path_bytes, perm, mode, data_bytes, offset, lease_id)
    if bytes_written == -1:
        raise IOError(f"Failed to write to file: {path}")
    return bytes_written

def read(fd: int, buffer_size: int) -> bytes:
    """Reads up to `buffer_size` bytes from a file descriptor."""
    buffer = ctypes.create_string_buffer(buffer_size)
    bytes_read = _lib.read_stub(fd, buffer)
    if bytes_read == -1:
        raise IOError(f"Failed to read from file descriptor: {fd}")
    return buffer.raw[:bytes_read]

def pread(fd: int, buffer_size: int, offset: int) -> bytes:
    """Reads up to `buffer_size` bytes from a file descriptor at a specific offset."""
    buffer = ctypes.create_string_buffer(buffer_size)
    bytes_read = _lib.pread_stub(fd, buffer, offset)
    if bytes_read == -1:
        raise IOError(f"Failed to read from file descriptor {fd} at offset {offset}")
    return buffer.raw[:bytes_read]

def write(fd: int, data: str | bytes) -> int:
    """Writes data to a file descriptor."""
    data_bytes = data.encode("utf-8") if isinstance(data, str) else data
    bytes_written = _lib.write_stub(fd, data_bytes)
    if bytes_written == -1:
        raise IOError(f"Failed to write to file descriptor: {fd}")
    return bytes_written

def seek(fd: int, offset: int):
    """Changes the read/write offset of a file descriptor."""
    _lib.seek_stub(fd, offset)

# === Client and Process Management ===

def get_client_id() -> int:
    """Gets the client ID."""
    client_id = _lib.clnt_id_stub()
    if client_id == -1:
        raise RuntimeError("Failed to get client ID.")
    return client_id

def started():
    """Notifies the system that the process has started."""
    _lib.started()

def exited(status: int, message: str):
    """Notifies the system that the process has exited and cleans up resources."""
    msg_bytes = message.encode("utf-8")
    _lib.exited(status, msg_bytes)

def wait_for_eviction():
    """Blocks until an eviction event occurs."""
    _lib.wait_evict()