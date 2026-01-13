import uuid
import splib

splib.started()

# File creation
pathname = f"name/tmp_{str(uuid.uuid4())}.txt"
fd = splib.create(pathname, 777, 0x01)
print("Fd:", fd)

# Write to file
data = "hello"
written = splib.write(fd, data)
print("Written:", written)

# Get the file contents
contents = splib.get_file(pathname)
print("Contents:", contents)

# Close the file
splib.close(fd)
print("File closed")

# Delete the file
splib.remove(pathname)
print("File deleted")

# The jail file system should be read-only, so the following should fail
try:
    with open("test.txt", "w") as f:
        f.write("This is a test file.")
    splib.exited(splib.Status.Failure, "File system should be read-only")
except Exception as e:
    print(e)
    pass

splib.exited(splib.Status.Ok, "Exited normally!")