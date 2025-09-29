print("Hi")
import time
time.sleep(1)
print("Wake")

import faulthandler
faulthandler.enable()
time.sleep(1)

import splib

splib.started()
print("Hello World!")
splib.exited(1, "Exited normally!")
