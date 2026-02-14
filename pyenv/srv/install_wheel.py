import sys
import os
from installer import install
from installer.sources import WheelFile
from installer.destinations import SchemeDictionaryDestination

wheel_path = sys.argv[1]
target = sys.argv[2]

dest = SchemeDictionaryDestination(
    {
        "purelib": os.path.join(target, "site-packages"),
        "platlib": os.path.join(target, "site-packages"),
        "scripts": os.path.join(target, "bin"),
        "data": os.path.join(target, "data"),
    },
    interpreter=sys.executable,
    script_kind="posix",
)

with WheelFile.open(wheel_path) as wheel:
    install(wheel, dest, additional_metadata={})
