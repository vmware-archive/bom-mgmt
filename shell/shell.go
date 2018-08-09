package shell

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
)

func RunCommandIgnoreError(command string) {
	runCommand(command, true)
}

func RunCommand(command string) {
	runCommand(command, false)
}

func runCommand(command string, ignoreError bool) {
	fmt.Println("$: " + command)
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd := exec.Command("sh", "-c", command)

	stdoutIn, _ := cmd.StdoutPipe()
	stderrIn, _ := cmd.StderrPipe()

	var errStdout, errStderr error
	stdout := io.MultiWriter(os.Stdout, &stdoutBuf)
	stderr := io.MultiWriter(os.Stderr, &stderrBuf)
	err := cmd.Start()
	if err != nil {
		log.Fatalf("cmd.Start() failed with '%s'\n", err)
	}

	go func() {
		_, errStdout = io.Copy(stdout, stdoutIn)
	}()

	go func() {
		_, errStderr = io.Copy(stderr, stderrIn)
	}()

	err = cmd.Wait()
	if err != nil && !ignoreError {
		log.Fatalf("cmd.Run() failed with %s\n", err)
	}
}
