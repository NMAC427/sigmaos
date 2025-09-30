# syntax=docker/dockerfile:1-experimental

FROM ubuntu:24.04

RUN apt update && \
  apt install -y \
  git \
  wget \
  gcc \
  pkg-config \
  parallel \
  time \
  cmake \
  ccache \
  libprotobuf-dev \
  libseccomp-dev \
  libspdlog-dev \
  libabsl-dev \
  libffi-dev \
  libssl-dev \
  libprotoc-dev \
  protobuf-compiler

# Install specific version of OpenBLAS
RUN wget -P / https://github.com/xianyi/OpenBLAS/releases/download/v0.3.23/OpenBLAS-0.3.23.tar.gz && \
  tar -xzf /OpenBLAS-0.3.23.tar.gz && \
  rm /OpenBLAS-0.3.23.tar.gz && \
  cd /OpenBLAS-0.3.23 && \
  make -j 8 USE_THREAD=1 INTERFACE64=1 DYNAMIC_ARCH=1 SYMBOLSUFFIX=64_ CFLAGS="-fcommon -Wno-error=incompatible-pointer-types"

# Install Python
RUN wget https://github.com/python/cpython/archive/refs/tags/v3.11.13.tar.gz -O /cpython.tar.gz && \
    tar -xzf /cpython.tar.gz -C / && \
    rm /cpython.tar.gz && \
    mv /cpython-3.11.13 /cpython3.11 && \
    cd /cpython3.11 && \
    ./configure \
      --prefix=/home/sigmaos/bin/user \
      --exec-prefix=/home/sigmaos/bin/user \
      --with-ensurepip=install && \
    make -j

WORKDIR /home/sigmaos

CMD [ "/bin/bash", "-l" ]
