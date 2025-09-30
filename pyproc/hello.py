# Install fault handler so we can see a proter backtrace for segmentation faults
# caused by the splib library.
import faulthandler
faulthandler.enable()

import splib

splib.started()
print("Hello World!")
splib.exited(1, "Exited normally!")
