# syntax=docker/dockerfile:1

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
        zip \
        wget

# Install Bazel
ARG BAZEL_VERSION
ENV BAZEL_VERSION=${BAZEL_VERSION:-8.4.1}

RUN curl -fSL "https://github.com/bazelbuild/bazel/releases/download/${BAZEL_VERSION}/bazel-${BAZEL_VERSION}-installer-linux-x86_64.sh" -o bazel-installer.sh && \
    chmod +x bazel-installer.sh && \
    ./bazel-installer.sh && \
    rm bazel-installer.sh

# Download an initial version of Go
ARG GO_VERSION=1.24.7
RUN curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" | tar -C /usr/local -xzf -

# Set the PATH to include the new Go install.
ENV PATH="${PATH}:/usr/local/go/bin"

# Install patched version of GO with a larger stack size
# Default is 2048, increase to 8194
RUN curl -fsSL "https://github.com/golang/go/archive/refs/tags/go${GO_VERSION}.tar.gz" | tar -C /tmp -xzf - && \
  mv "/tmp/go-go${GO_VERSION}" "/go${GO_VERSION}-bigstack" && \
  cd "/go${GO_VERSION}-bigstack/src" && \
  sed -i -E 's/^\s*(stackMin\s*=\s*)2048/\18194/' runtime/stack.go && \
  ./make.bash && \
  /go${GO_VERSION}-bigstack/bin/go version

ENV GOROOT="/go${GO_VERSION}-bigstack"

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