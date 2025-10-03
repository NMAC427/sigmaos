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

# Copy Python executable
cp /cpython3.11/python "$OUTPATH/kernel"
cp -r /cpython3.11 "$OUTPATH/kernel"

echo "/tmp/python/Lib" > "$OUTPATH/kernel/python.pth"

cat > "$OUTPATH/kernel/pyvenv.cfg" <<EOF
home = /~~
include-system-site-packages = false
version = 3.11.13
EOF

# Copy libraries
PY_OUTPATH=$OUTPATH/kernel/cpython3.11
LIBDIR="$PY_OUTPATH/Lib"
cp ./pylib/_clntlib.cpython-311-*.so $LIBDIR
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
    elif [[ -f "$entry" && "$entry" == *.so ]]; then
      filename=$(basename "$entry" .so)
      touch "$LIBDIR/$filename-$OVERRIDEFILE"
    fi
  fi
done

# Build python shim
make -C ld_preload --no-print-directory
cp $ROOT/ld_preload/ld_preload.so $PY_OUTPATH/

# Copy Python user processes
cp -r ./pyproc $PY_OUTPATH
