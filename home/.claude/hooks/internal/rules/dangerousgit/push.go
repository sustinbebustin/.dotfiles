package dangerousgit

import (
	"fmt"
	"strings"

	"claude-hooks/internal/hook"
)

// protectedBranches are the branch names a push must never reach unprompted.
var protectedBranches = []string{"main", "master"}

// pushQuietFlags are the `git push` flags that change nothing about what is
// pushed or where. Any other flag prompts: --force, --mirror, --all, --tags,
// --delete, --prune, --repo, -o and the rest either widen the push or send it
// somewhere the refspecs do not name.
var pushQuietFlags = map[string]bool{
	"-u": true, "--set-upstream": true,
	"-v": true, "--verbose": true,
	"-q": true, "--quiet": true,
	"-n": true, "--dry-run": true,
	"--progress": true, "--no-progress": true,
	"--porcelain": true,
}

// pushQuietShorthands are the pushQuietFlags that bundle (`-uv`).
const pushQuietShorthands = "uvqn"

// checkPush lets a push through unprompted only when every branch it writes
// is named literally and none is protected: `git push -u origin feat/x` or
// `git push origin HEAD:feat/x`. Everything else asks, including a bare
// `git push` or `git push origin HEAD`: which branch they write depends on
// what is checked out, and this guard cannot see that.
//
// prefix is the git top-level flags before `push`; args are the tokens after
// it. literal reports whether every word of the command was plain text, with
// no expansion whose value the guard cannot know.
func checkPush(prefix, args []string, literal bool) (hook.Verdict, bool) {
	if !literal {
		return hook.Asked("git push with a shell expansion in its arguments - allow?"), true
	}
	for i := 0; i < len(prefix); i++ {
		if prefix[i] == "-C" {
			i++
			continue
		}
		return hook.Asked(fmt.Sprintf("git push with top-level %s detected - allow?", prefix[i])), true
	}

	var operands []string
	for i, a := range args {
		if a == "--" {
			operands = append(operands, args[i+1:]...)
			break
		}
		if strings.HasPrefix(a, "-") && !isQuietPushFlag(a) {
			return hook.Asked(fmt.Sprintf("git push %s detected - allow?", a)), true
		}
		if !strings.HasPrefix(a, "-") {
			operands = append(operands, a)
		}
	}

	if len(operands) < 2 {
		return hook.Asked("git push without an explicit branch pushes whatever is checked out, " +
			"which could be main - allow?"), true
	}
	if !plainRefText(operands[0], "@") {
		return hook.Asked(fmt.Sprintf("git push to remote %q detected - allow?", operands[0])), true
	}
	for _, spec := range operands[1:] {
		if v, hit := checkRefspec(spec); hit {
			return v, true
		}
	}
	return hook.Verdict{}, false
}

func isQuietPushFlag(a string) bool {
	if pushQuietFlags[a] {
		return true
	}
	if strings.HasPrefix(a, "--") || len(a) < 2 {
		return false
	}
	for _, r := range a[1:] {
		if !strings.ContainsRune(pushQuietShorthands, r) {
			return false
		}
	}
	return true
}

// checkRefspec asks unless spec writes one literally named, unprotected
// branch.
func checkRefspec(spec string) (hook.Verdict, bool) {
	switch {
	case strings.HasPrefix(spec, "+"):
		return hook.Asked(fmt.Sprintf("git push with forced refspec %q detected - allow?", spec)), true
	case strings.HasPrefix(spec, ":"):
		return hook.Asked(fmt.Sprintf("git push deleting remote ref %q detected - allow?", spec[1:])), true
	case !plainRefText(spec, ":"):
		// Globs, braces, and ref syntax such as ~ and ^ either expand in the
		// shell or select refs the guard would have to resolve.
		return hook.Asked(fmt.Sprintf("git push with refspec %q detected - allow?", spec)), true
	}

	src, dst, mapped := strings.Cut(spec, ":")
	if !mapped {
		dst = src
	}
	branch, isBranch := strings.CutPrefix(dst, "refs/heads/")
	switch {
	case dst == "" || dst == "HEAD":
		return hook.Asked(fmt.Sprintf("git push %s writes whatever is checked out, "+
			"which could be main - allow?", spec)), true
	case !isBranch && strings.HasPrefix(dst, "refs/"):
		return hook.Asked(fmt.Sprintf("git push to %s (not a branch) detected - allow?", dst)), true
	case isProtected(branch):
		return hook.Asked(fmt.Sprintf("git push to %s detected - allow?", branch)), true
	}
	return hook.Verdict{}, false
}

// plainRefText reports whether s holds only characters that the shell passes
// through unchanged and that name a ref directly, plus any in extra.
func plainRefText(s, extra string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case strings.ContainsRune("._/-", r), strings.ContainsRune(extra, r):
		default:
			return false
		}
	}
	return true
}

func isProtected(branch string) bool {
	for _, p := range protectedBranches {
		if strings.EqualFold(branch, p) {
			return true
		}
	}
	return false
}
