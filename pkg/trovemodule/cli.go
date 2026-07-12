package trovemodule

import (
	"context"

	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
)

// CLIHandler handles CLI command invocations for a module.
type CLIHandler interface {
	RunCommand(ctx context.Context, command string, args []string) (stdout, stderr []byte, exitCode int, err error)
}

// RunCommandRPC adapts CLIHandler to the gRPC request/response types.
func RunCommandRPC(ctx context.Context, h CLIHandler, req *troverpc.CLICommandRequest) (*troverpc.CLICommandResponse, error) {
	stdout, stderr, exitCode, err := h.RunCommand(ctx, req.GetCommand(), req.GetArgs())
	if err != nil {
		return nil, err
	}
	return &troverpc.CLICommandResponse{
		ExitCode: int32(exitCode), //nolint:gosec // G115: CLI exit codes are small
		Stdout:   stdout,
		Stderr:   stderr,
	}, nil
}
