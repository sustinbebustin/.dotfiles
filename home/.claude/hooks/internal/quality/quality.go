// Package quality decides which formatters and linters to run over a file
// Claude just edited, and runs them. It backs the claude-quality PostToolUse
// hook.
//
// Nothing is configured per project by default. The edited file's extension
// picks an ecosystem (see catalog.go), and each of that ecosystem's tools runs
// only when its own config file sits above the edited file, within the file's
// git repository. A .claude/hooks.json above the file can replace or disable a
// step where detection gets it wrong (see override.go).
//
// Deciding is a pure function over an fs.FS (Resolve); only Run starts
// processes.
package quality

// Kind is one step a tool performs on the edited file. The values are the keys
// a step goes by in .claude/hooks.json.
type Kind string

const (
	// Lint reports problems in the file, fixing what the linter can.
	Lint Kind = "lint"
	// Format rewrites the file's layout.
	Format Kind = "format"
)

// Notice is one finding worth surfacing after an edit. Summary is a single
// line for the user; Detail is the full text handed to Claude.
type Notice struct {
	Summary string
	Detail  string
}

// Step is one command to run over the edited file.
type Step struct {
	// Label names the command in notices: the detected tool, or an override's
	// program.
	Label string
	// Dir is the absolute directory the command runs in.
	Dir  string
	Argv []string
	// interpret turns the command's result into a notice, or nil when there is
	// nothing to report.
	interpret func(ok bool, output string) *Notice
}

// Plan is what Resolve decided for one edited file: the steps to run, in
// order, and the notices that stand however those steps go.
type Plan struct {
	Steps   []Step
	Notices []Notice
}
