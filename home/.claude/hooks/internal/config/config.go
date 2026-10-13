// Package config loads the machine-local settings the guards consult.
//
// The hooks live in a public dotfiles repository, so anything machine-specific
// -- above all the directories where a recursive rm may run unprompted -- must
// stay out of the tracked tree. It is read from ~/.claude/hooks/config.json,
// which is gitignored and stowed like the binary beside it; config.json.example
// records the shape. $CLAUDE_HOOKS_CONFIG overrides the location.
//
// Having no config file at all is the normal state, not an error: the guards
// then run exactly as they did before this package existed.
//
// Everything is validated here rather than where it is used, so a rule only
// ever sees absolute, cleaned paths and known policies. Validation rejects the
// whole file rather than dropping the offending entry, and unknown fields are an
// error too: a typo that silently half-applied an allowlist is the wrong kind of
// surprise.
//
// Almost everything here widens what runs unprompted. The one exception is an
// awsCli profile policy of deny, which narrows it: a broken file drops such a
// deny along with everything else, and the affected calls fall back to asking.
// That is a lost block, never a gained allow.
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// EnvPath names the environment variable that overrides the config location.
// It is the seam the tests read a fixture through, and the way a second machine
// or a sandbox can point the same binary somewhere else.
const EnvPath = "CLAUDE_HOOKS_CONFIG"

// defaultRel is where the config lives when EnvPath is unset, relative to the
// user's home directory.
var defaultRel = filepath.Join(".claude", "hooks", "config.json")

// Config is the validated configuration a rule reads. Its zero value is the
// no-config state and is what every caller falls back to, so a rule must behave
// correctly with every field empty.
type Config struct {
	// AllowedRmRoots are absolute, cleaned directories under which a recursive
	// rm needs no approval. Membership is decided in dangerousrm; see
	// underAllowedRoot there for what "under" means.
	AllowedRmRoots []string
	// RelaxedCredRoots are absolute, cleaned directories where credfiles skips
	// its name-word heuristics (secret, credential, token, id_*): projects whose
	// own domain vocabulary trips them, such as an AWS emulator with a Secrets
	// Manager service. Credential material proper stays denied there too; see
	// credfiles.classify for the split.
	RelaxedCredRoots []string
	// AWSProfiles maps an AWS CLI profile name to what block-aws-cli does with a
	// call made under it. A profile missing from the map gets AWSAsk.
	AWSProfiles map[string]AWSPolicy
}

// AWSPolicy is what block-aws-cli does with an aws call that would reach real
// AWS under one profile. The zero value is AWSAsk, the rule's behaviour for any
// profile the config does not name.
type AWSPolicy int

const (
	// AWSAsk prompts for every call.
	AWSAsk AWSPolicy = iota
	// AWSReadOnly runs read-only operations unprompted and asks for the rest.
	AWSReadOnly
	// AWSAllow runs every call unprompted.
	AWSAllow
	// AWSDeny blocks every call.
	AWSDeny
)

// awsPolicyNames are the spellings config.json accepts, in the order the error
// for an unknown one lists them.
var awsPolicyNames = []struct {
	name   string
	policy AWSPolicy
}{
	{"allow", AWSAllow},
	{"readOnly", AWSReadOnly},
	{"ask", AWSAsk},
	{"deny", AWSDeny},
}

// String returns the policy as config.json spells it.
func (p AWSPolicy) String() string {
	for _, n := range awsPolicyNames {
		if n.policy == p {
			return n.name
		}
	}
	return fmt.Sprintf("AWSPolicy(%d)", int(p))
}

// file mirrors config.json on the wire. It is kept separate from Config so the
// JSON shape can be namespaced per rule -- room for a second rule's settings
// without disturbing the first -- while Config stays flat and validated.
type file struct {
	DangerousRm struct {
		AllowedRoots []string `json:"allowedRoots"`
	} `json:"dangerousRm"`
	CredentialFiles struct {
		RelaxedRoots []string `json:"relaxedRoots"`
	} `json:"credentialFiles"`
	AWSCli struct {
		Profiles map[string]string `json:"profiles"`
	} `json:"awsCli"`
}

// Load reads and validates the configuration.
//
// A missing file yields the zero Config and no error. Every other failure --
// unreadable file, malformed JSON, an unusable root -- is returned, and the
// caller runs with the zero Config, which costs extra prompts and never fewer.
func Load() (Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Config{}, fmt.Errorf("locating the home directory to find the hook config: %w", err)
	}

	path := os.Getenv(EnvPath)
	if path == "" {
		path = filepath.Join(home, defaultRel)
	}

	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("reading the hook config at %s: %w", path, err)
	}

	cfg, err := parse(raw, home)
	if err != nil {
		return Config{}, fmt.Errorf("in the hook config at %s: %w", path, err)
	}
	return cfg, nil
}

// parse decodes and validates raw. home resolves a leading "~" and is passed in
// rather than looked up so the whole validation path is testable without
// touching the environment.
func parse(raw []byte, home string) (Config, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	// A misspelled key would otherwise decode cleanly to nothing, leaving a
	// config that looks applied and is not.
	dec.DisallowUnknownFields()

	var f file
	if err := dec.Decode(&f); err != nil {
		return Config{}, fmt.Errorf("decoding JSON: %w. Compare against config.json.example", err)
	}

	rmRoots, err := normalizeRoots(f.DangerousRm.AllowedRoots, "dangerousRm.allowedRoots", home)
	if err != nil {
		return Config{}, err
	}
	credRoots, err := normalizeRoots(f.CredentialFiles.RelaxedRoots, "credentialFiles.relaxedRoots", home)
	if err != nil {
		return Config{}, err
	}
	awsProfiles, err := parseAWSProfiles(f.AWSCli.Profiles)
	if err != nil {
		return Config{}, err
	}
	return Config{AllowedRmRoots: rmRoots, RelaxedCredRoots: credRoots, AWSProfiles: awsProfiles}, nil
}

// parseAWSProfiles validates the awsCli.profiles map. Keys are checked in sorted
// order so the error for a file with several bad entries is stable.
func parseAWSProfiles(raw map[string]string) (map[string]AWSPolicy, error) {
	out := make(map[string]AWSPolicy, len(raw))
	for _, profile := range slices.Sorted(maps.Keys(raw)) {
		if strings.TrimSpace(profile) == "" {
			return nil, errors.New("awsCli.profiles has an empty profile name. Use the name as it appears " +
				"in ~/.aws/config, such as \"default\" or \"dev\"")
		}
		if strings.TrimSpace(profile) != profile {
			return nil, fmt.Errorf("awsCli.profiles[%q] has surrounding whitespace. Profile names match "+
				"exactly, so this entry would never apply; remove the spaces", profile)
		}
		policy, ok := parseAWSPolicy(raw[profile])
		if !ok {
			names := make([]string, len(awsPolicyNames))
			for i, n := range awsPolicyNames {
				names[i] = n.name
			}
			return nil, fmt.Errorf("awsCli.profiles[%q] is %q, which is not a policy. Use one of: %s",
				profile, raw[profile], strings.Join(names, ", "))
		}
		out[profile] = policy
	}
	return out, nil
}

func parseAWSPolicy(s string) (AWSPolicy, bool) {
	for _, n := range awsPolicyNames {
		if n.name == s {
			return n.policy, true
		}
	}
	return AWSAsk, false
}

// normalizeRoots normalizes every root of one setting, naming the setting and
// index of the first one that cannot be used.
func normalizeRoots(raws []string, setting, home string) ([]string, error) {
	roots := make([]string, 0, len(raws))
	for i, raw := range raws {
		root, err := normalizeRoot(raw, home)
		if err != nil {
			return nil, fmt.Errorf("%s[%d]: %w", setting, i, err)
		}
		roots = append(roots, root)
	}
	return roots, nil
}

// minRootDepth is the shallowest root that is accepted. A one-directory root
// (/home, /var, /Users) is almost never what someone means, and getting it
// wrong hands away recursive rm over most of a filesystem, so it is refused
// rather than trusted.
const minRootDepth = 2

// normalizeRoot turns one configured root into the absolute, cleaned form the
// rules compare against, or explains why it cannot be used.
func normalizeRoot(raw, home string) (string, error) {
	p := strings.TrimSpace(raw)
	if p == "" {
		return "", errors.New("is empty. Give an absolute directory, such as /home/you/dev/project")
	}

	switch {
	case p == "~":
		p = home
	case strings.HasPrefix(p, "~/"):
		p = filepath.Join(home, p[len("~/"):])
	}

	if !filepath.IsAbs(p) {
		return "", fmt.Errorf("%q is relative. Roots must be absolute (or start with ~/) so they "+
			"mean the same directory whatever the command's working directory is", raw)
	}

	p = filepath.Clean(p)
	if depth(p) < minRootDepth {
		return "", fmt.Errorf("%q is only %d directories deep, and a root must be at least %d. "+
			"A root this broad would wave through recursive rm across most of the filesystem",
			p, depth(p), minRootDepth)
	}
	return p, nil
}

// depth counts the directories in an absolute, cleaned path: "/" is 0, "/home"
// is 1, "/home/you/dev" is 3.
func depth(p string) int {
	if p == "/" {
		return 0
	}
	return strings.Count(p, "/")
}
