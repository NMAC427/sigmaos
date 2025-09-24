#!/bin/bash

set -euo pipefail

REBUILD_FLAG=false
BAZEL_ARGS=()
for arg in "$@"; do
    case $arg in
        --rebuild-builder)
        REBUILD_FLAG=true
        shift # Remove --rebuild-builder from the list of arguments
        ;;
        *)
        BAZEL_ARGS+=("$arg")
        shift # Remove the current argument
        ;;
    esac
done

# If no specific bazel commands were provided, default to building everything.
if [ ${#BAZEL_ARGS[@]} -eq 0 ]; then
    BAZEL_ARGS=("build" "//...")
fi

ROOT=$(dirname $(realpath $0))

BUILDER_NAME="sig-bazel-builder"

BUILD_LOG="/tmp/sigmaos-build"

# Start / Rebuild Builder
buildercid=$((docker ps -a | grep -E " $BUILDER_NAME " | cut -d " " -f1) || true)
if [[ $REBUILD_FLAG == "true" ]]; then
  if ! [ -z "$buildercid" ]; then
    echo "========== Stopping old builder container $buildercid =========="
    docker stop $buildercid
  fi

  echo "========== Build builder image =========="
  DOCKER_BUILDKIT=1 docker build --progress=plain -f docker/bazel.Dockerfile -t $BUILDER_NAME . 2>&1 | tee $BUILD_LOG/sig-bazel-builder.out
  buildercid=""
  echo "========== Done building builder =========="
fi

if [ -z "$buildercid" ]; then
  echo "========== Starting builder container =========="
  docker run --rm -d -it \
    --name $BUILDER_NAME \
    --user $( id -u ):$( id -g ) \
    --mount "type=bind,src=${ROOT},dst=/src" \
    "${BUILDER_NAME}"
  buildercid=$(docker ps -a | grep -E " $BUILDER_NAME " | cut -d " " -f1)
  until [ "`docker inspect -f {{.State.Running}} $buildercid`"=="true" ]; do
      echo -n "." 1>&2
      sleep 0.1;
  done
  echo "========== Done starting builder ========== "
fi

if [ -t 1 ] ; then
  TTY_FLAG="-it";
else
  TTY_FLAG="";
fi

docker exec $TTY_FLAG $buildercid \
  bazel "${BAZEL_ARGS[@]}"