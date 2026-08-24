package shellast

import (
	"testing"

	"mvdan.cc/sh/v3/syntax"
)

func TestParseStatus(t *testing.T) {
	cases := []struct {
		name string
		cmd  string
		want Status
	}{
		{"empty command", "", Absent},
		{"ordinary command", "ls -la", Parsed},
		{"unterminated quote", `echo "unterminated`, Unparseable},
		{"unclosed subshell", `cat x && (`, Unparseable},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := Parse(tc.cmd)
			if s.Status() != tc.want {
				t.Fatalf("Parse(%q).Status() = %v, want %v", tc.cmd, s.Status(), tc.want)
			}
			file, ok := s.File()
			if want := tc.want == Parsed; ok != want {
				t.Fatalf("File() ok = %v, want %v", ok, want)
			}
			if ok && file == nil {
				t.Fatal("File() returned ok with a nil AST")
			}
			if !ok && file != nil {
				t.Fatal("File() returned an AST alongside a false ok")
			}
		})
	}
}

// firstArg renders the first word of the first command in cmd.
func firstArg(t *testing.T, cmd string) string {
	t.Helper()
	file, ok := Parse(cmd).File()
	if !ok {
		t.Fatalf("Parse(%q) did not produce an AST", cmd)
	}
	got, hit := FirstCall(file, func(c *syntax.CallExpr) (string, bool) {
		if len(c.Args) == 0 {
			return "", false
		}
		return WordLit(c.Args[0]), true
	})
	if !hit {
		t.Fatalf("no call found in %q", cmd)
	}
	return got
}

func TestWordLit(t *testing.T) {
	cases := []struct {
		cmd  string
		want string
	}{
		{`ls`, "ls"},
		{`'ls'`, "ls"},
		{`"ls"`, "ls"},
		{`"pre"post`, "prepost"},
		// Expansions render as nothing; that is what makes WordLit safe to use
		// for command-name matching but unfit for resolving paths.
		{`$CMD`, ""},
		{`"$HOME/.netrc"`, "/.netrc"},
		{`$(echo ls)`, ""},
		// Backslashes quote the character after them, so the shell runs `ls`
		// here. The parser keeps the source text, so WordLit has to undo it.
		{`\ls`, "ls"},
		{`l\s`, "ls"},
		{`\\ls`, `\ls`},
	}

	for _, tc := range cases {
		if got := firstArg(t, tc.cmd); got != tc.want {
			t.Errorf("WordLit of %q = %q, want %q", tc.cmd, got, tc.want)
		}
	}
}

func TestWordLitNilWord(t *testing.T) {
	if got := WordLit(nil); got != "" {
		t.Errorf("WordLit(nil) = %q, want empty", got)
	}
}

func TestCommandName(t *testing.T) {
	cases := map[string]string{
		"aws":                "aws",
		"/usr/local/bin/aws": "aws",
		"./aws":              "aws",
		"":                   "",
		"dir/":               "",
	}
	for in, want := range cases {
		if got := CommandName(in); got != want {
			t.Errorf("CommandName(%q) = %q, want %q", in, got, want)
		}
	}
}

// invocation runs Invocation over the first call in cmd, rendering the result
// so a table can state it as plain text.
func invocation(t *testing.T, cmd string) (name string, rest []string) {
	t.Helper()
	file, ok := Parse(cmd).File()
	if !ok {
		t.Fatalf("Parse(%q) did not produce an AST", cmd)
	}
	call, hit := FirstCall(file, func(c *syntax.CallExpr) (*syntax.CallExpr, bool) { return c, true })
	if !hit {
		t.Fatalf("no call found in %q", cmd)
	}
	name, words := Invocation(call.Args, WordLit)
	rest = make([]string, len(words))
	for i, w := range words {
		rest[i] = WordLit(w)
	}
	return name, rest
}

func TestInvocation(t *testing.T) {
	cases := []struct {
		cmd  string
		name string
		rest []string
	}{
		{`git push`, "git", []string{"push"}},
		{`/usr/bin/git push`, "/usr/bin/git", []string{"push"}},
		{`\git push`, "git", []string{"push"}},
		{`"git" push`, "git", []string{"push"}},
		{`sudo git push`, "git", []string{"push"}},
		{`sudo -u deploy git push`, "git", []string{"push"}},
		{`sudo -n -u deploy -- git push`, "git", []string{"push"}},
		{`env FOO=1 BAR=2 git push`, "git", []string{"push"}},
		{`env -u FOO git push`, "git", []string{"push"}},
		{`sudo env FOO=1 command git push`, "git", []string{"push"}},
		{`nohup git push`, "git", []string{"push"}},
		// No command word to find.
		{`sudo`, "", nil},
		{`sudo -u deploy`, "", nil},
		{`$CMD push`, "", nil},
		{`sudo $CMD push`, "", nil},
		// Not a wrapper: the operand is a duration, not a command.
		{`timeout 5 git push`, "timeout", []string{"5", "git", "push"}},
	}

	for _, tc := range cases {
		t.Run(tc.cmd, func(t *testing.T) {
			name, rest := invocation(t, tc.cmd)
			if name != tc.name {
				t.Errorf("name = %q, want %q", name, tc.name)
			}
			if len(rest) != len(tc.rest) {
				t.Fatalf("rest = %q, want %q", rest, tc.rest)
			}
			for i := range rest {
				if rest[i] != tc.rest[i] {
					t.Errorf("rest = %q, want %q", rest, tc.rest)
					break
				}
			}
		})
	}
}

// TestInvocationRefusesWordsItCannotRead pins the fail-open edge: a wrapper
// argument that renders empty stops the search rather than being read as the
// command word, so `sudo $CMD` does not report "".
func TestInvocationRefusesWordsItCannotRead(t *testing.T) {
	for _, cmd := range []string{`$(which git) push`, `"" push`} {
		if name, _ := invocation(t, cmd); name != "" {
			t.Errorf("Invocation(%q) name = %q, want empty", cmd, name)
		}
	}
}

// TestFirstCallWalksTheWholeTree pins the traversal the rules that use it rely
// on: a call nested in a subshell, pipeline, or command substitution is found.
func TestFirstCallWalksTheWholeTree(t *testing.T) {
	for _, cmd := range []string{
		`(aws s3 ls)`,
		`echo x | aws s3 ls`,
		`true && aws s3 ls`,
		`echo $(aws s3 ls)`,
		`{ aws s3 ls; }`,
	} {
		file, ok := Parse(cmd).File()
		if !ok {
			t.Fatalf("Parse(%q) failed", cmd)
		}
		_, hit := FirstCall(file, func(c *syntax.CallExpr) (bool, bool) {
			if len(c.Args) == 0 {
				return false, false
			}
			return true, CommandName(WordLit(c.Args[0])) == "aws"
		})
		if !hit {
			t.Errorf("FirstCall did not find aws in %q", cmd)
		}
	}
}

// TestFirstCallStopsAtTheFirstMatch keeps the short-circuit honest: the rules
// report the first guarded call, not the last.
func TestFirstCallStopsAtTheFirstMatch(t *testing.T) {
	file, ok := Parse(`alpha; beta; gamma`).File()
	if !ok {
		t.Fatal("parse failed")
	}
	visited := 0
	got, hit := FirstCall(file, func(c *syntax.CallExpr) (string, bool) {
		visited++
		name := WordLit(c.Args[0])
		return name, name == "beta"
	})
	if !hit || got != "beta" {
		t.Fatalf("got (%q, %v), want (beta, true)", got, hit)
	}
	if visited != 2 {
		t.Errorf("visited %d calls, want 2 -- the walk should stop at the match", visited)
	}
}
