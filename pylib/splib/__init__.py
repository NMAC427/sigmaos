"""SigmaOS Python client library.

This module wraps the pybind11 extension module (``_clntlib``).

Important: we intentionally do not auto-connect on import.
Forkable Python procs import ``splib`` before ``fork_point()`` and must avoid
creating sockets/threads in the parent. We instead lazily call ``init_socket()``
on first use of any RPC-like function.
"""

from __future__ import annotations

from typing import Any, Callable

import _clntlib as _c


_inited = False


def init_socket() -> None:
	global _inited
	if not _inited:
		_c.init_socket()
		_inited = True


def _ensure_init() -> None:
	init_socket()


# Re-export non-callable types directly.
Qid = _c.Qid
Stat = _c.Stat
Status = _c.Status
SigmaOSError = _c.SigmaOSError


def __getattr__(name: str) -> Any:
	attr = getattr(_c, name)
	if callable(attr):
		def _wrapped(*args: Any, **kwargs: Any) -> Any:
			_ensure_init()
			return attr(*args, **kwargs)

		_wrapped.__name__ = name
		return _wrapped
	return attr


__all__ = [
	"init_socket",
	"Qid",
	"Stat",
	"Status",
	"SigmaOSError",
]
