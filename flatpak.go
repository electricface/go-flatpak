package flatpak

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strings"
)

var regSpaces = regexp.MustCompile(`\s+`)

const flatpakBin = "flatpak"

func SupportedArches() ([]string, error) {
	cmd := exec.Command(flatpakBin, "--supported-arches")
	out, err := cmd.Output()
	if err != nil {
		return nil, wrapErrorCmdOutput(err)
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
		return "", wrapErrorCmdOutput(err)
	}
	out = bytes.TrimSpace(out)
	out = bytes.TrimPrefix(out, []byte("Flatpak "))
	return string(out), err
}

func GlDrivers() ([]string, error) {
	cmd := exec.Command(flatpakBin, "--gl-drivers")
	out, err := cmd.Output()
	if err != nil {
		return nil, wrapErrorCmdOutput(err)
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
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf
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
		return nil, wrapErrorCmdRun(err, &stderrBuf)
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
		return nil, wrapErrorCmdOutput(err)
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
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf
	err := cmd.Run()
	return wrapErrorCmdRun(err, &stderrBuf)
}
