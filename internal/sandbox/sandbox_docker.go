package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
)

type DockerSandboxDriver struct {
	DefaultImage string
}

func NewDockerSandboxDriver(defaultImage string) *DockerSandboxDriver {
	if defaultImage == "" {
		defaultImage = "alpine:latest" // Lightweight default
	}
	return &DockerSandboxDriver{DefaultImage: defaultImage}
}

func (d *DockerSandboxDriver) Run(ctx context.Context, cmdStr string, opts RunOptions) (*RunResult, error) {
	// 1. Ensure docker is installed and running
	if err := d.checkDocker(ctx); err != nil {
		return nil, fmt.Errorf("docker is not available or running: %w", err)
	}

	// 2. Prepare docker run command arguments
	// docker run --rm -v <workspace>:/workspace -w /workspace <network-flag> <env-flags> <image> sh -c "<cmd>"
	absWorkspace, err := filepath.Abs(opts.Dir)
	if err != nil {
		absWorkspace = opts.Dir
	}

	args := []string{
		"run",
		"--rm",
		"--user", "1000:1000",                       // Run as non-root
		"--cap-drop", "ALL",                          // Drop all capabilities
		"--security-opt", "no-new-privileges",        // Prevent privilege escalation
		"--read-only",                                // Read-only root filesystem
		"--tmpfs", "/tmp:rw,noexec,nosuid,size=100m", // Writable /tmp with security options
		"--memory", "512m",                           // Memory limit
		"--pids-limit", "100",                        // Process limit
		"-v", fmt.Sprintf("%s:/workspace", absWorkspace),
		"-w", "/workspace",
	}

	// Disable network if specified
	if !opts.Network {
		args = append(args, "--network", "none")
	}

	// Inject environment variables
	for _, env := range opts.Env {
		args = append(args, "-e", env)
	}

	// Add image and command
	args = append(args, d.DefaultImage, "sh", "-c", cmdStr)

	runCmd := exec.CommandContext(ctx, "docker", args...)

	var stdoutBuf, stderrBuf bytes.Buffer
	runCmd.Stdout = &stdoutBuf
	runCmd.Stderr = &stderrBuf

	// 3. Run container
	runErr := runCmd.Run()

	res := &RunResult{
		Stdout: stdoutBuf.String(),
		Stderr: stderrBuf.String(),
	}

	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			res.ExitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("failed to execute command in docker container: %w", runErr)
		}
	} else {
		res.ExitCode = 0
	}

	return res, nil
}

func (d *DockerSandboxDriver) checkDocker(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "docker", "info")
	return cmd.Run()
}
