## Overview

Go doc comments are first-class. `go doc`, `gopls` hover, and `pkg.go.dev` all read the same `//` lines that sit immediately above a declaration -- there is no separate format like JSDoc. The Go toolchain (`go vet`, `gofmt`) enforces structure, and Go 1.19 added a small markup language (headings, lists, doc links) that `gofmt` will rewrite for you.

Documentation is part of the API. A poorly written doc comment is a bug.

## The Iron Rule

**Every exported (capitalized) package, const, func, type, and var gets a doc comment. The first sentence is a complete sentence that begins with the name being declared. Use it to explain *what* and *why*; lean on doc links, examples, and `Deprecated:` markers instead of prose where possible.**

## Detection

Watch for these patterns in a code review:

```go
// RED FLAGS

// gets the user                        <- not a sentence, doesn't start with name
func GetUser(id string) (*User, error)

// FetchUser fetches a user.            <- tautological; adds zero information
func FetchUser(id string) (*User, error)

func ProcessOrder(o *Order) error {}    <- exported, no doc comment at all

// Returns true if the order is valid.  <- "returns true if" -- use "reports whether"
func (o *Order) IsValid() bool

// User represents a user.              <- "represents a" filler; describe the role
type User struct { ... }
```

## Placement Rules

- Doc comment sits **immediately above** the declaration with **no blank line** between.
- A blank line between the comment and the declaration severs them -- the comment becomes a free-floating comment and `go doc` will not pick it up.
- Use `//` line comments (idiomatic). `/* ... */` is allowed but rare outside package comments on `doc.go`.
- A directive like `//go:generate` is **not** a doc comment; `gofmt` will move it past any doc text to the end.

```go
// Good: glued to the declaration.
// Quote returns a double-quoted Go string literal representing s.
func Quote(s string) string { ... }

// BAD: blank line breaks the association.
// Quote returns a double-quoted Go string literal representing s.

func Quote(s string) string { ... }
```

## Package Comments

Every package gets one package comment, on the `package` clause. For multi-file packages, put it on its own file -- conventionally `doc.go`:

```go
// Package path implements utility routines for manipulating
// slash-separated paths.
//
// The path package should only be used for paths separated by
// forward slashes, such as the paths in URLs. This package does not
// deal with Windows paths with drive letters or backslashes; to
// manipulate operating system paths, use the [path/filepath] package.
package path
```

Rules:

- First sentence starts with **`Package <name>`**.
- One package comment per package -- if you put one on multiple files, godoc concatenates them in an unspecified order.
- For a `main` package (a command), describe the **program**, not the package. The first sentence starts with the capitalized program name:

```go
/*
Gofmt formats Go programs.
It uses tabs for indentation and blanks for alignment.

Usage:

    gofmt [flags] [path ...]
*/
package main
```

## Functions and Methods

```go
// HasPrefix reports whether the string s begins with prefix.
func HasPrefix(s, prefix string) bool

// Copy copies from src to dst until either EOF is reached on src or
// an error occurs. It returns the total number of bytes written and
// the first error encountered while copying, if any.
//
// A successful Copy returns err == nil, not err == io.EOF, because
// Copy is defined to read from src until EOF.
func Copy(dst Writer, src Reader) (n int64, err error)
```

Conventions:

- Begin with the function name.
- Use **"reports whether"** for boolean returns, not "returns true if". This is a Go-wide convention.
- Reference parameter names as plain words (no backticks).
- Top-level functions are assumed safe for concurrent use; only mention concurrency if a function is **not** safe.
- Methods on a type are assumed safe **only on a single goroutine** unless documented otherwise.
- Document non-obvious error semantics (e.g. "returns io.EOF only at EOF").

## Types

```go
// A Reader serves content from a ZIP archive.
type Reader struct { ... }

// Regexp is the representation of a compiled regular expression.
// A Regexp is safe for concurrent use by multiple goroutines, except
// for configuration methods, such as Longest.
type Regexp struct { ... }

// Buffer is a variable-sized buffer of bytes with [Buffer.Read] and
// [Buffer.Write] methods. The zero value for Buffer is an empty
// buffer ready to use.
type Buffer struct { ... }
```

Conventions:

- Document the **role** of the type, not its shape (the shape is in the source).
- State the zero-value contract if it is meaningful ("The zero value is ready to use").
- State concurrency safety on types that hold state.
- For interfaces, describe the contract callers must satisfy.

## Constants and Variables

Group-related declarations share one doc comment on the block:

```go
// The result of Scan is one of these tokens or a Unicode character.
const (
    EOF = -(iota + 1)
    Ident
    Int
    Float
    // ...
)

// Generic file system errors.
// Errors returned by file systems can be tested against these errors
// using [errors.Is].
var (
    ErrInvalid    = errInvalid()    // "invalid argument"
    ErrPermission = errPermission() // "permission denied"
    ErrExist      = errExist()      // "file already exists"
)
```

Single declarations get their own comment:

```go
// Version is the Unicode edition from which the tables are derived.
const Version = "13.0.0"
```

## Doc Links (Go 1.19+)

Reference other Go symbols with `[Name]` so `go doc` and pkg.go.dev render them as hyperlinks. Use these in prose -- they are far better than naked names.

| Form | Refers to |
|------|-----------|
| `[Name]` | Exported identifier in current package |
| `[Type.Method]` | Method in current package |
| `[*bytes.Buffer]` | Pointer type (leading `*` allowed) |
| `[pkg]` | Another package |
| `[pkg.Name]` | Identifier in another package |
| `[pkg.Name.Method]` | Method in another package |
| `[encoding/json.Decoder]` | Full import path also works |

```go
// NewDecoder returns a new decoder that reads from r.
//
// The decoder introduces its own buffering and may read data from r
// beyond the JSON values requested. See [Decoder.Token] for tokenized
// access; for streaming use [Decoder.Decode] in a loop.
func NewDecoder(r io.Reader) *Decoder
```

Doc links must be flanked by spaces, punctuation, or line boundaries. Things like `map[ast.Expr]TypeAndValue` are **not** doc links -- the brackets are part of the type.

## URL Links

Reference-style URL links keep the prose readable; `gofmt` collects the targets at the bottom of the comment.

```go
// Marshal serializes v as JSON. See [RFC 8259] for the JSON spec
// and "[JSON and Go]" for an introduction.
//
// [RFC 8259]: https://tools.ietf.org/html/rfc8259
// [JSON and Go]: https://go.dev/blog/json
func Marshal(v any) ([]byte, error)
```

Bare URLs are auto-linked, but reference-style links read better.

## Lists

Bullet list (use `-`; `gofmt` normalizes other markers to `-`):

```go
// Examples:
//   - the public suffix of "example.com" is "com",
//   - the public suffix of "foo1.foo2.foo3.co.uk" is "co.uk", and
//   - the public suffix of "bar.pvt.k12.ma.us" is "pvt.k12.ma.us".
```

Numbered list (`1.` or `1)`):

```go
// Clean applies these rules iteratively until no further processing
// can be done:
//
//  1. Replace multiple slashes with a single slash.
//  2. Eliminate each . path name element.
//  3. Eliminate each inner .. path name element along with the
//     non-.. element that precedes it.
//  4. Eliminate .. elements that begin a rooted path.
```

List items contain only paragraphs -- **no nested lists, no code blocks inside list items**. `gofmt` flattens any nesting.

## Code Blocks

Indent the block. `gofmt` pads with a blank line before and after and re-indents to a single tab.

```go
// Example usage:
//
//  cfg := &Config{Timeout: 5 * time.Second}
//  client, err := New(cfg)
//  if err != nil {
//      log.Fatal(err)
//  }
```

For runnable examples, prefer an `Example` test function (see below) -- it gets compiled, executed, and checked.

## Headings (Go 1.19+)

```go
// Package strconv implements conversions to and from string
// representations of basic data types.
//
// # Numeric conversions
//
// The most common numeric conversions are [Atoi] (string to int) and
// [Itoa] (int to string).
//
// # String conversions
//
// [Quote] and [Unquote] convert to and from double-quoted Go string
// literals.
package strconv
```

Rules: `#` followed by a single space, on its own line, surrounded by blank lines, unindented. Use sparingly -- they are for long package comments, not function docs.

## Deprecation

A paragraph starting with `Deprecated:` triggers special handling: pkg.go.dev hides the symbol by default and `gopls` warns at every call site.

```go
// Sum returns the sum of v.
//
// Deprecated: Use [SumContext] instead, which respects cancellation.
func Sum(v []int) int
```

The `Deprecated:` paragraph can appear anywhere in the comment, but conventionally goes last. Always state the replacement.

## TODO and BUG Comments

Form: `MARKER(uid): body`. `MARKER` is two or more uppercase letters; `uid` is typically a username. These are collected and surfaced by godoc/pkg.go.dev under their own section.

```go
// BUG(rsc): Mapping only works for Latin script.

// TODO(austin): refactor to accept a context.Context.
```

These are package-level comments -- they sit on no particular declaration.

## Examples (Testable Docs)

Examples are functions in `*_test.go` named `ExampleX` that godoc surfaces as code samples. Add an `// Output:` comment and `go test` will execute and verify them.

```go
// in strings/example_test.go
func ExampleHasPrefix() {
    fmt.Println(strings.HasPrefix("Gopher", "Go"))
    fmt.Println(strings.HasPrefix("Gopher", "C"))
    // Output:
    // true
    // false
}

// Method examples:  ExampleType_Method
func ExampleBuffer_Write() { ... }

// Disambiguate multiple examples for the same symbol with a suffix:
//   ExampleHasPrefix_match     ExampleHasPrefix_empty
func ExampleHasPrefix_empty() { ... }
```

For unordered output (e.g. map iteration), use `// Unordered output:`.

Prefer examples over hand-rolled code blocks: they cannot rot, because the test suite verifies them.

## Pressure Resistance Protocol

When documenting Go code:

1. **Start with the name.** "Foo does X." Never "Does X." `go vet` and reviewers will flag the latter.
2. **Use `[Name]` for cross-references.** Cheaper for the reader than naked words and renders as a link.
3. **State the zero-value contract.** Other Go programmers will assume the type follows the standard "ready to use" pattern unless told otherwise.
4. **State concurrency safety on stateful types.** Default assumption: types are not goroutine-safe; functions are.
5. **Use `Deprecated:`, not freeform "this is old."** It is the only marker tooling recognizes.
6. **Prefer `Example*` tests over inline code blocks.** They are checked by the compiler and the test runner.
7. **Run `gofmt`.** It will normalize lists, links, and indentation -- do not fight it.

## Red Flags

| Anti-pattern | Problem | Fix |
|--------------|---------|-----|
| Missing doc on exported symbol | `go vet`/golint flag; pkg.go.dev shows nothing | Add a sentence starting with the name |
| Comment doesn't start with the name | Breaks tooling expectations and convention | Rewrite: `Foo does X.` |
| `// returns true if ...` | Non-idiomatic for booleans | Use `reports whether` |
| Tautological docs (`Foo does foo`) | Zero information added | Describe the *role* and *why* |
| Blank line between comment and decl | Comment is detached from the symbol | Remove the blank line |
| Naked URLs / package names in prose | Render as text, not links | Use `[Name]` doc links and reference-style URLs |
| `// TODO: deprecated` | Tools ignore it | Use a `Deprecated:` paragraph |
| Custom freeform format for examples | Cannot be verified by `go test` | Use `Example*` testable functions |
| Documenting struct fields by repeating their type | Pure noise | Document only fields with non-obvious semantics |

## Common Rationalizations

### "The signature is self-explanatory."

Reality: `go doc` and pkg.go.dev render the comment regardless. A blank doc on `Marshal(v any) ([]byte, error)` tells callers nothing about error conditions, encoding rules, supported types, or panic behavior.

### "It's an internal package, no one will read it."

Reality: `go doc ./internal/foo` works the same as on public code, and your future self is the primary reader. Internal packages still get doc comments; they just do not need exhaustive examples.

### "I'll add the example later."

Reality: examples added later get the API wrong because by then you have forgotten the calling-context assumptions. Write the `Example*` function while the API is fresh.

### "Concurrency safety is obvious."

Reality: it is the single most common source of misuse in Go. State it explicitly on every stateful type. The `sync.Mutex` zero-value pattern only saves you if callers know to follow it.

## Quick Reference

| Tag / form | Use for |
|------------|---------|
| `// Foo does X.` | Required first sentence for every exported `Foo` |
| `[Name]` | Link to a symbol in the current package |
| `[pkg.Name]` | Link to a symbol in another package |
| `[Text]: https://...` | Reference-style URL link |
| `# Heading` | Section heading (Go 1.19+) |
| `  - item` | Bullet list item |
| `  1. item` | Numbered list item |
| `Deprecated: ...` | Mark a symbol deprecated; tools react |
| `BUG(uid): ...` | Known package-level bug |
| `TODO(uid): ...` | Tracked package-level TODO |
| `ExampleFoo` | Testable example in `*_test.go` |
| `// Output:` | Asserted stdout for an example |
| `// Unordered output:` | Same, but tolerates line reordering |

## The Bottom Line

Go's doc comments are tooling, not decoration. `gofmt`, `go vet`, `gopls`, and `pkg.go.dev` all parse the same `//` lines, so following the conventions -- start with the name, complete sentences, doc links, `Deprecated:`, testable examples -- is what makes your package navigable. Treat the comment as part of the type signature.
