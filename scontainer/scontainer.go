// This package provides StartSigmaContainer to run a proc inside a
// sigma container.
package scontainer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sched/msched/proc/srv/binsrv"
	"sigmaos/scontainer/python"
	sp "sigmaos/sigmap"
	"sigmaos/util/perf"
)

type uprocCmd struct {
	uproc    *proc.Proc
	cmd      *exec.Cmd
	jailPath string
}

func (upc *uprocCmd) Wait() error {
	return upc.cmd.Wait()
}

func (upc *uprocCmd) Pid() int {
	return upc.cmd.Process.Pid
}

// Contain user procs using uproc-trampoline trampoline
func StartSigmaContainer(uproc *proc.Proc, dialproxy bool) (*uprocCmd, error) {
	db.DPrintf(db.CONTAINER, "RunUProc dialproxy %v %v env %v\n", dialproxy, uproc, os.Environ())

	uprocCmd := &uprocCmd{uproc: uproc, cmd: nil, jailPath: jailPath(uproc.GetPid())}

	straceProcs := proc.GetLabels(uproc.GetProcEnv().GetStrace())
	valgrindProcs := proc.GetLabels(uproc.GetProcEnv().GetValgrind())

	pn := binsrv.BinPath(uproc.GetVersionedProgram())
	pythonVersion := python.IsSupportedPythonVersion(uproc.GetProgram())
	isPythonProc := pythonVersion != nil
	if isPythonProc {
		pythonPath := pythonVersion.PythonPath

		// uproc-trampoline will mount the correct python interpreter files
		// from /home/sigmaos/bin/kernel/<python-version> to the sigma container
		// python dir /tmp/python/python.
		pn = "/tmp/python/python/python"

		os.MkdirAll(filepath.Join(uprocCmd.jailPath, "tmp/python"), 0777)

		if pythonFile, argIndex, err := python.GetPythonFileArg(uproc.Args); err == nil {
			db.DPrintf(db.CONTAINER, "pythonFile %v\n", pythonFile)

			// We need to prefix the python file path with where the python
			// interpreter can retrieve it inside the jail.
			uproc.Args[argIndex] = filepath.Join("/tmp/python/pyproc", pythonFile)

			// Set up python environment based on pylock file (if present)
			if pylockPath, err := python.GetPylockPath("/home/sigmaos/bin/kernel/pyproc", pythonFile); err == nil {
				db.DPrintf(db.CONTAINER, "setting up python site-packages from %v", pylockPath)
				sitePackagesDir, err := python.SetupSitePackages(pyEnvPath(uproc.GetPid()), pythonVersion, pylockPath)
				if err != nil {
					err = fmt.Errorf("setting up python site-packages failed: %w", err)
					db.DPrintf(db.CONTAINER, "%v", err)
					return nil, err
				}

				pythonPath = pythonPath + ":" + strings.TrimPrefix(sitePackagesDir, uprocCmd.jailPath)
			} else {
				db.DPrintf(db.CONTAINER, "No pylock.toml file found\n")
			}
		} else {
			db.DPrintf(db.CONTAINER, "No python file argument found\n")
		}

		db.DPrintf(db.CONTAINER, "PYTHONPATH: %v\n", pythonPath)
		uproc.AppendEnv("PYTHONPATH", pythonPath)
		uproc.AppendEnv("SIGMA_PYTHON_VERSION", pythonVersion.Version)
	}

	// Optionally strace the proc
	if straceProcs[uproc.GetProgram()] {
		args := []string{"--absolute-timestamps", "--absolute-timestamps=precision:us", "--syscall-times=us", "-D", "-f", "uproc-trampoline", uproc.GetPid().String(), pn, strconv.FormatBool(dialproxy)}
		if strings.Contains(uproc.GetProgram(), "cpp") || isPythonProc {
			// Don't catch SIGSEGV for C++ programs, as this can lead to an infinite
			// strace output loop.
			args = append([]string{"--signal=!SIGSEGV"}, args...)
		}
		args = append(args, uproc.Args...)
		uprocCmd.cmd = exec.Command("strace", args...)
	} else if valgrindProcs[uproc.GetProgram()] {
		uprocCmd.cmd = exec.Command("valgrind", append([]string{"--trace-children=yes", "uproc-trampoline", uproc.GetPid().String(), pn, strconv.FormatBool(dialproxy)}, uproc.Args...)...)
	} else {
		uprocCmd.cmd = exec.Command("uproc-trampoline", append([]string{uproc.GetPid().String(), pn, strconv.FormatBool(dialproxy)}, uproc.Args...)...)
	}
	uproc.AppendEnv("PATH", "/bin:/bin2:/usr/bin:/home/sigmaos/bin/kernel")
	uproc.AppendEnv("SIGMA_EXEC_TIME", strconv.FormatInt(time.Now().UnixMicro(), 10))
	b, err := time.Now().MarshalText()
	if err != nil {
		db.DFatalf("Error marshal timestamp pb: %v", err)
	}
	uproc.AppendEnv("SIGMA_EXEC_TIME_PB", string(b))
	uproc.AppendEnv("SIGMA_SPAWN_TIME", strconv.FormatInt(uproc.GetSpawnTime().UnixMicro(), 10))
	uproc.AppendEnv(proc.SIGMAPERF, uproc.GetProcEnv().GetPerf())
	uproc.AppendEnv("RUST_BACKTRACE", "full")
	uprocCmd.cmd.Env = uproc.GetEnv()

	uprocCmd.cmd.Stdout = os.Stdout
	uprocCmd.cmd.Stderr = os.Stderr

	// Set up new namespaces
	uprocCmd.cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS |
			syscall.CLONE_NEWPID |
			syscall.CLONE_NEWNS,
	}
	db.DPrintf(db.CONTAINER, "exec cmd %v", uprocCmd.cmd)

	s := time.Now()
	if err := uprocCmd.cmd.Start(); err != nil {
		db.DPrintf(db.CONTAINER, "Error start %v %v", uprocCmd.cmd, err)
		CleanupUProc(uprocCmd)
		return nil, err
	}
	perf.LogSpawnLatency("StartSigmaContainer cmd.Start", uproc.GetPid(), uproc.GetSpawnTime(), s)
	return uprocCmd, nil
}

func CleanupUProc(uprocCmd *uprocCmd) {
	pid := uprocCmd.uproc.GetPid()
	python.CleanSitePackages(pyEnvPath(pid))
	if err := os.RemoveAll(uprocCmd.jailPath); err != nil {
		db.DPrintf(db.ALWAYS, "Error cleanupJail: %v", err)
	}
}

func jailPath(pid sp.Tpid) string {
	return filepath.Join(sp.SIGMAHOME, "jail", pid.String())
}

func pyEnvPath(pid sp.Tpid) string {
	return filepath.Join(jailPath(pid), "python", "env")
}
