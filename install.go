package flatpak

import (
	"regexp"
	"fmt"
	"strconv"
	"os/exec"
	"bytes"
	"errors"
)

var regInstall = regexp.MustCompile(`Installing:\s+(\S+)\s+from`)
var regProgress = regexp.MustCompile(`\[(.*)\]\s+(.*)`)
var regSpeed = regexp.MustCompile(`\(([\d.]+) (\w+)/s\)`)

type InstallProgressMonitor struct {
	cb InstallProgressCallback
}

type InstallProgressCallback func(progress float64, status string, speed int64)

func newInstallProgressMonitor(cb InstallProgressCallback) *InstallProgressMonitor {
	return &InstallProgressMonitor{
		cb: cb,
	}
}

func getProgress(bar []byte) (float64, error) {
	var total int
	for _, b := range bar {
		switch b {
		case '#':
			total += 3
		case '=':
			total += 2
		case '-':
			total += 1
		case ' ':
			total += 0
		default:
			return 0, fmt.Errorf("unknown byte %q in progress bar", b)
		}
	}
	return float64(total) / float64(len(bar)*3), nil
}

func getSpeed(status []byte) (int64, error) {
	result := regSpeed.FindSubmatch(status)
	if len(result) > 0 {
		speedStr := string(result[1])
		speedUnit := string(result[2])

		speedNum, err := strconv.ParseFloat(speedStr, 64)
		if err != nil {
			return 0, err
		}
		if speedNum < 0 {
			return 0, errors.New("speed is less then 0")
		}

		unit, err := parseDataUnit(speedUnit)
		if err != nil {
			return 0, err
		}
		return int64(speedNum * unit), nil
	}
	return 0, errors.New("not match speed regexp")
}

func parseDataUnit(unit string) (float64, error) {
	switch unit {
	case "kB":
		return 1000, nil
	case "MB":
		return 1000 * 1000, nil
	case "GB":
		return 1000 * 1000 * 1000, nil
	case "TB":
		return 1000 * 1000 * 1000 * 1000, nil
	case "bytes":
		return 1, nil
	default:
		return 0, fmt.Errorf("unknown speed unit %q", unit)
	}
}

func (w *InstallProgressMonitor) Write(p []byte) (n int, err error) {
	//log.Printf("write %s\n", p)
	result := regInstall.FindSubmatch(p)
	if len(result) > 0 {
		// matched
		ref := result[1]
		fmt.Printf("get installing %s\n", ref)
		goto out
	}

	result = regProgress.FindSubmatch(p)
	if len(result) > 0 {
		// matched
		progressBar := result[1]
		status := result[2]
		progress, err := getProgress(progressBar)
		if err != nil {
			goto out
		}

		speed, _ := getSpeed(status)
		w.cb(progress, string(status), speed)
	}

out:
	return len(p), nil
}

type InstallOptions struct {
	User    bool
	System  bool
	Runtime bool
	App     bool

	NoPull         bool
	NoDeploy       bool
	NoRelated      bool
	NoDeps         bool
	NoStaticDeltas bool

	Bundle bool
	From   bool

	GPGFile string

	AssumeYes bool
}

func (o *InstallOptions) getArgs() (args []string) {
	if o == nil {
		return nil
	}
	if o.User {
		args = append(args, "--user")
	}
	if o.System {
		args = append(args, "--system")
	}
	if o.Runtime {
		args = append(args, "--runtime")
	}
	if o.App {
		args = append(args, "--app")
	}

	if o.NoPull {
		args = append(args, "--no-pull")
	}

	if o.NoDeploy {
		args = append(args, "--no-deploy")
	}

	if o.NoRelated {
		args = append(args, "--no-related")
	}

	if o.NoDeps {
		args = append(args, "--no-deps")
	}

	if o.NoStaticDeltas {
		args = append(args, "--no-static-deltas")
	}

	if o.Bundle {
		args = append(args, "--bundle")
	}

	if o.From {
		args = append(args, "--from")
	}

	if o.GPGFile != "" {
		args = append(args, "--gpg-file="+o.GPGFile)
	}

	if o.AssumeYes {
		args = append(args, "-y")
	}

	return args
}

func Install(location string, refs []string, opts *InstallOptions, cb InstallProgressCallback) error {
	args := []string{"install", location}
	args = append(args, refs...)
	optArgs := opts.getArgs()
	args = append(args, optArgs...)
	fmt.Printf("install args: %#v\n", args)
	cmd := exec.Command(flatpakBin, args...)

	if cb != nil {
		cmd.Stdout = newInstallProgressMonitor(cb)
	}
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	return wrapErrorCmdRun(err, &stderrBuf)
}
