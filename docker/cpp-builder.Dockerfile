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
    binutils-dev \
    libprotobuf-dev \
    libseccomp-dev \
    libspdlog-dev \
    libabsl-dev \
    libprotoc-dev \
    protobuf-compiler \
    software-properties-common && \
  add-apt-repository -y ppa:deadsnakes/ppa && \
  apt install -y \
    python3.11-dev

# Set up builder user
ARG USER_ID=1000
ARG GROUP_ID=1000

RUN groupadd -g ${GROUP_ID} builder && \
    useradd -m -u ${USER_ID} -g ${GROUP_ID} -s /bin/bash builder

USER builder

WORKDIR /home/sigmaos

CMD [ "/bin/bash", "-l" ]
