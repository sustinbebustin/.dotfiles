package supabaseremote

import (
	"strings"
	"testing"

	"claude-hooks/internal/hook"
)

func TestCheck(t *testing.T) {
	cases := []struct {
		name     string
		cmd      string
		decision string
	}{
		// db reset: local by default, a hard deny against any remote target.
		{"reset defaults to local", `supabase db reset`, "allow"},
		{"reset --no-seed is local", `supabase db reset --no-seed`, "allow"},
		{"reset --local", `supabase db reset --local`, "allow"},
		{"reset --linked", `supabase db reset --linked`, "deny"},
		{"reset --linked=true", `supabase db reset --linked=true`, "deny"},
		{"reset --linked=false is local", `supabase db reset --linked=false`, "allow"},
		{"reset --project-ref", `supabase db reset --project-ref abcdefghijklmnopqrst`, "deny"},
		{"reset --project-ref=", `supabase db reset --project-ref=abcdefghijklmnopqrst`, "deny"},
		{"reset remote --db-url", `supabase db reset --db-url postgresql://postgres:pw@db.example.com:5432/postgres`, "deny"},
		{"reset localhost --db-url", `supabase db reset --db-url postgresql://postgres:postgres@127.0.0.1:54322/postgres`, "allow"},
		{"reset localhost name --db-url=", `supabase db reset --db-url=postgresql://postgres:postgres@localhost:54322/postgres`, "allow"},
		{"reset unreadable --db-url fails safe", `supabase db reset --db-url "$DB_URL"`, "deny"},
		{"reset --local beside a remote flag", `supabase db reset --local --linked`, "deny"},
		{"global flags before the subcommand", `supabase --workdir frontend db reset --linked`, "deny"},
		{"flag between the subcommand words", `supabase db --linked reset`, "deny"},

		// migration down reverts data as well.
		{"migration down local", `supabase migration down --last 1`, "allow"},
		{"migration down --linked", `supabase migration down --linked --last 1`, "deny"},

		// Deleting projects and branches is always remote.
		{"projects delete", `supabase projects delete abcdefghijklmnopqrst`, "deny"},
		{"branches delete", `supabase branches delete feature-x`, "deny"},

		// Remote writes that are not a wipe ask.
		{"db push defaults to remote", `supabase db push`, "ask"},
		{"db push --local", `supabase db push --local`, "allow"},
		{"migration repair defaults to remote", `supabase migration repair --status applied 20260101000000`, "ask"},
		{"migration repair --local", `supabase migration repair --local --status applied 20260101000000`, "allow"},
		{"migration up --linked", `supabase migration up --linked`, "ask"},
		{"migration up local", `supabase migration up`, "allow"},
		{"migration squash --linked", `supabase migration squash --linked`, "ask"},
		{"db query --linked", `supabase db query --linked "delete from users"`, "ask"},
		{"db query local", `supabase db query "select 1"`, "allow"},
		{"storage rm defaults to remote", `supabase storage rm ss:///bucket/a.png`, "ask"},
		{"storage rm --local", `supabase storage rm --local ss:///bucket/a.png`, "allow"},

		// Reads stay open, remote or not.
		{"remote dump", `supabase db dump --project-ref abcdefghijklmnopqrst --data-only -f out.sql`, "allow"},
		{"projects list", `supabase projects list`, "allow"},
		{"status", `supabase status`, "allow"},
		{"migration list --linked", `supabase migration list --linked`, "allow"},
		{"help", `supabase db reset --help`, "allow"},

		// Spellings of the command that still run the CLI.
		{"absolute path", `/home/linuxbrew/.linuxbrew/bin/supabase db reset --linked`, "deny"},
		{"pnpm exec", `pnpm exec supabase db reset --linked`, "deny"},
		{"pnpm -C exec", `pnpm -C frontend exec supabase db reset --linked`, "deny"},
		{"pnpm dlx", `pnpm dlx supabase db reset --linked`, "deny"},
		{"npx", `npx supabase db reset --linked`, "deny"},
		{"npx pinned version", `npx supabase@2.117.0 db reset --linked`, "deny"},
		{"bunx", `bunx supabase db push`, "ask"},
		{"yarn", `yarn supabase db reset --linked`, "deny"},
		{"behind env", `env SUPABASE_ACCESS_TOKEN=x supabase db reset --linked`, "deny"},
		{"pnpm add is not a call", `pnpm add -D supabase`, "allow"},

		// Nesting still walked.
		{"in a subshell", `(cd frontend && supabase db reset --linked)`, "deny"},
		{"behind &&", `just gsw main && supabase db reset --linked`, "deny"},
		{"in a pipeline", `yes | supabase db reset --linked`, "deny"},

		{"supabase as an argument", `echo supabase db reset --linked`, "allow"},
		{"unrelated command", `ls -la`, "allow"},
		{"unparseable command", `supabase db reset "unterminated`, "allow"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Check(hook.NewRequest("Bash", "", "", tc.cmd))
			if got.Decision.String() != tc.decision {
				t.Errorf("Check(%q) = %s, want %s (reason: %s)", tc.cmd, got.Decision, tc.decision, got.Reason)
			}
		})
	}
}

// TestReasonNamesTheTarget pins that a block says which database it would have
// hit, since that is what the user needs to judge it.
func TestReasonNamesTheTarget(t *testing.T) {
	cases := []struct{ cmd, want string }{
		{`supabase db reset --linked`, "the linked project"},
		{`supabase db reset --project-ref abcdefghijklmnopqrst`, "project abcdefghijklmnopqrst"},
		{`supabase db reset --db-url postgresql://u:pw@db.example.com/postgres`, "db.example.com"},
		{`supabase db push`, "the linked project"},
	}
	for _, tc := range cases {
		got := Check(hook.NewRequest("Bash", "", "", tc.cmd)).Reason
		if !strings.Contains(got, tc.want) {
			t.Errorf("Check(%q) reason = %q, want it to name %q", tc.cmd, got, tc.want)
		}
		if strings.Contains(got, "pw") {
			t.Errorf("Check(%q) reason leaks the connection string's password: %q", tc.cmd, got)
		}
	}
}

func TestNonBashIsIgnored(t *testing.T) {
	if got := Check(hook.NewRequest("Read", "", "", "supabase db reset --linked")); got.Decision != hook.Allow {
		t.Fatalf("Read payload = %q, want allow", got.Decision)
	}
}
