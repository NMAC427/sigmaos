#!/bin/bash

usage() {
  echo "Usage: $0 [--version VERSION] [-j NJOBS] [--build_type BUILD_TYPE]" 1>&2
}

VERSION="1.0"
NJOBS=$(nproc)
CMAKE_BUILD_TYPE=""
while [[ "$#" -gt 0 ]]; do
  case "$1" in
  --version)
    shift
    VERSION="$1"
    shift
    ;;
  -j)
    shift
    NJOBS="$1"
    shift
    ;;
  --build_type)
    shift
    BUILD_TYPE="$1"
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
USERBIN=$ROOT/bin/user

# Compile protobufs
find cpp -iname "*.pb.cc" -o -iname "*.pb.h" -delete

proto_cmds=()

for P in \
  sigmap/*.proto \
  proc/*.proto \
  proxy/sigmap/proto/*.proto \
  rpc/proto/*.proto;
do
  proto_cmds+=("protoc -I=. --cpp_out=./cpp $P")
done

proto_cmds+=("protoc -I=. --cpp_out=./cpp/apps/echo/proto --proto_path example/example_echo_server/proto example_echo_server.proto")
proto_cmds+=("protoc -I=. --cpp_out=./cpp/apps/spin/proto --proto_path apps/spin/proto spin.proto")
proto_cmds+=("protoc -I=. --cpp_out=./cpp/apps/cossim/proto --proto_path apps/cossim/proto cossim.proto")
proto_cmds+=("protoc -I=. --cpp_out=./cpp/apps/epcache/proto --proto_path apps/epcache/proto epcache.proto")
proto_cmds+=("protoc -I=. --cpp_out=./cpp/apps/cache/proto --proto_path apps/cache/proto cache.proto")
proto_cmds+=("protoc -I=. --cpp_out=./cpp util/tracing/proto/tracing.proto")

printf "%s\n" "${proto_cmds[@]}" | parallel -j"$NJOBS" --verbose

# Make a build directory
cd cpp
mkdir -p build

# Run the build
cd build
cmake .. -DCMAKE_BUILD_TYPE="${CMAKE_BUILD_TYPE}" && \
  cmake --build . -j "$NJOBS"

export EXIT_STATUS=$?
if [ $EXIT_STATUS  -ne 0 ]; then
  exit $EXIT_STATUS
fi

# Copy to bins
cd $ROOT
USERBUILD=$ROOT/cpp/build/user
for p in $USERBUILD/* ; do
  name=$(basename $p)
  # Skip non-directories, and CMakefiles directory
  if ! [ -d $p ] || [[ "$name" == "CMakeFiles" ]] ; then
    continue
  fi
  # Copy to userbin
  cp $p/$name $USERBIN/$name-v$VERSION
done

# Copy shared libs
cp $ROOT/cpp/build/libsigmaos_core.so $ROOT/bin/kernel/lib/
cp $ROOT/cpp/build/python/_clntlib.*.so $ROOT/pylib/
