// Package subproc wraps os/exec for running external binaries.
// It replaces cps/subproc_wrapper.py.
package subproc

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// Result holds the output from a subprocess run.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Run executes a command and returns its combined output.
func Run(ctx context.Context, name string, args ...string) (*Result, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	slog.Debug("subproc.Run", "cmd", name, "args", args)

	err := cmd.Run()
	res := &Result{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			res.ExitCode = exitErr.ExitCode()
			return res, fmt.Errorf("%s exited %d: %s", name, res.ExitCode, strings.TrimSpace(res.Stderr))
		}
		return res, err
	}
	return res, nil
}

// RunWithEnv runs a command with additional environment variables merged into the current env.
func RunWithEnv(ctx context.Context, env map[string]string, name string, args ...string) (*Result, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Merge env
	base := os.Environ()
	for k, v := range env {
		base = append(base, k+"="+v)
	}
	cmd.Env = base

	slog.Debug("subproc.RunWithEnv", "cmd", name, "args", args)

	err := cmd.Run()
	res := &Result{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			res.ExitCode = exitErr.ExitCode()
			return res, fmt.Errorf("%s exited %d: %s", name, res.ExitCode, strings.TrimSpace(res.Stderr))
		}
		return res, err
	}
	return res, nil
}

var progressRe = regexp.MustCompile(`(\d+)%`)

// ParseProgress extracts the last percentage value from text output.
// Returns -1 if none found.
func ParseProgress(output string) float64 {
	matches := progressRe.FindAllStringSubmatch(output, -1)
	if len(matches) == 0 {
		return -1
	}
	last := matches[len(matches)-1][1]
	v, _ := strconv.ParseFloat(last, 64)
	return v / 100.0
}

// LookPath wraps exec.LookPath and returns "" if not found.
func LookPath(binary string) string {
	p, err := exec.LookPath(binary)
	if err != nil {
		return ""
	}
	return p
}
