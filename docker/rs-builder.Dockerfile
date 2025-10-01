# syntax=docker/dockerfile:1-experimental

FROM ubuntu:24.04

RUN apt-get update && apt-get install -y \
  curl \
  gcc \
  time \
  parallel \
  libseccomp-dev \
  binaryen \
  build-essential

RUN echo 'will cite' | parallel --citation || true

WORKDIR /home/sigmaos
RUN mkdir -p bin/kernel && \
  mkdir -p bin/user

# Copy rust trampoline
COPY rs rs

# Set up builder user
ARG USER_ID=1000
ARG GROUP_ID=1000

RUN groupadd -g ${GROUP_ID} builder && \
    useradd -m -u ${USER_ID} -g ${GROUP_ID} -s /bin/bash builder

USER builder

# Install rust
RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y
ENV PATH="/home/builder/.cargo/bin:${PATH}"
RUN rustup update \
    && cargo install wasm-pack protobuf-codegen \
    && rustup target add wasm32-unknown-unknown

ENV LIBSECCOMP_LINK_TYPE=static
ENV LIBSECCOMP_LIB_PATH="/usr/lib/x86_64-linux-gnu"

CMD [ "/bin/bash", "-l" ]
