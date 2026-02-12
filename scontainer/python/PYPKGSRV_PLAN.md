# Plan: Centralized PyPkgSrv

## Problem

- Multiple dcontainers each run an isolated `PyMgr` singleton
- They share `/tmp/python/package-cache` on disk
- Atomic renames handle concurrent installs, but there is no cross-container coordination for **reference counting** needed for safe eviction

## Core observation

`installWheel` (python.go:284) only needs the `dcontainerPath` Python binary, which is available on the **host** at `/home/sigmaos/bin/kernel/cpython3.11`. So the coordinator can perform installs directly, without delegating into any container.

---

## Architecture

Introduce `PyPkgSrv` — a new RPC service that runs **inside the MSched process** (one per machine). The existing `PyMgr` in each dcontainer becomes a thin RPC client.

```
[dcontainer A]         [dcontainer B]
  PyMgr (thin client)    PyMgr (thin client)
       \                      /
        \____________________/
                 |
            RPC (sigmasrv)
                 |
         [MSched process]
          PyPkgSrv
           - downloads
           - installs
           - ref counting
           - eviction
```

---

## New files

| Path | Purpose |
|---|---|
| `scontainer/python/pypkgsrv/proto/pypkg.proto` | RPC messages |
| `scontainer/python/pypkgsrv/srv.go` | Service implementation (runs in MSched) |
| `scontainer/python/pypkgsrv/clnt.go` | RPC client (runs in dcontainer) |

## Modified files

| Path | Change |
|---|---|
| `scontainer/python/mgr.go` | Rewrite as thin client wrapper around `pypkgsrv.Clnt` |
| `scontainer/python/python.go` | Pass container/proc ID into `SetupSitePackages` |
| `sched/msched/srv/srv.go` | Start `PyPkgSrv` alongside MSched; hook `Exited()` to call `ReleaseAll` |

---

## Proto (pypkg.proto)

```protobuf
message WheelSpec {
  string name       = 1;
  string url        = 2;
  string sha256     = 3;
  int64  size       = 4;
  string pyVersion  = 5;
}

// Blocks until all wheels are installed. Increments ref counts.
message AcquireWheelsReq {
  string              procID = 1;
  repeated WheelSpec  wheels = 2;
}
message AcquireWheelsRep {
  repeated string installPaths = 1;  // parallel to wheels[]
}

// Decrements ref counts. Called on proc exit.
message ReleaseWheelsReq {
  string          procID = 1;
  repeated string keys   = 2;  // sha256+version
}
message ReleaseWheelsRep {}
```

---

## PyPkgSrv state (srv.go)

```go
type PyPkgSrv struct {
    mu               sync.Mutex
    installedWheels  map[string]*installResult     // key -> result
    pendingInstalls  map[string]*sync.Cond          // key -> cond
    downloadedWheels map[string]string              // URL -> tmp path
    pendingDownloads map[string]*sync.Cond          // URL -> cond
    refs             map[string]map[string]struct{} // key -> set of procIDs
    procKeys         map[string][]string            // procID -> acquired keys (for ReleaseAll)
    installSem       chan struct{}
    downloadSem      chan struct{}
}
```

This is essentially the existing `PyMgr` logic, plus `refs` and `procKeys` maps.

**RPC methods:**

```go
func (s *PyPkgSrv) AcquireWheels(ctx fs.CtxI, req *proto.AcquireWheelsReq, res *proto.AcquireWheelsRep) error
func (s *PyPkgSrv) ReleaseWheels(ctx fs.CtxI, req *proto.ReleaseWheelsReq, res *proto.ReleaseWheelsRep) error
```

`AcquireWheels` installs all requested wheels in parallel (one goroutine per wheel), then under the lock increments `refs[key][procID]` for each wheel and appends keys to `procKeys[procID]`.

---

## MSched integration

In `sched/msched/srv/srv.go`:

1. **Start**: after `NewMSched`, create and register `PyPkgSrv` at `/pypkgsrv/{kernelID}`
2. **Hook `Exited()`**: call `pyPkgSrv.ReleaseAll(procID)` when any proc exits

`ReleaseAll` removes the procID from all `refs[key]` entries it holds (using `procKeys[procID]`), enabling those packages to be evicted.

---

## Container-side (clnt.go + mgr.go rewrite)

The dcontainer has access to `procEnv` which contains both its `procID` and `kernelID`. The client:

1. On init, connects to `/pypkgsrv/{kernelID}` using the existing `SigmaClnt`
2. `InstallWheel` becomes `AcquireWheels` (batching all wheels for a given pylock)
3. Records acquired keys locally so they can be passed to `ReleaseWheels`

`SetupSitePackages` in `python.go` already calls `pm.InstallWheel()` in parallel goroutines — replace that with a single `AcquireWheels` batch call to the coordinator.

---

## Eviction (future step, not blocking)

Once ref counting is in place, add a background goroutine in `PyPkgSrv` that:
- Periodically scans `installedWheels`
- For any entry where `len(refs[key]) == 0`, removes it from disk and the map
- Respects a configurable size limit (e.g., evict LRU when cache exceeds N GB)

This is safe because the atomic `os.Rename` approach already ensures installs are crash-consistent, and eviction only touches packages with zero live references.

---

## Key design decisions

1. **Installs happen in the coordinator** (not delegated back to containers): the `dcontainerPath` python binary is available on the host, so no container involvement is needed
2. **Release is implicit on proc exit**: MSched's `Exited()` hook handles both clean exits and crashes — no container-side cleanup required
3. **Same locking structure as current PyMgr**: condition-variable-based deduplication is preserved, just moved to the coordinator where it now works across all containers
