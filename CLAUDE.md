# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build Commands

**IMPORTANT: Always use `./build.sh` to build the project. Do not use individual compilation scripts.**

### Build Flags
```bash
# Basic build for local development
./build.sh

# Parallel build with N jobs (default: uses all cores)
./build.sh -j 8

# Build only specific user procs (comma-separated list)
./build.sh --userbin sleeper,spinner

# Rebuild builder containers (rarely needed, e.g., after Dockerfile changes)
./build.sh --rebuildbuilder

# Skip specific build stages
./build.sh --no_go           # Skip all Go builds
./build.sh --no_go_user      # Skip Go user binaries only
./build.sh --no_cpp          # Skip C++ builds
./build.sh --no_py           # Skip Python builds
./build.sh --no_rs           # Skip Rust builds

# Debug build (for C++ components)
./build.sh --debug

# Build without Docker cache (slower, but ensures clean build)
./build.sh --nocache

# Enable race detector (for Go builds)
./build.sh --race

# Build for remote deployment with specific version
./build.sh --target remote --version 1.0 --push TAG

# Recompile protocol buffers (after modifying .proto files)
./compile-proto.sh
```

## Testing Commands

### Running Individual Tests

**Standard test pattern** (always stop existing instances first):
```bash
# Typical test invocation with debug logging
./stop.sh && SIGMADEBUG="TEST;BOOT;SYSTEM;BINSRV;PYPROXYSRV;PYPROXYSRV_ERR;SPPROXYCLNT;SPPROXYCLNT_ERR;CONTAINER;" \
  go test sigmaos/scontainer --run TestPythonStartup -v --start

# Simpler test without debug logging
./stop.sh && go test -v sigmaos/<pkg_name> --start --run <test_name>

# Example: Run InitFs test
./stop.sh && go test -v sigmaos/sigmaclnt/fslib --run InitFs --start
```

### Running Test Suites

```bash
# Run basic tests (default, tests core packages)
./test.sh 2>&1 | tee /tmp/out

# Run fast app tests only
./test.sh --apps-fast 2>&1 | tee /tmp/out

# Run all app tests (slow)
./test.sh --apps 2>&1 | tee /tmp/out

# Run tests with full cleanup between packages (ensures clean state)
./test.sh --cleanup 2>&1 | tee /tmp/out

# Test if packages compile (no execution)
./test.sh --compile 2>&1 | tee /tmp/out

# Skip to specific package in test sequence
./test.sh --skipto sigmaclnt/procclnt 2>&1 | tee /tmp/out

# Save logs for each test
./test.sh --savelogs 2>&1 | tee /tmp/out

# Reuse kernel between tests (requires --cleanup)
./test.sh --reuse-kernel --cleanup 2>&1 | tee /tmp/out

# Use spproxyd in tests
./test.sh --usespproxyd 2>&1 | tee /tmp/out

# Disable dialproxy in tests
./test.sh --nodialproxy 2>&1 | tee /tmp/out
```

### Test Infrastructure Notes
- **Always prefix tests with `./stop.sh &&`** to ensure clean state
- The `--start` flag indicates the test should start its own SigmaOS instance
- Omit `--start` when testing against an already-running cluster (e.g., remote benchmarks)
- Use `SIGMADEBUG` environment variable inline to enable debug logging (see debug/selector.go)
- Clear test cache with: `go clean -testcache`
- Tests may hang if previous instances weren't cleaned up

## Running SigmaOS

### Start and Mount
```bash
# Start dependencies
./start-etcd.sh

# Check etcd is running
docker exec etcd-server etcdctl version

# Create required directories (first time only)
mkdir -p ~/.aws /mnt/9p
sudo chown $USER /mnt/9p

# Boot and mount SigmaOS (replace LOCAL_IP with your machine's IP from hostname -I)
./mount.sh --boot LOCAL_IP

# Access SigmaOS namespace
ls /mnt/9p/

# Access S3 through SigmaOS
ls /mnt/9p/s3/SERVER_ID/
```

### Stop SigmaOS
```bash
# Stop and clean up containers
./stop.sh --parallel

# Stop without purging Docker cache (faster rebuilds)
./stop.sh --parallel --nopurge
```

### Debugging
```bash
# View logs from all containers
# Use this command AFTER running tests to see the complete debug output
./logs.sh

# Wipe etcd state
./fsetcd-wipe.sh

# Dump etcd contents
./fsetcd-dump.sh
```

## Architecture Overview

### Core System Components

SigmaOS is a cloud operating system with the following key components:

1. **Proc Model**: All computation happens in "procs" (processes)
   - Two types: Batch-and-Evict (T_BE) and Latency-Critical (T_LC)
   - Resource specs: Mcpu (CPU cores in 10s), Mem (memory in MB)
   - Serialized via protobuf for distribution across machines

2. **Scheduling Pipeline**: `MSched` (macro scheduler) → `BESched` (batch scheduler) → `ProcD` (process daemon) → `LCSched` (LC scheduler)

3. **Namespace and Naming**: Distributed namespace using `named` (backed by etcd) with realm-based multi-tenancy

4. **Protocol**: `Sigmap` - a 9P-inspired file-based protocol with extensions for leases, watches, and fault-tolerance

### Directory Structure

```
sigmaos/
├── proc/                   # Process abstraction and management
├── kernel/                 # Kernel components (named, msched, procd, etc.)
├── proxy/                  # Service proxies (sigmap, db, mongo, s3, ninep, wasm)
├── sched/                  # Schedulers (besched, lcsched, msched)
├── apps/                   # Application services (hotel, socialnetwork, cache, mr)
├── sigmaclnt/             # Client-side APIs (fslib, procclnt, fidclnt)
├── sigmap/                # Core protocol definitions
├── rpc/                   # RPC infrastructure
├── namesrv/               # Naming service (named, knamed)
├── realm/                 # Multi-tenancy/realm management
├── session/               # Session layer with connection recovery
├── spproto/               # Protocol server infrastructure
├── sigmasrv/              # Server-side infrastructure
├── cpp/                   # C++ implementations (IO, RPC, Python bindings)
├── pylib/                 # Python client library bindings
├── pyproc/                # Python process implementations
├── rs/                    # Rust components (WASM runtime)
├── cmd/kernel/            # Kernel service executables
├── cmd/user/              # User proc executables
├── test/                  # Test infrastructure
├── benchmarks/            # Benchmark suite
└── util/                  # Utilities (perf, auth, io, coordination)
```

### Client-Server Architecture

**Client Stack:**
```
User Code
  ↓
SigmaClnt (facade combining FsLib + ProcAPI)
  ↓
FsLib (file operations) / ProcClnt (process operations)
  ↓
FidClnt (file descriptor management)
  ↓
SessClnt (session layer with reconnection)
  ↓
NetClnt (TCP transport)
```

**Server Stack:**
```
NetSrv (TCP listener)
  ↓
SessSrv (session protocol)
  ↓
ProtoSrv (sigmap message handlers)
  ↓
Service implementation (named, msched, etc.)
```

### Key Libraries

- **sigmaclnt**: Main client API facade
- **fslib**: File system operations (Open, Read, Write, Create, etc.)
- **procclnt**: Process operations (Spawn, Evict, Wait)
- **fidclnt**: File descriptor tracking
- **sigmap**: Protocol types and operations (9P-inspired)
- **session**: Connection management with automatic recovery
- **rpc**: RPC infrastructure (clnt, srv, transport)
- **namesrv**: Naming service backed by etcd
- **sigmasrv**: Server-side infrastructure
- **protsrv**: Generic protocol server with handlers for sigmap messages

### Language Usage

- **Go**: Core system, schedulers, proxies, most services (~357 .go files)
- **C++**: Performance-critical paths (IO layer, RPC transport, Python bindings via clntlib)
- **Python**: User procs via pylib bindings to C++ clntlib
- **Rust**: WASM runtime and trampoline code
- **Protobuf**: Service definitions and serialization

### Protocol Buffers

All protobuf definitions are in `*/proto/` directories:
- `proc.proto`: Process definitions (ProcProto, ProcEnvProto)
- `sigmap.proto`: Protocol messages (Tread, Twrite, Topen, etc.)
- `rpc.proto`: Generic RPC envelope (Req, Rep)
- `kernel.proto`, `realm.proto`, `session.proto`: Core system protocols
- Application-specific protos in each app directory

After modifying `.proto` files, run `./compile-proto.sh` to regenerate Go code.

## Development Workflow

### Adding a New Service

1. Define protobuf interface in `yourservice/proto/yourservice.proto`
2. Run `./compile-proto.sh` to generate Go code
3. Implement service in `cmd/kernel/yourservice/` or `cmd/user/yourservice/`
4. Add handlers using `protsrv` infrastructure
5. Create client library using `rpcclnt` or `fslib`
6. Add tests in `yourservice/yourservice_test.go`
7. Rebuild with `./build.sh`

### Debugging Tips

**Enable debug logging (inline with test command):**
```bash
# Inline SIGMADEBUG (preferred pattern)
./stop.sh && SIGMADEBUG="TEST;BOOT;SYSTEM;" go test -v sigmaos/example --start --run YourTest

# Common SIGMADEBUG selectors (see debug/selector.go for complete list):
# TEST, BOOT, SYSTEM, BINSRV, PROCD, MSCHED, BESCHED, LCSCHED
# NAMED, KNAMED, SIGMAP, SIGMACLNT, FSETCD
# PYPROXYSRV, PYPROXYSRV_ERR, SPPROXYCLNT, SPPROXYCLNT_ERR
# CONTAINER, S3, DB, MONGO

# Example with comprehensive logging for Python/container debugging:
./stop.sh && SIGMADEBUG="TEST;BOOT;SYSTEM;BINSRV;PYPROXYSRV;PYPROXYSRV_ERR;SPPROXYCLNT;SPPROXYCLNT_ERR;CONTAINER;" \
  go test sigmaos/scontainer --run TestPythonStartup -v --start

# View logs from containers
./logs.sh
```

**Performance profiling (inline with test command):**
```bash
# Inline SIGMAPERF
./stop.sh && SIGMAPERF="NAMED_PPROF;NAMED_PPROF_MUTEX;" go test -v sigmaos/example --start --run YourTest

# Profile output will be in /tmp/sigmaos-perf/PID-selector.out
# Common SIGMAPERF selectors (see perf/selector.go for all selectors)
```

**Common debug patterns:**
1. Always prefix with `./stop.sh &&` to ensure clean state
2. Use inline `SIGMADEBUG` to enable relevant logging (don't export permanently)
3. Run test with `--start` flag
4. Check logs with `./logs.sh`
5. Inspect etcd state with `./fsetcd-dump.sh`

### Code Navigation

- **Process spawning**: Start at `proc/proc.go` → `kernel.go` → `sched/msched/`
- **File operations**: Start at `sigmaclnt/fslib/file.go` → `fidclnt` → `rpc/clnt`
- **Naming**: Check `namesrv/named.go` and etcd integration
- **Scheduling**: `sched/msched/` (cluster-wide) → `sched/besched/` (queueing)
- **Session layer**: `session/sessclnt/sessclnt.go` for client, `session/sesssrv/sesssrv.go` for server

## Configuration

### Build Configurations

Two main configurations:
- **local**: Fast builds, short timeouts, local storage (for development/testing)
- **aws**: Production builds, longer timeouts, S3 storage (for benchmarking/deployment)

Configuration is set via `-ldflags="-X sigmaos/sigmap.Target=local"` during build.

### AWS Configuration

SigmaOS requires AWS credentials in `~/.aws/credentials`:
```
[sigmaos]
aws_access_key_id = YOUR_KEY_ID
aws_secret_access_key = YOUR_SECRET_KEY
region=us-east-1
```

Test with: `aws s3 ls --profile sigmaos`

## Benchmarking

### Local Benchmarks
```bash
# Run benchmark tests
go test -v sigmaos/benchmarks --start --run BenchmarkName
```

### Remote Cluster Benchmarks

1. Deploy cluster (see `tutorial/02_remote_dev.md`)
2. Run benchmarks without `--start` flag (cluster already running)
3. See `benchmarks/remote/remote_test.go` for orchestration
4. Results stored in `benchmarks/results/VERSION`

Entry point: `benchmarks/benchmarks_test.go`
Command constructors: `benchmarks/remote/benchcmds.go`

## Common Patterns

### Proc Lifecycle
```go
// Spawn a proc
procID, err := sc.Spawn(procName, args, procType, mcpu, mem)

// Wait for proc to start
sc.WaitStart(procID)

// Wait for proc to exit
status, err := sc.WaitExit(procID)
```

### File Operations
```go
// Create and write
fd, err := sc.Create(path, perm)
sc.Write(fd, data)
sc.Close(fd)

// Read
fd, err := sc.Open(path)
data, err := sc.Read(fd)
sc.Close(fd)

// List directory
entries, err := sc.GetDir(path)
```

### RPC Server Pattern
1. Define proto messages
2. Create server with `sigmasrv.NewSigmaSrv()`
3. Implement handlers using `protsrv`
4. Register in namespace via `named`
5. Start serving

## Testing Patterns

Tests are colocated with code in `*_test.go` files. Common patterns:

```go
// Start test instance with just named
ts := test.NewTstatePath(t, "test-name")
defer ts.Shutdown()

// Start test instance with all kernel services
ts := test.NewTstateAll(t)
defer ts.Shutdown()

// Access client
sc := ts.SigmaClnt
```

Test infrastructure automatically:
- Boots required kernel components
- Cleans up on exit
- Provides isolated namespaces

## Dependencies

Key external dependencies (see `go.mod`):
- etcd (distributed configuration/naming)
- protobuf (serialization)
- Docker API (container management)
- AWS SDK (cloud integration)
- go-fuse (FUSE filesystem support)
- OpenTelemetry (distributed tracing)

Go version: 1.21

## Important Notes

- **Always use `./build.sh` to build** - do not use individual compilation scripts
- **Always prefix tests with `./stop.sh &&`** to ensure clean state and avoid conflicts
- **Use inline `SIGMADEBUG`** for debug logging - don't export it permanently
- Never use `sudo` with `build.sh` - if needed, fix Docker permissions instead
- Tests may require access to private S3 buckets (contact maintainers for access)
- Parallel builds (`-j N`) use significant memory - use with caution on constrained machines
- The `--nopurge` flag with `./stop.sh` preserves Docker build cache for faster rebuilds
- Protocol buffer changes require running `./compile-proto.sh` before rebuilding
