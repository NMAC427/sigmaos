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
PATH_HASH=$(echo ${ROOT} | md5sum | cut -c -8)

BUILDER_IMAGE_NAME="sig-bazel-builder"
BUILDER_CONTAINER_NAME="${BUILDER_IMAGE_NAME}_${PATH_HASH}"

BUILD_LOG="/tmp/sigmaos-build"
mkdir -p "$BUILD_LOG"

function container_exists() {
  docker container inspect $BUILDER_CONTAINER_NAME >/dev/null 2>&1 && echo "true" || echo "false"
}

function builder_running() {
  if [[ "$(container_exists)" == "true" ]]; then
    docker container inspect -f '{{.State.Running}}' $BUILDER_CONTAINER_NAME 2>/dev/null || echo "false"
  else
    echo "false"
  fi
}

if [[ $REBUILD_FLAG == "true" ]]; then
  if [[ "$(container_exists)" == "true" ]]; then
    echo "========== Stopping and removing old builder container $BUILDER_CONTAINER_NAME =========="
    docker rm -f $BUILDER_CONTAINER_NAME
  fi

  echo "========== Build builder image =========="
  DOCKER_BUILDKIT=1 docker build \
    --progress=plain \
    --build-arg DOCKER_GID=$(getent group docker | cut -d: -f3) \
    -f docker/bazel.Dockerfile \
    -t $BUILDER_IMAGE_NAME . \
    2>&1 | tee $BUILD_LOG/sig-bazel-builder.out
  echo "========== Done building builder =========="
fi

if [[ "$(builder_running)" != "true" ]]; then
    # If the container exists but is stopped, remove it before trying to run a new one.
    if [[ "$(container_exists)" == "true" ]]; then
        echo "========== Removing stopped container $BUILDER_CONTAINER_NAME =========="
        docker rm $BUILDER_CONTAINER_NAME
    fi
  echo "========== Starting builder container =========="
  mkdir -p /tmp/bazel_build_cache
  docker run --rm -d -it \
    -v /var/run/docker.sock:/var/run/docker.sock \
    --name "${BUILDER_CONTAINER_NAME}" \
    --user "$(id -u):$(id -g)" \
    --mount "type=bind,src=/tmp/bazel_build_cache,dst=/tmp/bazel_build_cache" \
    --mount "type=bind,src=${ROOT},dst=/home/builder/${ROOT}" \
    "${BUILDER_IMAGE_NAME}"
  # Loop until the container is in a running state.
  until [[ "$(builder_running)" == "true" ]]; do
      echo -n "." 1>&2
      sleep 0.1
  done
  echo "========== Done starting builder ========== "
fi

if [ -t 1 ] ; then
  TTY_FLAG="-it"
else
  TTY_FLAG=""
fi

docker exec $TTY_FLAG \
  -w "/home/builder/${ROOT}" \
  -e "PATH_HASH=${PATH_HASH}" \
  "${BUILDER_CONTAINER_NAME}" \
  bazel "${BAZEL_ARGS[@]}"