package flatpak

import (
	"bytes"
	"os/exec"
)

type FlatpakError struct {
	desc       string
	origin     *exec.ExitError
	usageShown bool
}

func (err *FlatpakError) Error() string {
	if err.desc != "" {
		if err.usageShown {
			return "flatpak argument error:" + err.desc
		}
		return "flatpak:" + err.desc
	}
	return err.origin.Error()
}

func wrapError(err *exec.ExitError, stderr []byte) error {
	var lastLine []byte
	var usageShown bool
	lines := bytes.Split(stderr, []byte{'\n'})
	if len(lines) > 0 && bytes.HasPrefix(lines[0], []byte("Usage:")) {
		// first line is Usage:
		usageShown = true
	}

	for _, line := range lines {
		if len(line) > 0 {
			lastLine = line
		}
	}

	lastLine = bytes.TrimPrefix(lastLine, []byte("error:"))

	return &FlatpakError{
		desc:       string(lastLine),
		origin:     err,
		usageShown: usageShown,
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
