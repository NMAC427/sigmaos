import os, sys, sysconfig, site, platform, pprint

def dump_python_env():
    print("=== Environment Variables ===")
    pprint.pprint(dict(os.environ))

    print("\n=== sys.path ===")
    pprint.pprint(sys.path)

    print("\n=== Loaded Modules ===")
    pprint.pprint(list(sys.modules.keys()))

    print("\n=== Meta Path Finders ===")
    pprint.pprint(sys.meta_path)

    print("\n=== Path Hooks ===")
    pprint.pprint(sys.path_hooks)

    print("\n=== Path Importer Cache ===")
    pprint.pprint(sys.path_importer_cache)

    print("\n=== Built-in Modules ===")
    pprint.pprint(sys.builtin_module_names)

    print("\n=== Installation Paths ===")
    pprint.pprint(sysconfig.get_paths())

    print("\n=== Site Packages ===")
    pprint.pprint(site.getsitepackages())

    print("\n=== Python Executable and Platform Info ===")
    print("Executable:", sys.executable)
    print("Version:", sys.version)
    print("Platform:", platform.platform())
    print("Implementation:", platform.python_implementation())

dump_python_env()


import splib
help(splib)

splib.started()
print("Hello World!")
splib.exited(splib.Status.Ok, "Exited normally!")
