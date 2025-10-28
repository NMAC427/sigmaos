package scontainer_test

import (
	"fmt"
	"sigmaos/proc"
	"sigmaos/test"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func runBasicPythonTest(ts *test.Tstate, spawn_type string, proc *proc.Proc) {
	start := time.Now()
	err := ts.Spawn(proc)
	assert.Nil(ts.T, err)
	duration := time.Since(start)
	err = ts.WaitStart(proc.GetPid())
	assert.Nil(ts.T, err, "Error waitstart: %v", err)
	duration2 := time.Since(start)
	status, err := ts.WaitExit(proc.GetPid())
	assert.Nil(ts.T, err)
	assert.True(ts.T, status.IsStatusOK(), "Bad exit status: %v", status)
	duration3 := time.Since(start)
	fmt.Printf("%s spawn %v, start %v, exit %v\n", spawn_type, duration, duration2, duration3)
}

func runBasicPythonTestWithoutCheckingExitCode(ts *test.Tstate, spawn_type string, proc *proc.Proc) {
	start := time.Now()
	err := ts.Spawn(proc)
	assert.Nil(ts.T, err)
	duration := time.Since(start)
	err = ts.WaitStart(proc.GetPid())
	assert.Nil(ts.T, err, "Error waitstart: %v", err)
	duration2 := time.Since(start)
	_, _ = ts.WaitExit(proc.GetPid())
	duration3 := time.Since(start)
	fmt.Printf("%s spawn %v, start %v, exit %v\n", spawn_type, duration, duration2, duration3)
}

func TestPythonStartup(t *testing.T) {
	ts, _ := test.NewTstateAll(t)
	defer ts.Shutdown()

	p := proc.NewPythonProc(proc.Python311, []string{"-c", "exit(1)"})
	runBasicPythonTestWithoutCheckingExitCode(ts, "cold", p)

	p = proc.NewPythonProc(proc.Python311, []string{"-c", "exit(1)"})
	runBasicPythonTestWithoutCheckingExitCode(ts, "warm", p)
}

func TestPythonSplibImport(t *testing.T) {
	ts, _ := test.NewTstateAll(t)
	defer ts.Shutdown()

	// Launch, connect to sigmaos proxy, signal start & exit
	p := proc.NewPythonProc(proc.Python311, []string{"hello/main.py"})
	runBasicPythonTest(ts, "cold", p)

	p = proc.NewPythonProc(proc.Python311, []string{"hello/main.py"})
	runBasicPythonTest(ts, "warm", p)
}

func TestPythonEnvInfo(t *testing.T) {
	ts, _ := test.NewTstateAll(t)
	defer ts.Shutdown()

	p := proc.NewPythonProc(proc.Python311, []string{"hello/env_info.py"})
	runBasicPythonTest(ts, "cold", p)
}

func TestPythonBasicImport(t *testing.T) {
	ts, _ := test.NewTstateAll(t)
	defer ts.Shutdown()

	p := proc.NewPythonProc(proc.Python311, []string{"basic_import/main.py"})
	runBasicPythonTest(ts, "cold", p)
}

func TestPythonNeighborImport(t *testing.T) {
	ts, _ := test.NewTstateAll(t)
	defer ts.Shutdown()

	p := proc.NewPythonProc(proc.Python311, []string{"neighbor_import/main.py"})
	runBasicPythonTest(ts, "cold", p)
}

func TestPythonNumpyImport(t *testing.T) {
	ts, _ := test.NewTstateAll(t)
	defer ts.Shutdown()

	p := proc.NewPythonProc(proc.Python311, []string{"numpy_import/main.py"})
	runBasicPythonTest(ts, "cold", p)

	p2 := proc.NewPythonProc(proc.Python311, []string{"numpy_import/main.py"})
	runBasicPythonTest(ts, "warm", p2)
}

func TestImageProcessing(t *testing.T) {
	ts, _ := test.NewTstateAll(t)
	defer ts.Shutdown()

	p := proc.NewPythonProc(proc.Python311, []string{"imgprocessing/main.py"})
	runBasicPythonTest(ts, "cold", p)
}

func TestPythonReverseShell(t *testing.T) {
	ts, _ := test.NewTstateAll(t)
	defer ts.Shutdown()

	fmt.Printf("To connect to the python reverse shell, run:\n\n  nc -lvnp 4445\n\n")
	p := proc.NewPythonProc(proc.Python311, []string{"_reverse_shell/main.py"})
	runBasicPythonTestWithoutCheckingExitCode(ts, "cold", p)
}

// SigmaOS API Tests

func TestPythonStat(t *testing.T) {
	ts, _ := test.NewTstateAll(t)
	defer ts.Shutdown()

	p := proc.NewPythonProc(proc.Python311, []string{"stat_test/main.py"})
	runBasicPythonTest(ts, "cold", p)
}

func TestPythonFiles(t *testing.T) {
	ts, _ := test.NewTstateAll(t)
	defer ts.Shutdown()

	p := proc.NewPythonProc(proc.Python311, []string{"file_test/main.py"})
	runBasicPythonTest(ts, "cold", p)
}
