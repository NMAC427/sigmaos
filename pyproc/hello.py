import splib

help(splib)

splib.started()
print("Hello World!")
splib.exited(splib.Status.Ok, "Exited normally!")
