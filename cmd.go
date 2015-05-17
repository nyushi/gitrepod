package gitrepod

import (
	"fmt"
	"io"
	"os/exec"
	"sync"
)

func doCommand(cmd *exec.Cmd, stdin io.Reader, stdout, stderr io.Writer) error {
	cmdStdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	cmdStdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmdStderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	wg := &sync.WaitGroup{}
	go func() {
		io.Copy(cmdStdin, stdin)
		cmdStdin.Close()
	}()
	wg.Add(2)
	go func() {
		io.Copy(stdout, cmdStdout)
		wg.Done()
	}()
	go func() {
		io.Copy(stderr, cmdStderr)
		wg.Done()
	}()

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("error at start cmd: %s", err)
	}
	wg.Wait()
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("error at wait cmd: %s", err)
	}

	return nil
}
