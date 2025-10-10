import splib

splib.started()

import numpy as np
print("Numpy version:", np.__version__)
print(np.array([1, 2, 3]))

splib.exited(splib.Status.Ok, "Exited normally!")
