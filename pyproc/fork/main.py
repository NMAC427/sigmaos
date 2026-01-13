#!/usr/bin/env python3
"""
Python fork test program.

This program demonstrates the fork_point API. All imports and initialization
code before fork_point() are shared across all forked children via copy-on-write.
"""
from splib.fork import fork_point

import os
import sys

import splib

def main(args: list[str]):
    """Main function that runs after fork_point returns."""
    child_name = args[0] if args else "unnamed"
    print(f"[{child_name}] Running in forked child, pid={os.getpid()}")

    splib.started()
    print(f"[{child_name}] Hello from forked Python proc!")
    splib.exited(splib.Status.Ok, f"Child {child_name} completed successfully")


# This is the entry point for the Zygote
if __name__ == "__main__":
    print(f"[zygote] Starting Zygote process, pid={os.getpid()}")

    import time
    time.sleep(1)

    args = fork_point()

    main(args)
    os._exit(0)
