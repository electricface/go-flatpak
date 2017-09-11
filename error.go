package flatpak

import (
	"bytes"
	"os/exec"
)

type FlatpakError struct {
	desc   string
	origin *exec.ExitError
}

func (err *FlatpakError) Error() string {
	if err.desc != "" {
		return "flatpak:" + err.desc
	}
	return err.origin.Error()
}

func wrapError(err *exec.ExitError, stderr []byte) error {
	var desc []byte
	lines := bytes.Split(stderr, []byte{'\n'})

	for i := len(lines) - 1; i >= 0; i-- {
		line := bytes.TrimSpace(lines[i])
		if len(line) > 0 {
			desc = bytes.TrimPrefix(line, []byte("error:"))
			break
		}
	}

	return &FlatpakError{
		desc:   string(desc),
		origin: err,
	}
}

func wrapErrorCmdRun(err error, stderrBuf *bytes.Buffer) error {
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		return err
	}
	return wrapError(exitErr, stderrBuf.Bytes())
}

func wrapErrorCmdOutput(err error) error {
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		return err
	}
	return wrapError(exitErr, exitErr.Stderr)
}
