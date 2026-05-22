package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

	// We start with "allow default" (allows read, memory, threads, IPC, etc. to avoid breaking standard environments).
	// Then we deny file-write* globally.
	// Then we whitelist file-write* for the mirrored workspace and macOS system temp directories.
	profile := fmt.Sprintf(`(version 1)
(allow default)
(deny file-write* (subpath "/"))
(allow file-write* (subpath "%s"))
(allow file-write* (subpath "/private/tmp"))
(allow file-write* (subpath "/private/var"))
(allow file-write* (subpath "/var/folders"))
`, absWorkspace)

	if !allowNetwork {
		profile += "(deny network-outbound)\n"
	}

	return profile
}
