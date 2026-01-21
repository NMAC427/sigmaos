import os
import sys


is_forking = os.environ.get("SIGMA_FORK_ZYGOTE_KEY") is not None
if is_forking:
    from splib.fork import fork_point


import splib
import time
import random


# Allocate 100 MB of memory BEFORE forking
memory = bytearray(100 * 1024 * 1024)


if is_forking:
    args = fork_point()


splib.started()

# Modify some of the memory to trigger copy-on-write
memory[random.randint(0, len(memory) - 1)] = 1
time.sleep(60)

splib.exited(splib.Status.Ok, "Ok")
print(len(memory))

os._exit(0)
