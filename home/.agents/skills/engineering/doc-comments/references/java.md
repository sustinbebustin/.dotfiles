## Overview

Javadoc comments are the Java API specification format. The same `/** */` (or, since JDK 23, `///`) block feeds IDE hover, `javadoc`-generated HTML, and the doclint checks that run inside `javac`. Oracle's style guide exists because these comments *are* the Java Platform API Specification -- its stated audience includes "people writing compatibility tests or re-implementing the platform," not just callers ([How to Write Doc Comments for the Javadoc Tool](https://www.oracle.com/technical-resources/articles/java/javadoc-tool.html)).

Doclint is **on by default** in `javadoc` and reports missing comments and tags as warnings, bad references and syntax as errors ([javadoc tool spec](https://docs.oracle.com/en/java/javase/25/docs/specs/man/javadoc.html)).

## The Iron Rule

**Every visible class, member, and record component gets a doc comment. The summary is a *fragment*: a verb phrase in third-person declarative for methods ("Returns the customer ID."), a noun phrase for classes and fields ("A button label."). Never imperative. Then `@param`, `@return`, `@throws`, `@deprecated` -- in that order, none with an empty description.**

## The Mood Rule -- and the Contradiction

Oracle: "Use 3rd person (descriptive) rather than 2nd person (prescriptive)." So `Gets the label.` is correct and `Get the label.` is not ([Oracle style guide](https://www.oracle.com/technical-resources/articles/java/javadoc-tool.html)).

Google Java Style 7.2 says the same thing from the other side: the summary fragment "is a noun phrase or verb phrase, not a complete sentence," it "does not begin with `A {@code Foo} is a...`, or `This method returns...`, nor does it form a complete imperative sentence like `Save the record.`" -- yet it "is capitalized and punctuated as if it were a complete sentence" ([Google Java Style Guide](https://google.github.io/styleguide/javaguide.html)).

> **Contradiction alert.** Java is third-person declarative. **Python (PEP 257) and Go are imperative/name-first.** Rust is also third person (RFC 1574), so Java and Rust agree with each other and disagree with Python. Do not carry a "Return the ..." habit from Python into Java, or a "Returns the ..." habit from Java into Python.

```java
// DO
/** Returns the label of this button. */
/** Gets the label. */
/** A button label. */                     // field: noun phrase, subject omitted

// DON'T
/** Get the label. */                      // 2nd person imperative
/** This method gets the label of this button. */   // states the subject
/** This field is a button label. */                // states the subject
/** @return the customer ID */             // no summary at all (Google 7.2: common error)
```

Method descriptions "begin with a verb phrase, since a method implements an operation." Class, interface, and field descriptions "can omit the subject and simply state the object," since they describe things rather than actions (Oracle style guide).

## Comment Placement

A doc comment is recognized **only immediately before** a declaration of a module, package, class, interface, constructor, method, annotation interface element, enum member, or field -- before any annotations, modifiers, or keywords. Only the comment closest to the declaration is used ([Documentation Comment Specification](https://docs.oracle.com/en/java/javase/24/docs/specs/javadoc/doc-comment-spec.html)).

## The Summary Sentence

"The first sentence should be a concise but complete summary" of the declared entity; it is extracted for summary tables and the index (Doc Comment Spec). To override the default first-sentence extraction, use `{@summary ...}` at the **beginning** of the main description.

The main description **cannot continue after any block tag** -- everything after the first `@param` is tag content (Doc Comment Spec).

```java
/**
 * {@summary Returns the first element.} Longer prose that is not the summary,
 * mentioning e.g. the Ph.D. abbreviation that would otherwise end the sentence.
 */
```

`{@return description}` (JDK 16+) writes both the summary sentence and the `@return` section from one source -- ideal for a getter whose description and return value are the same fact:

```java
/** {@return the customer ID} */
public String customerId() { ... }
```

## Block Tags

Order, per Oracle's style guide:

```text
@author        (classes and interfaces only)
@version       (classes and interfaces only)
@param         (methods and constructors only; declaration order)
@return        (methods only)
@throws        (@exception is an older synonym; alphabetical by exception name)
@see
@since
@serial        (or @serialField / @serialData)
@deprecated
```

Google Java Style 7.1.3 narrows the required sequence to `@param`, `@return`, `@throws`, `@deprecated`, and adds: "these four types never appear with an empty description." Continuation lines indent four or more spaces from the `@`.

Phrasing (Oracle style guide):

- Tag text is a **phrase, not a sentence**: do not capitalize it, do not end it with a period.
- If a phrase is followed by full sentences, still do not capitalize the phrase, but do end it with a period to separate it from what follows.
- Use the same capitalization and punctuation for `@return` as for `@param`.
- Do not wrap the `@param` name in `<code>`; javadoc does that, and since 1.4 it compares the name against the signature and warns on a mismatch.

```java
// DO
/**
 * Reads up to {@code len} bytes into {@code b}.
 *
 * @param b   the buffer into which the data is read
 * @param off the start offset in {@code b} at which the data is written
 * @param len the maximum number of bytes to read
 * @return the total number of bytes read, or {@code -1} at end of stream
 * @throws IOException if the first byte cannot be read for any reason
 *     other than end of file
 * @throws IndexOutOfBoundsException if {@code off} is negative
 */
int read(byte[] b, int off, int len) throws IOException;

// DON'T
/**
 * @param b The buffer into which the data is read.   <- capitalized, ends in period
 * @param <code>off</code> the offset                  <- HTML in the name; warns
 * @return                                             <- empty description
 */
```

### What is required, and when it may be omitted

A method comment must supply: the main description; a `@param` per type parameter; a `@return` if non-`void`; a `@param` per formal parameter; a `@throws` per exception in the `throws` clause. A missing item is an **error** if the declaration is not overriding; otherwise a missing item is treated as `{@inheritDoc}` -- **inheritance by omission** (Doc Comment Spec).

Oracle: omit `@return` for `void` methods and constructors; include it for every other method "even if its content is entirely redundant with the method description," because "an explicit `@return` tag makes the return value easier to find quickly."

Google Java Style 7.3 exempts two cases:

- **7.3.1 self-explanatory members** -- a simple `getFoo()` may go undocumented, "but only when there truly is nothing worth saying beyond restating the name." This exemption cannot justify "omitting relevant information that a typical reader might need to know."
- **7.3.2 overrides** -- "Javadoc is not always present on a method that overrides a supertype method."

Everything else that is visible (a `public` top-level class; a `public`/`protected` member of a visible class; a record component of a visible record) requires Javadoc.

## Inheritance and `{@inheritDoc}`

Bare `{@inheritDoc}` searches the supertypes in a defined order: the recursive phase walks the superclass chain (skipping `java.lang.Object`) then each direct superinterface in declaration order; only the **final phase** considers `java.lang.Object`, deliberately last, "since `Object`'s docs for `equals`, `hashCode`, `toString` are often overly general" (Doc Comment Spec). `{@inheritDoc S}` names the supertype explicitly.

Omitting a tag on an overriding method is equivalent to writing `{@inheritDoc}` for it. So:

```java
/**
 * @param scale a non-zero number
 * @throws IllegalArgumentException if scale is 0
 */
@Override
<T> T magnify(int scale, T element) throws MagnificationException
```

is treated as:

```java
/**
 * {@inheritDoc}
 *
 * @param <T> {@inheritDoc}
 * @param scale a non-zero number
 * @param element {@inheritDoc}
 * @return {@inheritDoc}
 * @throws MagnificationException {@inheritDoc}
 */
```

`{@inheritDoc}` in a `@param` matches **by position, not by name** -- renaming a parameter in the override does not break it. Markdown and traditional comments may inherit freely from each other.

## Deprecation: tag *and* annotation

```java
/**
 * Sums the values in {@code v}.
 *
 * @deprecated As of 2.0, replaced by {@link #sum(IntStream)}, which
 *     short-circuits on overflow.
 */
@Deprecated(since = "2.0", forRemoval = true)
public int sum(int[] v) { ... }
```

- `@Deprecated` (annotation) is what the **compiler** acts on -- it produces the call-site warning.
- `@deprecated` (tag) is what the **docs** show. "The first sentence should at least tell the user when the API was deprecated and what to use instead"; later sentences explain why (Oracle style guide).
- In **traditional** `/** */` comments, the tag alone historically marked the declaration deprecated. In **Markdown `///` comments the `@Deprecated` annotation must be used**; the tag is ignored without it (Doc Comment Spec).

Always write both.

## `@apiNote` / `@implSpec` / `@implNote`

These split a comment into four boxes ([JDK-8008632](https://bugs.openjdk.org/browse/JDK-8008632), [JEP draft 8068562](https://openjdk.org/jeps/8068562)):

| Box | Normative? | Inherited? | Content |
|-----|-----------|-----------|---------|
| Untagged description | Yes -- the specification | Yes | Behavior every valid implementation must exhibit; pre/postconditions |
| `@apiNote` | No | -- | Commentary, rationale, examples for the **caller** |
| `@implSpec` | Yes | **No** | What a conforming override/default implementation must do; enough for an implementer to decide whether to override |
| `@implNote` | No | **No** | Incidental implementation facts (performance in this JDK) that may change across versions and vendors |

```java
/**
 * Returns a sequential stream with this collection as its source.
 *
 * @implSpec
 * The default implementation creates a sequential {@code Stream} from the
 * collection's {@code Spliterator}.
 *
 * @implNote
 * The returned spliterator reports {@code SIZED}.
 *
 * @apiNote
 * Prefer {@link #parallelStream()} for large collections.
 */
default Stream<E> stream() { ... }
```

Caveat: these are **not** standard doclet tags. Support was implemented via the standard doclet's custom-tag mechanism, so builds must pass them explicitly (JEP draft 8068562):

```text
javadoc -tag 'apiNote:a:API Note:' \
        -tag 'implSpec:a:Implementation Requirements:' \
        -tag 'implNote:a:Implementation Note:'
```

Without those flags the tags render as unknown-tag noise.

## `{@snippet}` -- Example Code (JDK 18+)

`@snippet` was added by [JEP 413](https://openjdk.org/jeps/413), delivered in JDK 18, to replace `{@code}`-in-`<pre>` examples and to let external tools compile and validate the fragments. javadoc itself does not compile them -- that is an explicit non-goal.

Inline form:

```java
/**
 * Reads a line.
 *
 * {@snippet lang = java:
 *     var in = new BufferedReader(new InputStreamReader(System.in));
 *     String line = in.readLine();   // @highlight substring="readLine"
 * }
 */
```

External form -- the snippet lives in a real, compilable file under `snippet-files/` or on `--snippet-path`, which is the version worth using for anything non-trivial:

```java
/**
 * {@snippet file="ShowOptional.java" region="example"}
 */
```

Markup tags live in `//` comments (invisible in output) and apply to the current line, or the next line if the comment ends with `:`. Available: `@start region=` / `@end`, `@highlight`, `@replace`, `@link` (Doc Comment Spec).

Inline snippets cannot contain `*/` and must have balanced braces; external files have neither restriction.

## Markdown `///` Comments (JDK 23+)

[JEP 467](https://openjdk.org/jeps/467) added Markdown documentation comments, delivered in **JDK 23**. Write a run of contiguous `///` lines; the standard doclet parses them as **CommonMark 0.31.2** plus GFM pipe tables plus all Javadoc tags ([Using Markdown Documentation Comments](https://docs.oracle.com/en/java/javase/25/javadoc/using-markdown-documentation-comments.html)).

```java
/// Returns a lease for a [ByteBuffer] with at least the given capacity.
///
/// The buffer is drawn from the pool; see [#release(ByteBuffer)].
///
/// @param capacity the minimum capacity, in bytes
/// @return a lease that must be closed by the caller
public Lease acquire(int capacity) { ... }
```

Why it is better than `/** */`:

- **No `*/` restriction.** Regexes, glob patterns, and `/* */` in example code just work.
- **No `{@code}`/`{@literal}` needed.** Code spans and fenced/indented blocks hold literal text; tags are not interpreted inside them.
- **Links are brackets.** `[element]` is `{@link ...}` (monospace); `[text][element]` is `{@linkplain ...}` (plain font).

```java
/// * a module [java.base/]
/// * a package [java.util]
/// * a class [String]
/// * a field [String#CASE_INSENSITIVE_ORDER]
/// * a method [String#chars()]
/// * with alternative text: [a method][String#chars()]
```

Gotchas:

- A truly blank line (one not starting with `///`) **splits** the comment; all but the last fragment is discarded as a "dangling comment" and silently ignored.
- Square brackets inside a reference must be escaped: `[String#copyValueOf(char\[\])]`.
- Markdown headings start at **level 1** in every context and are re-leveled on output (HTML headings do not work this way -- those start at level 2 for types, level 4 for members).
- Anchors use `##`: `[access mode restrictions][MemoryLayout##access-mode-restrictions]`.
- A reference link in the **first sentence** cannot use a link reference definition declared elsewhere in the comment -- use an inline link there.
- `@Deprecated` the annotation is mandatory (see above).
- Malformed CommonMark produces no error; it renders as literal text. Proofread the output.

## Doclint

`-Xdoclint` is **enabled by default** in `javadoc`; disable with `-Xdoclint:none`. Groups ([javadoc tool spec](https://docs.oracle.com/en/java/javase/25/docs/specs/man/javadoc.html)):

| Group | Severity | Checks |
|-------|----------|--------|
| `missing` | **warning** | Missing comment on a declaration; missing `@param` / `@return` |
| `reference` | error | `@see` / `{@link}` target not found; bad `@param` / `@throws` name |
| `syntax` | error | Unescaped `<`, `>`, `&`; invalid tags |
| `html` | error | Block element inside inline element; unclosed elements |
| `accessibility` | error | Missing `alt` on `<img>`, missing table caption |

Enable selectively per package with `-Xdoclint/package:`, and suppress in source with `@SuppressWarnings("doclint:missing")` (comma-separated group list). Suppression covers errors as well as warnings, and cannot be selectively re-enabled on enclosed declarations.

Under `javac`, doclint relies **solely** on the `-Xdoclint...` options -- it checks nothing unless asked.

## Pressure Resistance Protocol

1. **Third person, always.** "Returns the label." Not "Return the label." Not "This method returns the label."
2. **Summary is a fragment, punctuated like a sentence.** Capital letter, trailing period, no subject.
3. **`@param` / `@return` / `@throws` / `@deprecated`, in that order, never empty.** An empty tag is worse than a missing one -- it satisfies doclint while telling the reader nothing.
4. **Write `@return` even when it duplicates the summary.** Oracle mandates it for findability; `{@return ...}` gets you both from one line.
5. **`@deprecated` tag AND `@Deprecated` annotation.** The tag documents; the annotation warns. Under `///` comments the annotation is required for the tag to count at all.
6. **Let overrides inherit.** Omitting a tag on an `@Override` method is `{@inheritDoc}`. Do not paste the supertype's prose -- it will drift.
7. **Use `@implSpec` when a method is overridable.** Subclassers cannot decide whether to override without knowing what the default implementation guarantees.
8. **Prefer external `{@snippet}` files.** They are the only examples that a build can compile.

## Red Flags

| Anti-pattern | Problem | Fix |
|--------------|---------|-----|
| `/** Get the label. */` | 2nd person imperative | `/** Gets the label. */` |
| `/** This method returns the ID. */` | States the subject; Google 7.2 forbids | `/** Returns the ID. */` |
| `/** @return the customer ID */` with no summary | Summary is the only text in index tables | Add the summary, or use `{@return the customer ID}` |
| `@param b The buffer.` | Tag text is a phrase: lowercase, no period | `@param b the buffer` |
| `@return` with empty text | Passes a shallow lint, informs nobody | Describe the value, including special cases |
| `@param <code>off</code> ...` | HTML in the name; javadoc warns since 1.4 | Plain name; javadoc formats it |
| `@deprecated` with no replacement | Reader is stuck | Name the successor with `{@link}` and say since when |
| `@deprecated` tag without `@Deprecated` | No compiler warning; ignored entirely in `///` comments | Add the annotation |
| Copy-pasted supertype Javadoc on an override | Diverges the moment the supertype changes | Omit the tags, or `{@inheritDoc}` |
| `@implSpec` documented as normative caller contract | Not inherited; callers may never see it | Caller contract goes in the untagged description |
| `@apiNote`/`@implSpec` without matching `-tag` flags | Renders as unknown-tag noise | Add the `-tag` options to the javadoc invocation |
| `<pre>{@code ...}</pre>` for a nontrivial example | Never compiled; rots | External `{@snippet file=...}` |
| A blank line inside a `///` block | Splits the comment; the top half is discarded silently | Every line, including blanks, starts with `///` |
| `-Xdoclint:none` in the build | Turns off the only automatic check | Enable at least `missing,reference,syntax` |

## Common Rationalizations

### "The getter is obvious, Google Style says I can skip it."

Reality: 7.3.1 exempts self-explanatory members "only when there truly is nothing worth saying beyond restating the name," and explicitly warns the exception cannot justify "omitting relevant information that a typical reader might need to know." A getter named `canonicalName` still needs a comment if the reader would not know what "canonical" means here.

### "It's an override, inheritance handles it."

Reality: inheritance handles the parts you *omit*. If the override narrows the contract -- fewer exceptions, a stricter range, a different null policy -- the inherited text is now wrong. Document the delta and let the rest inherit.

### "I'll put it in `@implNote`."

Reality: `@implNote` is explicitly non-normative and **not inherited**, meaning callers of the interface never see it. If a subclasser must honor it, it belongs in `@implSpec`; if a caller must rely on it, it belongs in the untagged description.

### "`@return` just repeats the first sentence."

Oracle's style guide anticipated that and requires it anyway: an explicit `@return` "makes the return value easier to find quickly," and gives you somewhere to state the special cases (what comes back for an out-of-range argument, an empty input, end of stream). `{@return ...}` removes the duplication entirely.

### "Markdown comments are cosmetic."

Reality: they remove three real footguns -- the `*/` termination problem, the `{@code}` escaping tax, and hand-written `{@link}` boilerplate. The one real cost is that a stray blank line silently deletes half your comment.

## Quick Reference

| Tag / form | Use for |
|------------|---------|
| `/** ... */` | Traditional doc comment |
| `/// ...` | Markdown doc comment (JDK 23+, CommonMark) |
| `@param name desc` | One per formal parameter, in declaration order |
| `@param <T> desc` | One per type parameter |
| `@return desc` | Every non-`void` method; omit for `void` and constructors |
| `{@return desc}` | Summary + `@return` in one (JDK 16+) |
| `@throws Ex desc` | One per exception in the `throws` clause |
| `@deprecated desc` | Docs half of deprecation -- pair with `@Deprecated` |
| `@since 21` | Release the element was introduced |
| `@see reference` | "See Also" entry |
| `{@link ref label}` | Inline link, monospace |
| `{@linkplain ref label}` | Inline link, plain font |
| `{@inheritDoc}` / `{@inheritDoc S}` | Pull description or tag text from a supertype |
| `{@code text}` | Code font, no HTML/tag interpretation |
| `{@literal text}` | Same, without code font |
| `{@summary text}` | Explicit summary (start of description only) |
| `{@value #FIELD}` | Value of a compile-time constant |
| `{@snippet ...}` | Example code, inline or from a file (JDK 18+) |
| `@apiNote` / `@implSpec` / `@implNote` | Caller note / override contract / incidental detail |
| `@hidden` | Exclude an element from generated docs (JDK 9+) |
| `[String#chars()]` | Markdown link to a program element |
| `-Xdoclint:missing,reference,syntax` | Turn the checks on in CI |

## The Bottom Line

Javadoc is a specification format, so write it as one: a third-person fragment summary, complete `@param`/`@return`/`@throws` in the prescribed order, `@implSpec` on anything overridable, both halves of a deprecation, and examples in snippet files the build can compile. Leave doclint on. Java's mood convention is the opposite of Python's -- check which language you are in before you type "Return".
