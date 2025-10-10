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

# Copy python binary
rm -rf "$OUTPATH/kernel/cpython3.11"
cp -r /cpython3.11 "$OUTPATH/kernel"

PY_OUTPATH=$OUTPATH/kernel/cpython3.11
PY_USER_SITE_PACKAGES="$PY_OUTPATH/sigmaos/user/site-packages"
PY_KERNEL_SITE_PACKAGES="$PY_OUTPATH/sigmaos/kernel/site-packages"

mkdir -p "$PY_USER_SITE_PACKAGES"
mkdir -p "$PY_KERNEL_SITE_PACKAGES"

# Copy user libraries (used by user processes)
cp ./pylib/_clntlib.cpython-311-*.so "$PY_USER_SITE_PACKAGES"
cp -r ./pylib/splib "$PY_USER_SITE_PACKAGES"

# Install kernel libraries (used by kernel processes)
wget -q -O "/tmp/installer.whl" "https://files.pythonhosted.org/packages/e5/ca/1172b6638d52f2d6caa2dd262ec4c811ba59eee96d54a7701930726bce18/installer-0.7.0-py3-none-any.whl"
unzip -q -d "$PY_KERNEL_SITE_PACKAGES" "/tmp/installer.whl"

wget -q -O "/tmp/packaging.whl" "https://files.pythonhosted.org/packages/20/12/38679034af332785aac8774540895e234f4d07f7545804097de4b666afd8/packaging-25.0-py3-none-any.whl"
unzip -q -d "$PY_KERNEL_SITE_PACKAGES" "/tmp/packaging.whl"

function py() {
  PYTHONPATH="$PY_OUTPATH/build/lib.linux-x86_64-3.11:$PY_OUTPATH/Lib:$PY_KERNEL_SITE_PACKAGES" $PY_OUTPATH/python $@
}

# Generate sys_tags file, containing a list of all supported platform compatibility tags
# https://packaging.python.org/en/latest/specifications/platform-compatibility-tags/
py > "$PY_OUTPATH/sigmaos/sys_tags" <<EOF
import packaging.tags
for tag in packaging.tags.sys_tags():
    print(tag)
EOF

cp "$ROOT/scontainer/python/install_wheel.py" "$PY_OUTPATH/sigmaos/kernel"

# Add checksum overrides for default libraries
OVERRIDEFILE="sigmaos-checksum-override"
for entry in "$PY_OUTPATH/Lib/*"; do
  if [ -e "$entry" ]; then
    if [ -d "$entry" ]; then
      touch "$entry/$OVERRIDEFILE"
    elif [[ -f "$entry" && "$entry" == *.py ]]; then
      filename=$(basename "$entry" .py)
      touch "$PY_OUTPATH/Lib/$filename-$OVERRIDEFILE"
    elif [[ -f "$entry" && "$entry" == *.so ]]; then
      filename=$(basename "$entry" .so)
      touch "$PY_OUTPATH/Lib/$filename-$OVERRIDEFILE"
    fi
  fi
done

# Build python shim
make -C ld_preload --no-print-directory
cp $ROOT/ld_preload/ld_preload.so $PY_OUTPATH/sigmaos

# Copy Python user processes
cp -r ./pyproc $PY_OUTPATH
