# syntax=docker/dockerfile:1-experimental

FROM ubuntu:24.04 AS base

# Install essential build tools: a C++ compiler, linker, standard libraries,
# and other utilities needed for Bazel and building C++ code.
RUN apt-get update && \
    apt-get install --yes \
        build-essential \
        curl \
        git \
        openjdk-8-jdk \
        python3 \
        python3-pip \
        unzip \
        zip

# --- Install Bazel ---
ARG BAZEL_VERSION
ENV BAZEL_VERSION=${BAZEL_VERSION:-8.4.1}

RUN curl -fSL "https://github.com/bazelbuild/bazel/releases/download/${BAZEL_VERSION}/bazel-${BAZEL_VERSION}-installer-linux-x86_64.sh" -o bazel-installer.sh && \
    chmod +x bazel-installer.sh && \
    ./bazel-installer.sh && \
    rm bazel-installer.sh

# Install additional dependencies
RUN apt install -y \
  libseccomp-dev

ENV LIBSECCOMP_LINK_TYPE=static
ENV LIBSECCOMP_LIB_PATH="/usr/lib"

# Setup user
RUN groupadd -g 1001 builder && \
    useradd -m -u 1001 -g 1001 -s /bin/bash builder
USER builder
WORKDIR /home/builder

CMD [ "/bin/bash", "-l" ]