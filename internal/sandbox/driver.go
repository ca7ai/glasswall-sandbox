package sandbox

import (
	"context"
	"time"
)

type RunOptions struct {
	Dir     string        // Working directory to run the command in (typically the mirrored directory)
	Env     []string      // Environment variables to inject
	Network bool          // Whether outbound network access is allowed
	Timeout time.Duration // Command execution timeout
}

type RunResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

type SandboxDriver interface {
	Run(ctx context.Context, cmd string, opts RunOptions) (*RunResult, error)
}
