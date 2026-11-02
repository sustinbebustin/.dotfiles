## Overview

Rust doc comments are compiled artifacts. `///` and `/** */` desugar to `#[doc = "..."]`; `//!` and `/*! */` desugar to `#![doc = "..."]` ([Rust Reference, Comments](https://doc.rust-lang.org/reference/comments.html)). `rustdoc` renders them as CommonMark, `cargo test` **executes the code blocks**, and lints check the links. A doc comment is the only kind of comment the compiler can hold you to.

Because doctests run, a stale example is a failing build -- not just misleading prose.

## The Iron Rule

**Every public item gets a doc comment whose first line is a single-sentence summary in third-person present indicative ("Returns the ...", not "Return the ..."). Add `# Examples`; add `# Panics`, `# Errors`, and `# Safety` whenever they apply. Link with intra-doc links, never raw URLs to docs.rs.**

## Detection

```rust
// RED FLAGS

pub fn parse(s: &str) -> Result<Config, Error>   // public, undocumented

/// Parse the string.                            // imperative mood; RFC 1574 wants "Parses"
pub fn parse(s: &str) -> Result<Config, Error>

/// Returns a Config.
/// Takes a &str and gives back a Result<Config, Error>.   // restates the signature
pub fn parse(s: &str) -> Result<Config, Error>

/// Reads the whole file.                        // returns Result, no # Errors section
pub fn read_all(p: &Path) -> io::Result<Vec<u8>>

/// Inserts at index.                            // panics on out-of-bounds, no # Panics
pub fn insert(&mut self, i: usize, v: T)

/// Reads from the pointer.                      // unsafe fn with no # Safety
pub unsafe fn read<T>(src: *const T) -> T

/// See https://docs.rs/foo/latest/foo/struct.Bar.html   // bare URL; use [`Bar`]
pub fn make_bar() -> Bar

/// ```
/// let cfg = parse("x=1").unwrap();             // unwrap in an example users copy-paste
/// ```
pub fn parse(s: &str) -> Result<Config, Error>
```

## `///` vs `//!`

| Form | Documents | Typical placement |
|------|-----------|-------------------|
| `///` | The **item that follows** | Above every `pub fn`, `struct`, `enum`, `trait`, `const` |
| `//!` | The **enclosing** item (parent) | Top of `lib.rs` (crate docs) or `mod.rs` (module docs) |

RFC 1574 restricts `//!` to crate- and module-level docs: for a `mod` block, put `///` **outside** the block rather than `//!` inside it ([RFC 1574](https://rust-lang.github.io/rfcs/1574-more-api-documentation-conventions.html)).

```rust
//! Fast and easy queue abstraction.
//!
//! Provides an abstraction over a queue.

/// This module makes it easy.
pub mod easy {
    /// Use the abstraction function to do this specific thing.
    pub fn abstraction() {}
}
```

Prefer line comments over block comments. Block doc comments terminate at the first `*/`, so ``/** `glob = "*/*.rs";` */`` is a syntax error ([Rust Reference](https://doc.rust-lang.org/reference/comments.html)).

## The Summary Line

RFC 1574: the opening line is "a single-line short sentence providing a summary of the code," and it "should be written in third person singular present indicative form" -- **"Returns"**, not "Return" ([RFC 1574](https://rust-lang.github.io/rfcs/1574-more-api-documentation-conventions.html)).

> **Contradiction alert.** This is the opposite of Python (PEP 257 prescribes imperative mood: "Return the label"). It matches Java and C#. When switching languages in one session, re-check the mood.

Everything before the first blank line is reused as the search/module-overview blurb, so "It is good practice to keep the summary to one line: concise writing is a goal of good documentation" ([rustdoc book](https://doc.rust-lang.org/rustdoc/how-to-write-documentation.html)).

```rust
// DO
/// Returns the arguments which this program was started with.
///
/// The first element is traditionally the path of the executable, but it
/// can be set to arbitrary text, and may not even exist.
pub fn args() -> Args

// DON'T -- imperative, and the summary spills over the blurb boundary
/// Return the arguments the program started with, noting that the first
/// element is traditionally the path of the executable but may not exist.
pub fn args() -> Args
```

The recommended item structure ([rustdoc book](https://doc.rust-lang.org/rustdoc/how-to-write-documentation.html)):

```text
[short sentence explaining what it is]

[more detailed explanation]

[at least one code example that users can copy/paste to try it]

[even more advanced explanations if necessary]
```

Do not restate types. "Because the type system does a good job of defining what types a function passes and returns, there is no benefit of explicitly writing it into the documentation, especially since `rustdoc` adds hyper links to all types in the function signature" (ibid.).

## Sections

RFC 1574 lists the common headings, top-level `#`, **always plural** ("use the plural form: 'Examples' rather than 'Example'"), in this order:

```text
# Examples
# Panics
# Errors
# Safety
# Aborts
# Undefined Behavior
```

The Rust API Guidelines state which are expected ([API Guidelines: Documentation](https://rust-lang.github.io/api-guidelines/documentation.html)) -- all phrased as "should," which is the guidelines' strongest form:

| Section | Guideline | Rule |
|---------|-----------|------|
| `# Examples` | **C-EXAMPLE** | "All items have a rustdoc example." Applied within reason; a link to an example elsewhere can suffice. |
| `# Errors` | **C-FAILURE** | "Error conditions should be documented in an 'Errors' section." Includes trait methods whose impls may return errors. |
| `# Panics` | **C-FAILURE** | "Panic conditions should be documented in a 'Panics' section." Not every conceivable case, but "err on the side of documenting more panic cases." |
| `# Safety` | **C-FAILURE** | "Unsafe functions should be documented with a 'Safety' section that explains all invariants that the caller is responsible for upholding." |

`# Safety` on an `unsafe fn` is the one that is effectively non-negotiable: without it the caller has no way to know what contract they are signing.

```rust
/// Inserts an element at position `index` within the vector.
///
/// # Panics
///
/// Panics if `index > len`.
///
/// # Examples
///
/// ```
/// let mut v = vec![1, 2, 3];
/// v.insert(1, 4);
/// assert_eq!(v, [1, 4, 2, 3]);
/// ```
pub fn insert(&mut self, index: usize, element: T)

/// Reads the value from `src` without moving it.
///
/// # Safety
///
/// * `src` must be valid for reads.
/// * `src` must be properly aligned.
/// * `src` must point to a properly initialized value of type `T`.
pub unsafe fn read<T>(src: *const T) -> T
```

## Doctests Are Real Tests

Fenced code blocks in doc comments are compiled and run by `cargo test`. A block with no language tag is assumed to be Rust; `` ```rust `` and `` ``` `` are equivalent ([rustdoc book: Documentation tests](https://doc.rust-lang.org/rustdoc/write-documentation/documentation-tests.html)).

Pre-processing (why you rarely write `main`): if the block has no `fn main`, rustdoc wraps the code in one; `extern crate <mycrate>;` is injected; `unused_variables`, `unused_assignments`, `unused_mut`, `unused_attributes`, and `dead_code` are allowed (ibid.).

"Like regular unit tests, regular doctests are considered to 'pass' if they compile and run without panicking" -- so assert, don't just print:

```rust
/// ```
/// let foo = "foo";
/// assert_eq!(foo, "foo");
/// ```
```

Lines starting with `# ` are compiled but hidden from the rendered docs:

```rust
/// ```
/// # let cfg = my_crate::Config::default();
/// let out = my_crate::render(&cfg);
/// assert!(out.contains("hello"));
/// ```
```

### `?` in examples, not `unwrap`

API guideline **C-QUESTION-MARK**: "Examples use `?`, not `try!`, not `unwrap`," because "example code is often copied verbatim by users" and "Unwrapping an error should be a conscious decision that the user needs to make" ([API Guidelines](https://rust-lang.github.io/api-guidelines/documentation.html)).

`?` needs a `Result`-returning `main`. Hide it:

```rust
/// ```
/// use std::io;
/// # fn main() -> io::Result<()> {
/// let mut input = String::new();
/// io::stdin().read_line(&mut input)?;
/// # Ok(())
/// # }
/// ```
```

Or, since 1.34, omit `main` and pin the error type with a trailing hidden line -- write `(())` with no intervening whitespace so rustdoc recognizes it ([rustdoc book](https://doc.rust-lang.org/rustdoc/write-documentation/documentation-tests.html)):

```rust
/// ```
/// use std::io;
/// let mut input = String::new();
/// io::stdin().read_line(&mut input)?;
/// # Ok::<(), io::Error>(())
/// ```
```

### Code block attributes

| Attribute | Effect |
|-----------|--------|
| `ignore` | Not built, not run. "Almost never what you want" -- prefer `text` or hidden `#` lines |
| `should_panic` | Must compile **and** panic |
| `no_run` | Compiled, not executed (network, infinite loops, UB demos) |
| `compile_fail` | Compilation must fail |
| `text` | Not Rust; not tested |
| `edition2015` / `2018` / `2021` / `2024` | Compile under that edition |

Mis-typed attributes (`should-panic`) are caught by `rustdoc::invalid_codeblock_attributes`, warn-by-default.

## Intra-Doc Links

Link by Rust **path**, not URL. Backticks around the link text are stripped, and `[bar][Bar]` works without a reference definition ([rustdoc book: Linking to items by name](https://doc.rust-lang.org/rustdoc/write-documentation/linking-to-items-by-name.html)).

```rust
/// This is a version of [`Receiver<T>`] with support for [`std::future`].
///
/// You can obtain a [`std::future::Future`] by calling [`Self::recv()`].
pub struct AsyncReceiver<T> { /* ... */ }
```

- Anything in scope where the item is **defined** resolves: `Self`, `self`, `super`, `crate`, dependencies, `std`/`core`/`alloc`, primitives, associated items.
- Generics resolve: `[`Vec<T>`]` behaves like `[`Vec<T>`](Vec)`.
- Fragments work: `[positional parameters]: std::fmt#formatting-parameters`.
- Fully-qualified syntax (`<Vec as IntoIterator>::into_iter()`) is **not** supported yet.

Disambiguate across the type/value/macro namespaces with a `prefix@Path` (`struct@`, `fn@`, `mod@`, `type@`, `value@`, `prim@`, `macro@`, `trait@`, ...) or the suffix forms `foo()` for functions and `foo!()` for macros. The prefix is stripped in rendered output.

```rust
/// See also: [`Foo`](struct@Foo)
struct Bar;

/// This is different from [`Foo`](fn@Foo)
struct Foo {}
```

Anything containing `/` or `[]` is not treated as an intra-doc link and is silently left alone -- which is why a broken docs.rs URL never warns while a broken `[`Bar`]` does.

## Lints

Configure at the crate root ([rustdoc lints](https://doc.rust-lang.org/rustdoc/lints.html)):

```rust
#![warn(missing_docs)]
#![warn(rustdoc::broken_intra_doc_links)]
#![warn(rustdoc::missing_crate_level_docs)]
#![warn(rustdoc::unescaped_backticks)]
```

| Lint | Default | Catches |
|------|---------|---------|
| `missing_docs` | **allow** | Undocumented public items. Also available from `rustc`, unlike the rest |
| `rustdoc::missing_crate_level_docs` | allow | No `//!` docs at the crate root |
| `rustdoc::broken_intra_doc_links` | **warn** | Unresolved or ambiguous `[Name]` links |
| `rustdoc::private_intra_doc_links` | warn | Public docs linking to private items |
| `rustdoc::bare_urls` | warn | `http://...` written as text; suggests `<http://...>` |
| `rustdoc::invalid_codeblock_attributes` | warn | `should-panic` instead of `should_panic` |
| `rustdoc::invalid_rust_codeblocks` | warn | Empty or unparsable Rust blocks |
| `rustdoc::invalid_html_tags` | warn | Unclosed/invalid HTML |
| `rustdoc::redundant_explicit_links` | warn | `[`usize`](usize)` -- the explicit target adds nothing |
| `rustdoc::unescaped_backticks` | allow | `` `add(a, b) is the same as `add(b, a)`. `` |
| `rustdoc::private_doc_tests` | allow | Doctests on private items |

`missing_docs` being allow-by-default is why `#![warn(missing_docs)]` (or `deny`) belongs in every library crate root -- nothing enforces public-API docs otherwise.

## `#[doc(hidden)]`

Rustdoc "is supposed to include everything users need to use the crate fully and nothing more" ([API Guidelines: C-HIDDEN](https://rust-lang.github.io/api-guidelines/documentation.html)). Hide items that are technically public but unusable -- classically, impls involving private types.

```rust
// Users can never hold a PrivateError, so this impl is noise.
#[doc(hidden)]
impl From<PrivateError> for PublicError {
    fn from(e: PrivateError) -> Self { /* ... */ }
}
```

`#[doc(hidden)]` hides from docs; it does **not** make an item private. If the item should not be reachable at all, use `pub(crate)`.

## Markdown

Rustdoc renders CommonMark plus GitHub-flavored extensions: strikethrough, footnotes (`text[^note]`), pipe tables, task lists, and smart punctuation (`--` becomes an en dash) ([rustdoc book](https://doc.rust-lang.org/rustdoc/how-to-write-documentation.html)).

Warning callouts need a blank line before Markdown inside the HTML is interpreted:

```rust
/// <div class="warning">
///
/// Go to [this link](https://rust-lang.org)!
///
/// </div>
```

## Crate-Level Docs

**C-CRATE-DOC**: "Crate level docs are thorough and include examples." The first line should be "a sentence without highly technical details, but with a good description of where this crate fits within the rust ecosystem. Users should know whether this crate meets their use case after reading this line" ([rustdoc book](https://doc.rust-lang.org/rustdoc/how-to-write-documentation.html)).

Follow it with a real-world example, without shortcuts -- readers copy-paste the front page.

## Pressure Resistance Protocol

1. **Third person, one line, ends the sentence.** "Returns the ...", not "Return the ...".
2. **`# Safety` on every `unsafe fn`.** No exceptions -- it is the caller's only contract.
3. **`# Errors` on every fallible public fn; `# Panics` on every fn that can panic.** The signature shows `Result`; it does not show *why*.
4. **Examples use `?`.** Never `unwrap` in a snippet users will paste into production.
5. **`[`Name`]`, never a docs.rs URL.** Only intra-doc links get link-checked.
6. **Turn on `#![warn(missing_docs)]`.** It is allow-by-default; without it nothing enforces the rule.
7. **Let the doctest be the proof.** If the example doesn't compile, `cargo test` fails and you find out immediately.

## Red Flags

| Anti-pattern | Problem | Fix |
|--------------|---------|-----|
| Public item with no `///` | `missing_docs` is off by default, so nothing catches it | Enable the lint; write the summary |
| "Parse the string." | Imperative; RFC 1574 specifies third person | "Parses the string." |
| Summary restates the signature | Rustdoc already links every type in the signature | Describe role and intent |
| Multi-line first paragraph | Whole paragraph becomes the search blurb | One-sentence summary, then a blank line |
| `# Example` (singular) | RFC 1574 mandates the plural for tooling | `# Examples` |
| `unsafe fn` with no `# Safety` | Caller cannot uphold an unstated invariant | Enumerate every precondition |
| Fallible fn with no `# Errors` | Callers guess which failures are recoverable | List the error conditions |
| `.unwrap()` in an example | Copied verbatim into user code | Use `?` with a hidden `main` |
| `` ```ignore `` | Silently untested and usually rots | Use hidden `#` lines, or `text` if it isn't Rust |
| Bare docs.rs URL | Not link-checked; breaks on rename | `[`Bar`]` intra-doc link |
| `[`usize`](usize)` | Redundant; `redundant_explicit_links` warns | `[`usize`]` |
| `#[doc(hidden)]` used to "make private" | Item is still callable | `pub(crate)` |

## Common Rationalizations

### "The type signature already says it returns a Result."

Reality: it says failure is possible; it does not say *which* failures, whether they are retryable, or what state is left behind. That is what `# Errors` is for, and it is exactly the information a caller needs to write a correct match arm.

### "It's an internal crate, nobody reads the docs."

Reality: `cargo doc --open` works on internal crates, and `#![warn(missing_docs)]` costs one line. The doctest is worth more than the prose anyway -- it is a compile-checked usage test you get for free.

### "The example is trivial, it isn't worth writing."

The rustdoc book disagrees: "while you might think that a code example is trivial, the examples are really important because they can help users understand what an item is, how it is used, and for what purpose it exists." A trivial example is still the fastest possible answer to "how do I call this?"

### "I'll link with a URL, it's the same thing."

Reality: it is not. Anything containing `/` is ignored by `broken_intra_doc_links`, so a URL that goes stale after a rename fails silently forever. `[`Bar`]` produces a warning the moment `Bar` moves.

### "The unsafe block is obviously fine."

Reality: `# Safety` is not for you, it is for the caller -- who is writing the `unsafe { }` block and legally owes the compiler proof of preconditions they cannot see. Omitting it makes the function unusable-in-good-conscience.

## Quick Reference

| Form | Use for |
|------|---------|
| `/// Returns ...` | Doc comment on the following item |
| `//! ...` | Doc comment on the enclosing crate or module |
| `# Examples` | Copy-pasteable usage (C-EXAMPLE) |
| `# Panics` | Every condition that panics (C-FAILURE) |
| `# Errors` | Every error condition of a fallible fn (C-FAILURE) |
| `# Safety` | Caller-upheld invariants of an `unsafe fn` (C-FAILURE) |
| `[`Name`]` | Intra-doc link to an item in scope |
| `[text](struct@Name)` | Namespace-disambiguated link |
| `[`Self::method()`]` | Link to an associated item |
| `` ``` `` | Doctest -- compiled and run by `cargo test` |
| `# ` (line prefix) | Hidden-but-compiled doctest line |
| `no_run` / `should_panic` / `compile_fail` / `text` | Code block attributes |
| `#[doc(hidden)]` | Keep an unusable public item out of the docs |
| `#![warn(missing_docs)]` | Enforce docs on public items (off by default) |
| `#[doc = include_str!("../README.md")]` | Reuse the README as crate docs |

## The Bottom Line

Rust doc comments are checked by the toolchain: `rustdoc` resolves the links, `cargo test` runs the examples, and the lints flag what is missing. Write the summary in third person on one line, add `# Examples` always and `# Panics` / `# Errors` / `# Safety` whenever they apply, link with `[`Name`]`, and turn `missing_docs` on. Anything you write that the tooling can verify will stay true; anything it cannot verify will eventually lie.
