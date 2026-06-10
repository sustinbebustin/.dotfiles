# Cutting a release

Reference for the `release` skill: how to finalize a hand-maintained `CHANGELOG.md` and publish the GitHub release. Follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) + [SemVer](https://semver.org/spec/v2.0.0.html).

## Why this order

Commit the finalized changelog first, push, then create the release **targeting that commit**. The tag then points at a commit whose changelog already shows the version as released -- not at a commit that still says "Unreleased". `gh release create` creates the tag for you on the remote, so there's no separate `git tag` / push-tag step.

## Changelog transformation

Move the `[Unreleased]` entries down into a new dated version section, and leave `[Unreleased]` empty for the next cycle.

Before (cutting `v1.8.7` on `2026-06-10`):

```markdown
## [Unreleased]

### Added
- Add SSO login

### Fixed
- Fix timezone drift on reports

## [1.8.6] - 2026-05-01
...

[Unreleased]: https://github.com/user/repo/compare/v1.8.6...HEAD
[1.8.6]: https://github.com/user/repo/releases/tag/v1.8.6
```

After:

```markdown
## [Unreleased]

## [1.8.7] - 2026-06-10

### Added
- Add SSO login

### Fixed
- Fix timezone drift on reports

## [1.8.6] - 2026-05-01
...

[Unreleased]: https://github.com/user/repo/compare/v1.8.7...HEAD
[1.8.7]: https://github.com/user/repo/compare/v1.8.6...v1.8.7
[1.8.6]: https://github.com/user/repo/releases/tag/v1.8.6
```

Rules:

- Keep an empty `## [Unreleased]` heading at the top (no subsections under it).
- Insert `## [X.Y.Z] - YYYY-MM-DD` directly below it, then the moved subsections in Keep a Changelog order: Added, Changed, Deprecated, Removed, Fixed, Security. Drop any empty subsections.
- Date is ISO 8601 (`YYYY-MM-DD`), today's date from Current State.

## Compare-link footer

Only touch the footer if the changelog already has one (Current State reports this -- don't fabricate links for a changelog that never used them).

- Repoint `[Unreleased]` to compare from the new tag: `...vX.Y.Z...HEAD`.
- Add a `[X.Y.Z]` line comparing the previous tag to the new one: `compare/<prevTag>...vX.Y.Z`.
- Match the existing URL shape exactly (host, owner/repo, `v` prefix). Copy the previous line's pattern rather than guessing.
- For the very first release there's no previous tag: link `[X.Y.Z]` to `releases/tag/vX.Y.Z`.

## Version choice

SemVer bump from the `[Unreleased]` content:

| Unreleased contains | Bump |
|---------------------|------|
| `**BREAKING:**` or `### Removed` (incompatible) | major |
| `### Added` (backwards-compatible feature) | minor |
| only `### Fixed` / `### Changed` (compatible) | patch |

Keep the repo's existing tag prefix. If existing tags are `v1.8.6`, the new tag is `v1.8.7`; if they're bare `1.8.6`, use `1.8.7`. Pre-releases use SemVer identifiers (`2.0.0-rc.1`, `2.0.0-alpha.1`).

## Publishing with gh

```sh
notes="$(mktemp)"
# write the moved section bullets (no "## [X.Y.Z]" heading line) into "$notes"
gh release create <tag> \
  --target "$(git -C <root> rev-parse HEAD)" \
  --title "<tag>" \
  --notes-file "$notes"
rm -f "$notes"
```

- `<tag>` is the prefixed version, e.g. `v1.8.7`. `gh` creates the git tag at `--target` on the remote.
- `--target` is the just-pushed cut commit's SHA, so the tag lands exactly there. It must be the full 40-char SHA (use `git rev-parse HEAD`); a short SHA returns `422 target_commitish is invalid`.
- Release body = the version's changelog bullets (the `### Added` / `### Fixed` subsections), without the `## [X.Y.Z]` heading -- GitHub already shows the tag as the title.
- Add `--prerelease` for pre-release versions so GitHub doesn't mark them "Latest".
- `gh` prints the release URL on success. A pre-existing tag makes it fail loudly -- surface that to the user rather than retrying blindly.

After publishing, the tag exists only on the remote until the user runs `git pull --tags`.
