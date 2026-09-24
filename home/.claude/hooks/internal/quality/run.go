package quality

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"time"
)

// commandTimeout bounds each step. The hook entry's own timeout in
// settings.json must cover every step of one file run back to back.
const commandTimeout = 60 * time.Second

// Run runs plan's steps in order and returns every notice worth surfacing:
// the plan's own, then each step's.
func Run(ctx context.Context, plan Plan) []Notice {
	notices := slices.Clone(plan.Notices)
	for _, s := range plan.Steps {
		ok, output := runStep(ctx, s)
		if n := s.interpret(ok, output); n != nil {
			notices = append(notices, *n)
		}
	}
	return notices
}

// runStep returns whether s succeeded and its combined output. A command that
// could not run at all reports why in place of output, so the step's notice
// says what went wrong.
func runStep(ctx context.Context, s Step) (ok bool, output string) {
	ctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()

	c := exec.CommandContext(ctx, s.Argv[0], s.Argv[1:]...)
	c.Dir = s.Dir
	out, err := c.CombinedOutput()
	output = strings.TrimSpace(string(out))

	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return false, fmt.Sprintf("timed out after %s in %s", commandTimeout, s.Dir)
	}
	if err == nil {
		return true, output
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return false, output
	}
	if errors.Is(err, exec.ErrNotFound) {
		return false, fmt.Sprintf("Command not found: %s. Install it or put it on PATH.", s.Argv[0])
	}
	return false, fmt.Sprintf("could not run %s in %s: %v", s.Argv[0], s.Dir, err)
}
