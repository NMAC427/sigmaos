package srv

import (
	"bufio"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/scontainer"
	"sigmaos/scontainer/python"
	sp "sigmaos/sigmap"
)

const (
	forkSockEnv    = "SIGMA_FORK_SOCK"
	forkZygoteEnv  = "SIGMA_FORK_ZYGOTE_KEY"
	defaultSockRel = "/tmp/sigma_fork.sock"
)

type forkMsg struct {
	Type      string   `json:"type"`
	ZygoteKey string   `json:"zygote_key,omitempty"`
	ReqID     string   `json:"req_id,omitempty"`
	Env       []string `json:"env,omitempty"`
	Args      []string `json:"args,omitempty"`
}

func writeFrame(w io.Writer, msg any) error {
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(len(b)))
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}

func readFrame(r io.Reader, out any) error {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return err
	}
	n := binary.BigEndian.Uint32(hdr[:])
	if n == 0 || n > 16*1024*1024 {
		return fmt.Errorf("invalid frame length %d", n)
	}
	b := make([]byte, n)
	if _, err := io.ReadFull(r, b); err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

func randID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

type zygoteEntry struct {
	key       string
	keepAlive time.Duration

	mu       sync.Mutex
	sendMu   sync.Mutex
	listener *net.UnixListener
	zygConn  *net.UnixConn
	readyCh  chan struct{}
	pending  map[string]chan int

	zygProc *proc.Proc
	zygCmd  *scontainer.UProcCmd

	children int
	lastIdle time.Time
	evictT   *time.Timer

	closed chan struct{}

	exitErr error
}

type forkMgr struct {
	ps *ProcSrv

	mu      sync.Mutex
	zygotes map[string]*zygoteEntry
}

func newForkMgr(ps *ProcSrv) *forkMgr {
	return &forkMgr{ps: ps, zygotes: make(map[string]*zygoteEntry)}
}

func (fm *forkMgr) forkSockHostPath(pid sp.Tpid) string {
	// The uproc-trampoline bind-mounts jail/<pid>/tmp as /tmp inside the proc.
	return filepath.Join(sp.SIGMAHOME, "jail", pid.String(), "tmp", filepath.Base(defaultSockRel))
}

func (fm *forkMgr) ensureZygote(uproc *proc.Proc) (*zygoteEntry, error) {
	fp := uproc.GetForkProc()
	if fp == nil {
		return nil, fmt.Errorf("ensureZygote called for non-fork proc")
	}
	key := fp.GetZygoteKey()

	// Check if we already have a matching Zygote running.
	fm.mu.Lock()
	if ze, ok := fm.zygotes[key]; ok {
		fm.mu.Unlock()
		return ze, nil
	}
	ze := &zygoteEntry{
		key:       key,
		keepAlive: time.Duration(fp.GetKeepAliveNs()),
		readyCh:   make(chan struct{}),
		pending:   make(map[string]chan int),
		closed:    make(chan struct{}),
	}
	fm.zygotes[key] = ze
	fm.mu.Unlock()

	// Build the zygote proc. It inherits the realm/secrets/sigmap endpoints from
	// the forked proc, but runs without SPProxy until after the fork point.
	zyg := proc.NewProc(fp.GetZygoteProgram(), append([]string{}, fp.GetZygoteArgs()...))
	zyg.InheritParentProcEnv(uproc.GetProcEnv())
	zyg.SetRealm(uproc.GetRealm())
	for k, v := range fp.GetZygoteEnv() {
		zyg.AppendEnv(k, v)
	}
	zyg.GetProcEnv().UseSPProxy = false
	zyg.GetProcEnv().UseSPProxyProcClnt = false
	zyg.SetType(uproc.GetType())
	zyg.SetMcpu(uproc.GetMcpu())
	zyg.SetMem(uproc.GetMem())

	// Set up supervisor socket in the zygote jail tmp dir.
	sockHost := fm.forkSockHostPath(zyg.GetPid())
	if err := os.MkdirAll(filepath.Dir(sockHost), 0777); err != nil {
		return nil, fmt.Errorf("mkdir fork sock dir: %w", err)
	}
	_ = os.Remove(sockHost)
	addr, err := net.ResolveUnixAddr("unix", sockHost)
	if err != nil {
		return nil, err
	}
	l, err := net.ListenUnix("unix", addr)
	if err != nil {
		return nil, err
	}
	ze.listener = l
	ze.zygProc = zyg

	zyg.AppendEnv(forkSockEnv, defaultSockRel)
	zyg.AppendEnv(forkZygoteEnv, key)

	// Assign procd to realm and finalize env before starting the container.
	isPythonProc := python.IsSupportedPythonVersion(zyg.GetProgram()) != nil
	var stringProg string
	if isPythonProc {
		stringProg = zyg.GetProgram()
	} else {
		stringProg = zyg.GetVersionedProgram()
	}
	if err := fm.ps.assignToRealm(zyg.GetRealm(), zyg.GetPid(), stringProg, zyg.GetSigmaPath(), zyg.GetSecrets()["s3"], zyg.GetNamedEndpoint()); err != nil {
		return nil, err
	}
	zyg.FinalizeEnv(fm.ps.pe.GetInnerContainerIP(), fm.ps.pe.GetOuterContainerIP(), fm.ps.pe.GetPID())

	// Start accept loop first, then start the zygote container.
	go fm.acceptLoop(ze)
	cmd, err := scontainer.StartSigmaContainer(zyg, fm.ps.dialproxy)
	if err != nil {
		return nil, err
	}
	ze.zygCmd = cmd

	go fm.waitZygoteExit(ze)
	return ze, nil
}

func (fm *forkMgr) acceptLoop(ze *zygoteEntry) {
	for {
		conn, err := ze.listener.AcceptUnix()
		if err != nil {
			select {
			case <-ze.closed:
				return
			default:
			}
			db.DPrintf(db.PROCD_ERR, "forkmgr accept error: %v", err)
			continue
		}
		go fm.handleConn(ze, conn)
	}
}

func (fm *forkMgr) handleConn(ze *zygoteEntry, conn *net.UnixConn) {
	defer func() {
		if conn != nil {
			_ = conn.Close()
		}
	}()
	r := bufio.NewReader(conn)
	var m forkMsg
	if err := readFrame(r, &m); err != nil {
		return
	}

	switch m.Type {
	case "hello":
		if m.ZygoteKey != ze.key {
			_ = writeFrame(conn, forkMsg{Type: "err"})
			return
		}
		var zc *net.UnixConn
		ze.mu.Lock()
		if ze.zygConn == nil {
			ze.zygConn = conn
			// Keep the zygote conn open; prevent deferred close.
			conn = nil
			close(ze.readyCh)
			ze.lastIdle = time.Now()
		}
		zc = ze.zygConn
		ze.mu.Unlock()
		if zc != nil {
			ze.sendMu.Lock()
			_ = writeFrame(zc, forkMsg{Type: "ok"})
			ze.sendMu.Unlock()
		}
	case "child":
		// Identify host PID via SO_PEERCRED.
		f, err := conn.File()
		if err != nil {
			return
		}
		ucred, err := syscall.GetsockoptUcred(int(f.Fd()), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
		_ = f.Close()
		if err != nil {
			return
		}
		hostPid := int(ucred.Pid)
		ze.mu.Lock()
		ch := ze.pending[m.ReqID]
		if ch != nil {
			delete(ze.pending, m.ReqID)
		}
		ze.mu.Unlock()
		if ch != nil {
			ch <- hostPid
			close(ch)
		}
		_ = writeFrame(conn, forkMsg{Type: "ok"})
	default:
		_ = writeFrame(conn, forkMsg{Type: "err"})
	}
}

func (fm *forkMgr) waitZygoteExit(ze *zygoteEntry) {
	err := ze.zygCmd.Wait()
	ze.mu.Lock()
	ze.exitErr = err
	ze.mu.Unlock()
	if err != nil {
		db.DPrintf(db.PROCD_ERR, "zygote %s exited: %v", ze.key, err)
	}
	close(ze.closed)
	_ = ze.listener.Close()
	if ze.zygConn != nil {
		_ = ze.zygConn.Close()
	}
	scontainer.CleanupUProc(ze.zygCmd)

	fm.mu.Lock()
	delete(fm.zygotes, ze.key)
	fm.mu.Unlock()
}

// Returns the host PID of the forked child.
func (fm *forkMgr) forkChild(uproc *proc.Proc) (int, error) {
	fp := uproc.GetForkProc()
	if fp == nil {
		return 0, fmt.Errorf("forkChild called for non-fork proc")
	}

	ze, err := fm.ensureZygote(uproc)
	if err != nil {
		return 0, err
	}
	select {
	case <-ze.readyCh:
	case <-ze.closed:
		ze.mu.Lock()
		err := ze.exitErr
		ze.mu.Unlock()
		return 0, fmt.Errorf("zygote exited before ready: %v", err)
	case <-time.After(1 * time.Minute):
		select {
		case <-ze.closed:
			ze.mu.Lock()
			err := ze.exitErr
			ze.mu.Unlock()
			return 0, fmt.Errorf("zygote exited before ready: %v", err)
		default:
		}
		return 0, fmt.Errorf("zygote setup timed out")
	}

	reqID, err := randID()
	if err != nil {
		return 0, err
	}
	respCh := make(chan int, 1)
	ze.mu.Lock()
	ze.pending[reqID] = respCh
	zygConn := ze.zygConn
	ze.mu.Unlock()
	if zygConn == nil {
		return 0, fmt.Errorf("zygote connection missing")
	}

	// Send fork request to zygote.
	ze.sendMu.Lock()
	err = writeFrame(zygConn, forkMsg{Type: "fork", ReqID: reqID, Env: uproc.GetEnv(), Args: fp.GetChildArgs()})
	ze.sendMu.Unlock()
	if err != nil {
		return 0, err
	}

	select {
	case hostPid := <-respCh:
		ze.mu.Lock()
		ze.children++
		ze.mu.Unlock()
		return hostPid, nil
	case <-time.After(10 * time.Second):
		return 0, fmt.Errorf("timeout waiting for forked child")
	}
}

func (fm *forkMgr) childDone(zygoteKey string) {
	fm.mu.Lock()
	ze := fm.zygotes[zygoteKey]
	fm.mu.Unlock()
	if ze == nil {
		return
	}

	ze.mu.Lock()
	if ze.children > 0 {
		ze.children--
	}
	ze.lastIdle = time.Now()
	keepAlive := ze.keepAlive
	shouldEvictNow := ze.children == 0 && keepAlive <= 0

	if ze.children == 0 && keepAlive > 0 {
		if ze.evictT != nil {
			ze.evictT.Stop()
		}
		ze.evictT = time.AfterFunc(keepAlive, func() {
			fm.evictIfIdle(zygoteKey)
		})
	}
	ze.mu.Unlock()

	if shouldEvictNow {
		fm.evictIfIdle(zygoteKey)
	}
}

func (fm *forkMgr) evictIfIdle(zygoteKey string) {
	fm.mu.Lock()
	ze := fm.zygotes[zygoteKey]
	fm.mu.Unlock()
	if ze == nil {
		return
	}

	ze.mu.Lock()
	if ze.children != 0 {
		ze.mu.Unlock()
		return
	}
	if ze.keepAlive > 0 && time.Since(ze.lastIdle) < ze.keepAlive {
		ze.mu.Unlock()
		return
	}
	cmd := ze.zygCmd
	ze.mu.Unlock()

	select {
	case <-ze.closed:
		return
	default:
	}

	// Best-effort: if the zygote is still alive, terminate it so that its jail is
	// cleaned up. The jail must outlive all children; eviction is gated on
	// ze.children==0.
	if cmd != nil {
		_ = cmd.Kill()
	}
}

func waitForHostPIDExit(hostPid int) error {
	fd, err := unix.PidfdOpen(hostPid, 0)
	if err != nil {
		return fmt.Errorf("pidfd_open(%d): %w", hostPid, err)
	}
	defer unix.Close(fd)
	_, err = unix.Poll([]unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}, -1)
	if err != nil {
		return fmt.Errorf("pidfd poll(%d): %w", hostPid, err)
	}
	return nil
}
