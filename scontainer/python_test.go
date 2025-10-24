package scontainer_test

import (
	"fmt"
	"os"
	"sigmaos/proc"
	"sigmaos/test"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const bucket = "sigmaos-ncam"

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

	p := proc.NewPythonProc([]string{"-c", "exit(1)"}, bucket)
	runBasicPythonTestWithoutCheckingExitCode(ts, "cold", p)

	p = proc.NewPythonProc([]string{"-c", "exit(1)"}, bucket)
	runBasicPythonTestWithoutCheckingExitCode(ts, "warm", p)
}

func TestPythonSplibImport(t *testing.T) {
	ts, _ := test.NewTstateAll(t)
	defer ts.Shutdown()

	// Launch, connect to sigmaos proxy, signal start & exit
	p := proc.NewPythonProc([]string{"hello/main.py"}, bucket)
	runBasicPythonTest(ts, "cold", p)

	p = proc.NewPythonProc([]string{"hello/main.py"}, bucket)
	runBasicPythonTest(ts, "warm", p)
}

func TestPythonEnvInfo(t *testing.T) {
	ts, _ := test.NewTstateAll(t)
	defer ts.Shutdown()

	p := proc.NewPythonProc([]string{"hello/env_info.py"}, bucket)
	runBasicPythonTest(ts, "cold", p)
}

func TestPythonBasicImport(t *testing.T) {
	ts, _ := test.NewTstateAll(t)
	defer ts.Shutdown()

	p := proc.NewPythonProc([]string{"basic_import/main.py"}, bucket)
	runBasicPythonTest(ts, "cold", p)
}

func TestPythonAWSImport(t *testing.T) {
	ts, _ := test.NewTstateAll(t)
	defer ts.Shutdown()

	p := proc.NewPythonProc([]string{"aws_import/main.py"}, bucket)
	runBasicPythonTest(ts, "cold", p)
}

func TestPythonNeighborImport(t *testing.T) {
	ts, _ := test.NewTstateAll(t)
	defer ts.Shutdown()

	p := proc.NewPythonProc([]string{"neighbor_import/main.py"}, bucket)
	runBasicPythonTest(ts, "cold", p)
}

func TestPythonNumpyImport(t *testing.T) {
	ts, _ := test.NewTstateAll(t)
	defer ts.Shutdown()

	p := proc.NewPythonProc([]string{"numpy_import/main.py"}, bucket)
	runBasicPythonTest(ts, "cold", p)

	p2 := proc.NewPythonProc([]string{"numpy_import/main.py"}, bucket)
	runBasicPythonTest(ts, "warm", p2)
}

func TestImageProcessing(t *testing.T) {
	ts, _ := test.NewTstateAll(t)
	defer ts.Shutdown()

	p := proc.NewPythonProc([]string{"imgprocessing/main.py"}, bucket)
	runBasicPythonTest(ts, "cold", p)
}

func TestPythonChecksumVerification(t *testing.T) {
	ts, _ := test.NewTstateAll(t)
	defer ts.Shutdown()

	fmt.Printf("Starting 1st proc...\n")
	p := proc.NewPythonProc([]string{"aws_import/main.py"}, bucket)
	runBasicPythonTest(ts, "cold", p)

	checksumPath := "/tmp/python/Lib/dummy_package-sigmaos-checksum"
	_, err := os.Stat(checksumPath)
	assert.Nil(t, err)

	fmt.Printf("Starting 2nd proc (cached lib)...\n")
	p2 := proc.NewPythonProc([]string{"aws_import/main.py"}, bucket)
	runBasicPythonTest(ts, "warm", p2)

	_, err = os.Stat(checksumPath)
	assert.Nil(t, err)
	err = os.Remove(checksumPath)
	assert.Nil(t, err)
	_, err = os.Stat(checksumPath)
	assert.NotNil(t, err)

	fmt.Printf("Starting 3rd proc (invalid cache)...\n")
	p3 := proc.NewPythonProc([]string{"aws_import/main.py"}, bucket)
	runBasicPythonTest(ts, "warm", p3)
}

func TestPythonReverseShell(t *testing.T) {
	ts, _ := test.NewTstateAll(t)
	defer ts.Shutdown()

	fmt.Printf("To connect to the python reverse shell, run:\n\n  nc -lvnp 4445\n\n")
	p := proc.NewPythonProc([]string{"_reverse_shell/main.py"}, bucket)
	runBasicPythonTest(ts, "cold", p)
}

// SigmaOS API Tests

func TestPythonStat(t *testing.T) {
	ts, _ := test.NewTstateAll(t)
	defer ts.Shutdown()

	p := proc.NewPythonProc([]string{"stat_test/main.py"}, bucket)
	runBasicPythonTest(ts, "cold", p)
}

func TestPythonFiles(t *testing.T) {
	ts, _ := test.NewTstateAll(t)
	defer ts.Shutdown()

	p := proc.NewPythonProc([]string{"file_test/main.py"}, bucket)
	runBasicPythonTest(ts, "cold", p)
}
