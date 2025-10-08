import splib

splib.started()

import shlex
print(shlex.join(["Hello", "World"]))

try:
    # Shouldn't be able to import from other directories
    from .. import hello
    splib.exited(splib.Status.Error, "Imported from other directories inside /pyproc should fail!")
    exit(1)
except ImportError as e:
    print("[EXPECTED] ImportError:", e)

splib.exited(splib.Status.Ok, "Exited normally!")