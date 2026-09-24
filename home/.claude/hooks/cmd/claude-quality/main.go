// Command claude-quality is the PostToolUse formatter and linter for Claude
// Code. It reads one Write/Edit payload on stdin, runs the formatters and
// linters the edited file's project uses (see internal/quality), and reports
// what they could not fix.
//
// It never blocks: the edit has already happened, and a missing or broken tool
// is a notice, not a reason to stop. For PostToolUse, plain stdout only reaches
// the debug log, so findings travel as JSON: systemMessage is shown to the user
// and hookSpecificOutput.additionalContext is added beside the tool result for
// Claude. See https://code.claude.com/docs/en/hooks.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"claude-hooks/internal/quality"
)

type input struct {
	ToolInput struct {
		FilePath string `json:"file_path"`
	} `json:"tool_input"`
	Cwd string `json:"cwd"`
}

type output struct {
	SystemMessage      string        `json:"systemMessage,omitempty"`
	HookSpecificOutput *hookSpecific `json:"hookSpecificOutput,omitempty"`
}

type hookSpecific struct {
	HookEventName     string `json:"hookEventName"`
	AdditionalContext string `json:"additionalContext"`
}

func main() {
	// A payload that cannot be read names no file to check.
	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		return
	}
	var in input
	if json.Unmarshal(raw, &in) != nil || in.ToolInput.FilePath == "" {
		return
	}

	file := in.ToolInput.FilePath
	if !filepath.IsAbs(file) {
		file = filepath.Join(in.Cwd, file)
	}
	projectDir := os.Getenv("CLAUDE_PROJECT_DIR")
	if projectDir == "" {
		projectDir = in.Cwd
	}

	plan := quality.Resolve(os.DirFS("/"), quality.Input{File: file, ProjectDir: projectDir})
	emit(quality.Run(context.Background(), plan))
}

// emit writes notices as PostToolUse output. Nothing is written when there is
// nothing to report.
func emit(notices []quality.Notice) {
	if len(notices) == 0 {
		return
	}
	summaries := make([]string, len(notices))
	details := make([]string, len(notices))
	for i, n := range notices {
		summaries[i] = n.Summary
		details[i] = n.Detail
	}
	enc, err := json.Marshal(output{
		SystemMessage: strings.Join(summaries, "\n"),
		HookSpecificOutput: &hookSpecific{
			HookEventName:     "PostToolUse",
			AdditionalContext: strings.Join(details, "\n\n"),
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "claude-quality: could not encode the findings: %v\n", err)
		return
	}
	fmt.Println(string(enc))
}
