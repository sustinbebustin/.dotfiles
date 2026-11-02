package quality

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"
)

// OverrideFile is where a project states overrides, relative to a directory
// above the edited file. The nearest one applies whole; outer ones are not
// merged in.
//
//	{"quality": {"node": {"format": false},
//	             "go":   {"lint": ["golangci-lint", "run", "{pkg}"]}}}
//
// Keys under "quality" are ecosystem names, then step kinds. false disables the
// step; a command replaces whatever was detected for it and runs in the
// ecosystem's project root, with {file} and {pkg} filled in (see target). A
// step not listed is still detected.
const OverrideFile = ".claude/hooks.json"

// override is what a project says about one step: disabled or command.
type override interface{ isOverride() }

type disabled struct{}

type command struct{ argv []string }

func (disabled) isOverride() {}
func (command) isOverride()  {}

// overrides holds a parsed OverrideFile by ecosystem name, then step.
type overrides map[string]map[Kind]override

// findOverrides reads the nearest OverrideFile from dir up to stop. Finding
// none is not an error and yields no overrides. p is the path read, for
// reporting.
func findOverrides(fsys fs.FS, dir, stop string, cat []ecosystem) (ovs overrides, p string, err error) {
	for d := range ancestors(dir, stop) {
		p = path.Join(d, OverrideFile)
		raw, err := fs.ReadFile(fsys, p)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, p, fmt.Errorf("reading it: %w", err)
		}
		ovs, err := parseOverrides(raw, cat)
		return ovs, p, err
	}
	return nil, "", nil
}

// parseOverrides validates the whole file or rejects it: a typo that silently
// half-applied would run a tool the file was written to stop.
func parseOverrides(raw []byte, cat []ecosystem) (overrides, error) {
	var file struct {
		Quality map[string]map[string]json.RawMessage `json:"quality"`
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&file); err != nil {
		return nil, fmt.Errorf("decoding JSON: %w", err)
	}

	ovs := overrides{}
	for ecoName, steps := range file.Quality {
		i := slices.IndexFunc(cat, func(e ecosystem) bool { return e.name == ecoName })
		if i < 0 {
			return nil, fmt.Errorf("quality.%s: unknown ecosystem. Known: %s", ecoName, strings.Join(ecosystemNames(cat), ", "))
		}
		eco := cat[i]
		ovs[ecoName] = map[Kind]override{}
		for stepName, rawStep := range steps {
			kind := Kind(stepName)
			if !slices.Contains(eco.steps, kind) {
				return nil, fmt.Errorf("quality.%s.%s: unknown step. Known: %s", ecoName, stepName, strings.Join(kindNames(eco.steps), ", "))
			}
			o, err := parseStep(rawStep)
			if err != nil {
				return nil, fmt.Errorf("quality.%s.%s: %w", ecoName, stepName, err)
			}
			ovs[ecoName][kind] = o
		}
	}
	return ovs, nil
}

func parseStep(raw json.RawMessage) (override, error) {
	var b bool
	if json.Unmarshal(raw, &b) == nil {
		if b {
			return nil, errors.New("true is not a setting. Use false to disable the step, or leave it out to detect it")
		}
		return disabled{}, nil
	}
	var argv []string
	if err := json.Unmarshal(raw, &argv); err != nil || len(argv) == 0 {
		return nil, fmt.Errorf("must be false or a non-empty array of strings (the command and its arguments), got %s", raw)
	}
	for i, a := range argv {
		if strings.TrimSpace(a) == "" {
			return nil, fmt.Errorf("argument %d is empty", i)
		}
	}
	return command{argv: argv}, nil
}

func ecosystemNames(cat []ecosystem) []string {
	out := make([]string, len(cat))
	for i, e := range cat {
		out[i] = e.name
	}
	return out
}

func kindNames(kinds []Kind) []string {
	out := make([]string, len(kinds))
	for i, k := range kinds {
		out[i] = string(k)
	}
	return out
}

// expand fills an override's placeholders in for t.
func (c command) expand(t target) []string {
	r := strings.NewReplacer("{file}", t.File, "{pkg}", t.Pkg)
	out := make([]string, len(c.argv))
	for i, a := range c.argv {
		out[i] = r.Replace(a)
	}
	return out
}
