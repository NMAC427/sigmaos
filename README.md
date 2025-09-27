SigmaOS is an experimental cloud operating system under development.
See `tutorial/` for how to run it.

The SOSP24 SigmaOS paper is available [here](https://pdos.csail.mit.edu/papers/sigmaos:sosp24.pdf).

## Bazel run targets
- `./build.sh run //:gazelle` generate go BUILD files
    - `./build.sh mod tidy` and `./build.sh run @rules_go//go -- mod tidy` for dependencies
- `./build.sh run //tools/genbuild:bin` to generate the BUILD file in `/bin/gen` to handle
  packaging of all binaries that should get shipped with SigmaOS.