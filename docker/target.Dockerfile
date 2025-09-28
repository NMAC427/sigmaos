# syntax=docker/dockerfile:1

FROM ubuntu:24.04 AS base

RUN apt update && \
  apt install -y \
  libseccomp-dev \
  strace \
  fuse \
  libspdlog-dev \
  libprotobuf-dev \
  valgrind \
  libc6-dbg \
  libabsl-dev \
  curl \
  golang

# Install wasmer go pkg
RUN mkdir t && \
  cd t && \
  go mod init tmod && \
  go get github.com/NMAC427/wasmer-go/wasmer@latest

WORKDIR /home/sigmaos
RUN mkdir bin && \
    mkdir all-realm-bin && \
    mkdir bin/user && \
    mkdir bin/kernel && \
    mkdir bin/linux

# ========== local user image ==========
FROM base AS sigmauser-local
RUN mkdir jail && \
    mkdir /tmp/spproxyd

# ========== remote user image ==========
FROM sigmauser-local AS sigmauser-remote
ARG BIN_DIR
# Copy procd, the entrypoint for this container, to the user image.
COPY ${BIN_DIR}/kernel/procd bin/kernel/
# Copy spproxyd to the user image.
COPY ${BIN_DIR}/kernel/spproxyd bin/kernel/
## Copy rust trampoline to the user image.
COPY ${BIN_DIR}/kernel/uproc-trampoline bin/kernel/

# ========== local kernel image ==========
FROM base AS sigmaos-local
WORKDIR /home/sigmaos
ENV kernelid=kernel
ENV boot=named
ENV dbip=x.x.x.x
ENV mongoip=x.x.x.x
ENV buildtag="local-build"
ENV dialproxy="false"
# Install docker-cli
RUN apt install -y docker.io
ENV reserveMcpu="0"
ENV netmode="host"
ENV sigmauser="NOT_SET"

# Make a directory for binaries shared between realms.
RUN mkdir -p /home/sigmaos/bin/user/common
CMD ["/bin/sh", "-c", "bin/linux/bootkernel ${kernelid} ${named} ${boot} ${dbip} ${mongoip} ${reserveMcpu} ${buildtag} ${dialproxy} ${netmode} ${sigmauser}"]

# ========== remote kernel image ==========
FROM sigmaos-local AS sigmaos-remote
ARG BIN_DIR
ENV buildtag="remote-build"
# Copy linux bins
COPY ${BIN_DIR}/linux /home/sigmaos/bin/linux/
# Copy kernel bins
COPY ${BIN_DIR}/kernel /home/sigmaos/bin/kernel/
CMD ["/bin/sh", "-c", "bin/linux/bootkernel ${kernelid} ${named} ${boot} ${dbip} ${mongoip} ${reserveMcpu} ${buildtag} ${dialproxy} ${netmode} ${sigmauser}"]
