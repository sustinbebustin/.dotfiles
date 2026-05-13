# Changelog reference

Used by the `update-changelog` action: add entries under `[Unreleased]` in an existing `CHANGELOG.md`. Follows the [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) standard with [SemVer](https://semver.org/spec/v2.0.0.html).

## Release Please exception

If the repo has `release-please-config.json` at the root, Release Please owns `CHANGELOG.md`. Do not hand-edit it -- the bot generates entries from conventional-commit subjects on its release PR. Hand edits cause merge conflicts. (The skill's `gather-state.sh` already routes this case to `skip`.)

## Section types

Use these section headings, in this order. Omit empty sections.

1. `Added` -- new features
2. `Changed` -- changes in existing functionality
3. `Deprecated` -- soon-to-be removed features
4. `Removed` -- now removed features
5. `Fixed` -- bug fixes
6. `Security` -- vulnerability fixes

## Entry format

- Start with a verb (Add, Change, Deprecate, Remove, Fix).
- One short, user-facing sentence per bullet.
- Group related changes together.
- Include issue/PR references inline when helpful: `([#123](https://github.com/user/repo/issues/123))`.

### Do

- Focus on user impact, not implementation detail.
- Be specific and concrete.
- Note breaking changes prominently with a **BREAKING:** prefix.

### Don't

- No commit hashes in entries.
- No vague entries ("various fixes", "misc").
- No internal refactoring unless it affects users.
- No entries only developers would understand.

## File structure

```markdown
# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Add user authentication system
- Add password reset via email

### Changed
- Change default timeout from 30s to 60s
- **BREAKING:** Rename `config.timeout` to `config.requestTimeout`

### Fixed
- Fix login redirect loop ([#123](https://github.com/user/repo/issues/123))
- Fix memory leak in image processing

## [1.0.0] - 2024-01-15

### Added
- Initial release features

[Unreleased]: https://github.com/user/repo/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/user/repo/releases/tag/v1.0.0
```

## Notes for released versions

When a version is cut (not commit-push-pr's job, but useful context):

- Date format: ISO 8601 (`YYYY-MM-DD`).
- Yanked releases: append `[YANKED]` to the heading.
- Pre-release: SemVer identifiers (`2.0.0-alpha.1`, `2.0.0-rc.1`).
- Update the comparison links at the bottom for every version.
