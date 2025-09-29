import splib
import shlex

splib.started()
print(shlex.join(["Hello", "World"]))
splib.exited(1, "Exited normally!")
