#!/bin/bash

usage() {
  echo "Usage: $0 [--version VERSION]" 1>&2
}

VERSION="1.0"
while [[ "$#" -gt 0 ]]; do
  case "$1" in
  --version)
    shift
    VERSION="$1"
    shift
    ;;
  -help)
    usage
    exit 0
    ;;
  *)
   echo "unexpected argument $1"
   usage
   exit 1
  esac
done

if [ $# -gt 0 ]; then
    usage
    exit 1
fi

ROOT=$(pwd)
OUTPATH=./bin

mkdir -p $OUTPATH/kernel
mkdir -p $OUTPATH/user

# Inject custom Python lib
LIBDIR="/cpython3.11/Lib"
cp -r ./pylib/splib $LIBDIR

# Add checksum overrides for default libraries
OVERRIDEFILE="sigmaos-checksum-override"
for entry in "$LIBDIR"/*; do
  if [ -e "$entry" ]; then
    if [ -d "$entry" ]; then
      touch "$entry/$OVERRIDEFILE"
    elif [[ -f "$entry" && "$entry" == *.py ]]; then
      filename=$(basename "$entry" .py)
      touch "$LIBDIR/$filename-$OVERRIDEFILE"
    fi
  fi
done

# Copy OpenBLAS-0.3.23
cp /OpenBLAS-0.3.23/libopenblas64_p-r0.3.23.so $OUTPATH/kernel

# Copy Python executable
cp /cpython3.11/python "$OUTPATH/kernel"
cp -r /cpython3.11 "$OUTPATH/kernel"                    # TODO: Is this needed?
cp /cpython3.11/python "$OUTPATH/user"           # TODO: Why is this needed?
cp /cpython3.11/python "$OUTPATH/user/python-v$VERSION"  # TODO: Why is this needed?
echo "/tmp/python/Lib" > "$OUTPATH/kernel/python.pth"

cat > "$OUTPATH/kernel/pyvenv.cfg" <<EOF
home = /~~
include-system-site-packages = false
version = 3.11.13
EOF

# Build python shim
gcc -Wall -fPIC -shared -o $OUTPATH/kernel/ld_fstatat.so ./ld_preload/ld_fstatat.c -ldl

# Copy Python user processes
cp -r ./pyproc $OUTPATH/kernel
