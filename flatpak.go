package flatpak

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

var regSpaces = regexp.MustCompile(`\s+`)

const flatpakBin = "flatpak"

func SupportedArches() ([]string, error) {
	cmd := exec.Command(flatpakBin, "--supported-arches")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	parts := bytes.Split(out, []byte{'\n'})
	arches := make([]string, 0, len(parts))
	for _, p := range parts {
		if len(p) != 0 {
			arches = append(arches, string(p))
		}
	}
	return arches, nil
}

func Version() (string, error) {
	cmd := exec.Command(flatpakBin, "--version")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	out = bytes.TrimSpace(out)
	out = bytes.TrimPrefix(out, []byte("Flatpak "))
	return string(out), err
}

func GlDrivers() ([]string, error) {
	cmd := exec.Command(flatpakBin, "--gl-drivers")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	parts := bytes.Split(out, []byte{'\n'})
	drivers := make([]string, 0, len(parts))
	for _, p := range parts {
		if len(p) != 0 {
			drivers = append(drivers, string(p))
		}
	}
	return drivers, nil
}

type Ref struct {
	Name   string
	Arch   string
	Branch string
}

func (r *Ref) String() string {
	return fmt.Sprintf("%s/%s/%s", r.Name, r.Arch, r.Branch)
}

func parseRef(ref string) (Ref, error) {
	parts := strings.Split(ref, "/")
	if len(parts) != 3 {
		return Ref{}, errors.New("length of parts is not 3")
	}
	return Ref{
		Name:   parts[0],
		Arch:   parts[1],
		Branch: parts[2],
	}, nil
}

type ListResult struct {
	Ref           Ref
	Origin        string
	ActiveCommit  string
	LatestCommit  string
	InstalledSize string
	Options       []string
}

func parseList(line string) (*ListResult, error) {
	parts := regSpaces.Split(line, -1)
	//fmt.Printf("parts: %#v\n", parts)

	if len(parts) < 7 {
		return nil, errors.New("length of parts is less then 7")
	}

	ref, err := parseRef(parts[0])
	if err != nil {
		return nil, err
	}
	options := strings.Split(parts[6], ",")

	result := ListResult{
		Ref:           ref,
		Origin:        parts[1],
		ActiveCommit:  parts[2],
		LatestCommit:  parts[3],
		InstalledSize: parts[4] + " " + parts[5],
		Options:       options, // parts[6]
	}
	//fmt.Printf("result: %#v\n", result)
	return &result, nil
}

type ListOptions struct {
	User    bool
	System  bool
	Runtime bool
	App     bool
	Arch    string
	All     bool
}

func (o *ListOptions) GetArgs() (args []string) {
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
	if o.Arch != "" {
		args = append(args, "--arch="+o.Arch)
	}
	if o.All {
		args = append(args, "--all")
	}
	return args
}

func List(opts *ListOptions) (results []*ListResult, err error) {
	args := []string{"list", "-d"}
	optArgs := opts.GetArgs()
	args = append(args, optArgs...)

	cmd := exec.Command(flatpakBin, args...)
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	defer stdoutPipe.Close()

	err = cmd.Start()
	if err != nil {
		return nil, err
	}
	stdoutReader := bufio.NewReader(stdoutPipe)
	for {
		bytes, err := stdoutReader.ReadBytes('\n')
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}

		result, err := parseList(string(bytes))
		if err != nil {
			return nil, err
		}

		results = append(results, result)
	}
	err = cmd.Wait()
	if err != nil {
		return nil, err
	}

	return results, nil
}

type InfoResult struct {
	Ref           string
	ID            string
	Arch          string
	Branch        string
	Origin        string
	Commit        string
	Location      string
	InstalledSize string
	Runtime       string
}

type InfoOptions struct {
	User   bool
	System bool
}

func (o *InfoOptions) GetArgs() (args []string) {
	if o == nil {
		return nil
	}
	if o.User {
		args = append(args, "--user")
	}
	if o.System {
		args = append(args, "--system")
	}
	return args
}

func Info(ref Ref, opts *InfoOptions) (*InfoResult, error) {
	args := []string{"info", ref.String()}
	optArgs := opts.GetArgs()
	args = append(args, optArgs...)

	cmd := exec.Command(flatpakBin, args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var result InfoResult
	lines := bytes.Split(out, []byte{'\n'})
	for _, line := range lines {
		parts := bytes.SplitN(line, []byte{':', ' '}, 2)
		if len(parts) == 2 {
			key := string(parts[0])
			val := string(parts[1])

			switch key {
			case "Ref":
				result.Ref = val
			case "ID":
				result.ID = val
			case "Arch":
				result.Arch = val
			case "Branch":
				result.Branch = val
			case "Origin":
				result.Origin = val
			case "Commit":
				result.Commit = val
			case "Location":
				result.Location = val
			case "Installed size":
				result.InstalledSize = val
			case "Runtime":
				result.Runtime = val
			}
		}
	}

	return &result, nil
}

type UninstallOptions struct {
	Arch    string
	User    bool
	System  bool
	Runtime bool
	App     bool

	KeepRef     bool
	NoRelated   bool
	ForceRemove bool
}

func (o *UninstallOptions) getArgs() (args []string) {
	if o == nil {
		return nil
	}
	if o.Arch != "" {
		args = append(args, "--arch"+o.Arch)
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

	if o.KeepRef {
		args = append(args, "--keep-ref")
	}

	if o.NoRelated {
		args = append(args, "--no-related")
	}

	if o.ForceRemove {
		args = append(args, "--force-remove")
	}

	return args
}

func Uninstall(refs []string, opts *UninstallOptions) error {
	args := []string{"uninstall"}
	args = append(args, refs...)
	optArgs := opts.getArgs()
	args = append(args, optArgs...)

	cmd := exec.Command(flatpakBin, args...)
	return cmd.Run()
}

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

	return cmd.Run()
}
