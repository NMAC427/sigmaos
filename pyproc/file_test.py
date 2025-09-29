import splib

splib.started()
# File creation
pathname = "name/my_file"
fd = splib.create(pathname, 777, 0x01)
print("Fd:", fd)
# Write to file
data = "hello"
written = splib.write(fd, data)
print("Written:", written)
# Get the file contents
contents = splib.get_file(pathname)
print("Contents:", contents)
splib.exited(1, "Exited normally!")
