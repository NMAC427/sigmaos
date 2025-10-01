# syntax=docker/dockerfile:1-experimental

FROM alpine

RUN apk add --no-cache libseccomp \
  gcompat \
  musl-dev \
  curl \
  bash \
  gcc \
  libc-dev \
  parallel \
  binaryen \
  libseccomp-static

RUN echo 'will cite' | parallel --citation || true

ARG USER_ID=1000
ARG GROUP_ID=1000

RUN addgroup -g ${GROUP_ID} builder && \
    adduser -D -u ${USER_ID} -G builder builder

USER builder

WORKDIR /home/sigmaos
RUN mkdir -p bin/kernel && \
  mkdir -p bin/user

# Install rust
RUN curl https://sh.rustup.rs -sSf | bash -s -- -y
ENV PATH="/home/builder/.cargo/bin:${PATH}"

RUN rustup update
RUN cargo install \
  wasm-pack \
  protobuf-codegen
RUN rustup target add wasm32-unknown-unknown

# Copy rust trampoline
COPY rs rs
ENV LIBSECCOMP_LINK_TYPE=static
ENV LIBSECCOMP_LIB_PATH="/usr/lib"

CMD [ "/bin/bash", "-l" ]
