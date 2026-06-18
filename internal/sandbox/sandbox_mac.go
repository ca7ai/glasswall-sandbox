package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type MacSandboxDriver struct{}

func NewMacSandboxDriver() *MacSandboxDriver {
	return &MacSandboxDriver{}
}

func (d *MacSandboxDriver) Run(ctx context.Context, cmdStr string, opts RunOptions) (*RunResult, error) {
	// 1. Construct the sandbox profile
	profile := d.generateProfile(opts.Dir, opts.Network)

	// 2. Set up the cmd execution: sandbox-exec -p "<profile>" sh -c "<cmdStr>"
	// We run it under sh -c so that users can run compound shells, pipes, or direct scripts.
	runCmd := exec.CommandContext(ctx, "sandbox-exec", "-p", profile, "sh", "-c", cmdStr)
	runCmd.Dir = opts.Dir

	// Setup environment variables if provided
	if len(opts.Env) > 0 {
		runCmd.Env = append(os.Environ(), opts.Env...)
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	runCmd.Stdout = &stdoutBuf
	runCmd.Stderr = &stderrBuf

	// 3. Execute
	err := runCmd.Run()

	res := &RunResult{
		Stdout: stdoutBuf.String(),
		Stderr: stderrBuf.String(),
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			res.ExitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("command execution failed: %w", err)
		}
	} else {
		res.ExitCode = 0
	}

	return res, nil
}

func (d *MacSandboxDriver) generateProfile(workspacePath string, allowNetwork bool) string {
	// Clean workspacePath to be absolute
	absWorkspace, err := filepath.Abs(workspacePath)
	if err != nil {
		absWorkspace = workspacePath
	}

	// Escape the workspace path for Seatbelt DSL to prevent injection
	escapedWorkspace := escapeSeatbeltPath(absWorkspace)

	// We start with "allow default" (allows read, memory, threads, IPC, etc. to avoid breaking standard environments).
	// Then we deny file-write* globally.
	// Then we whitelist file-write* for the mirrored workspace only.
	profile := fmt.Sprintf(`(version 1)
(allow default)
(deny file-write* (subpath "/"))
(allow file-write* (subpath "%s"))
`, escapedWorkspace)

	if !allowNetwork {
		profile += "(deny network-outbound)\n"
	}

	return profile
}

// escapeSeatbeltPath escapes special characters in paths that could break Seatbelt DSL syntax
func escapeSeatbeltPath(path string) string {
	// Seatbelt uses Scheme-like syntax where quotes, parens, and backslashes are special
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		`(`, `\(`,
		`)`, `\)`,
	)
	return replacer.Replace(path)
}
