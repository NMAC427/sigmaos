"""Fork support for SigmaOS Python procs.

The proc runs expensive imports/initialization and then calls fork_point().
At fork_point, the process becomes a Zygote: it connects to the procd-local
fork supervisor socket and waits for fork requests.

The supervisor sends child args + env updates over the socket. The zygote forks,
creates a fresh PID namespace for the child, applies the env updates, and then
returns the child args to user code.
"""

from __future__ import annotations

import ctypes
import ctypes.util
import gc
import json
import os
import socket
import struct
from typing import Any


SIGMA_FORK_SOCK = "SIGMA_FORK_SOCK"
SIGMA_FORK_ZYGOTE_KEY = "SIGMA_FORK_ZYGOTE_KEY"


# Reduce memory overhead after forking.
gc.disable()


CLONE_NEWPID = 0x20000000

_libc = ctypes.CDLL("libc.so.6", use_errno=True)
_unshare = _libc.unshare
_unshare.argtypes = [ctypes.c_int]
_unshare.restype = ctypes.c_int


def _read_exact(sock: socket.socket, n: int) -> bytes:
    buf = bytearray()
    while len(buf) < n:
        chunk = sock.recv(n - len(buf))
        if not chunk:
            raise EOFError("unexpected EOF")
        buf.extend(chunk)
    return bytes(buf)


def _read_frame(sock: socket.socket) -> dict[str, Any]:
    hdr = _read_exact(sock, 4)
    (ln,) = struct.unpack(">I", hdr)
    if ln <= 0 or ln > 16 * 1024 * 1024:
        raise ValueError(f"invalid frame length {ln}")
    payload = _read_exact(sock, ln)
    return json.loads(payload.decode("utf-8"))


def _write_frame(sock: socket.socket, msg: dict[str, Any]) -> None:
    b = json.dumps(msg).encode("utf-8")
    sock.sendall(struct.pack(">I", len(b)) + b)


def fork_point() -> list[str]:
    """Block until fork supervisor requests a child, then fork and return args."""

    gc.freeze()

    sock_path = os.environ.get(SIGMA_FORK_SOCK)
    if not sock_path:
        raise RuntimeError(f"{SIGMA_FORK_SOCK} is not set")

    zygote_key = os.environ.get(SIGMA_FORK_ZYGOTE_KEY)
    if not zygote_key:
        raise RuntimeError(f"{SIGMA_FORK_ZYGOTE_KEY} is not set")

    # Persistent connection from the zygote to the supervisor.
    zsock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
    zsock.connect(sock_path)
    _write_frame(zsock, {"type": "hello", "zygote_key": zygote_key})
    resp = _read_frame(zsock)
    if resp.get("type") != "ok":
        raise RuntimeError(f"fork supervisor rejected hello: {resp}")

    while True:
        try:
            msg = _read_frame(zsock)
        except EOFError:
            # Supervisor closed the connection -> shutdown zygote.
            exit(0)

        if msg.get("type") != "fork":
            continue

        req_id = msg.get("req_id")
        env = msg.get("env") or []
        args = msg.get("args") or []

        pid = os.fork()
        if pid != 0:
            # Parent: continue servicing future fork requests.
            continue

        # Child: detach from the zygote connection to avoid sharing it.
        try:
            zsock.close()
        except Exception:
            pass

        # Create a fresh PID namespace so the child looks like a normal proc.
        if _unshare(CLONE_NEWPID) != 0:
            errno = ctypes.get_errno()
            raise OSError(errno, os.strerror(errno))

        pid2 = os.fork()
        if pid2 != 0:
            os._exit(0)

        # Notify supervisor that the child exists (peercred conveys host PID).
        csock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        csock.connect(sock_path)
        _write_frame(csock, {"type": "child", "req_id": req_id})
        _ = _read_frame(csock)
        csock.close()

        # Apply env updates for this child proc.
        # The supervisor sends either a list of "K=V" entries (preferred)
        # or a dict.
        if isinstance(env, dict):
            for k, v in env.items():
                os.environ[str(k)] = str(v)
        else:
            for entry in env:
                s = str(entry)
                if "=" not in s:
                    continue
                k, v = s.split("=", 1)
                os.environ[k] = v

        gc.enable()
        try:
            gc.unfreeze()
        except Exception:
            pass

        return [str(a) for a in args]
