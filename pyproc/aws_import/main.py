import splib
import dummy_package

splib.started()
dummy_package.sayHi()
splib.exited(splib.Status.Ok, "Exited normally!")
