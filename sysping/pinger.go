// copy from https://github.com/sggms/go-pingparse, only for linux ping
package sysping

import (
	"bytes"
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

// Ping will ping the specified IPv4 address wit the provided timeout, interval and size settings .
func Ping(ipV4Address string, interval, timeout int, count int) (*PingOutput, error) {
	var (
		output, errorOutput bytes.Buffer
		exitCode            int
	)
	var pingArgs []string
	if runtime.GOOS == "darwin" {
		pingArgs = []string{"-n", "-i", strconv.Itoa(interval), "-c", strconv.Itoa(count), ipV4Address}
	} else {
		pingArgs = []string{"-n", "-i", strconv.Itoa(interval), "-c", strconv.Itoa(count), ipV4Address}
	}
	cmd := exec.Command("ping", pingArgs...)
	cmd.Stdout = &output
	cmd.Stderr = &errorOutput
	fmt.Println(cmd.String())
	err := cmd.Run()
	if err == nil {
		ws := cmd.ProcessState.Sys().(syscall.WaitStatus)
		exitCode = ws.ExitStatus()
	} else {
		exitCode, err = parseExitCode(err)
		if err != nil {
			return nil, err
		}
	}
	fmt.Println(output.String())
	// try to parse output also in case of failure
	po, err := Parse(output.String())
	if err == nil {
		return po, nil
	}

	// in case of error, use also the execution context errors (if any)
	return nil, fmt.Errorf("command: ping %s\nexit code: %d\nparse error: %v\nstdout:\n%s\nstderr:\n%s", strings.Join(pingArgs, " "), exitCode, err, output.String(), errorOutput.String())
}

func parseExitCode(err error) (int, error) {
	// try to get the exit code
	if exitError, ok := err.(*exec.ExitError); ok {
		ws := exitError.Sys().(syscall.WaitStatus)
		return ws.ExitStatus(), nil
	}

	// This will happen (in OSX) if `name` is not available in $PATH,
	// in this situation, exit code could not be get, and stderr will be
	// empty string very likely, so we use the default fail code, and format err
	// to string and set to stderr
	return 0, fmt.Errorf("could not get exit code for failed program: %v", err)
}
