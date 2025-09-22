#!/bin/bash
ROOT=$(pwd)
OUTPATH=./bin

mkdir -p $OUTPATH/kernel
mkdir -p $OUTPATH/user

# Inject custom Python lib
LIBDIR="/cpython3.11/Lib"
cp ./pylib/splib.py $LIBDIR

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
cp -r /cpython3.11 "$OUTPATH/kernel"
echo "/tmp/python/Lib" > "$OUTPATH/kernel/python.pth"

cat > "$OUTPATH/kernel/pyvenv.cfg" <<EOF
home = /~~
include-system-site-packages = false
version = 3.11.10
EOF

# Copy and inject Python shim
gcc -Wall -fPIC -shared -o ld_fstatat.so ./ld_preload/ld_fstatat.c -ldl
cp ld_fstatat.so $OUTPATH/kernel

# Build Python library
gcc -I../sigmaos -Wall -fPIC -shared -L/usr/lib -lprotobuf-c \
  -o $OUTPATH/kernel/clntlib.so \
  ../sigmaos/pylib/clntlib.c \
  ../sigmaos/pylib/proto/*.pb-c.c

# Copy Python user processes
cp -r ./pyproc $OUTPATH/kernel
