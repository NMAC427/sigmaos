import splib
import shlex

splib.started()
print(shlex.join(["Hello", "World"]))
splib.exited(splib.Status.Ok, "Exited normally!")