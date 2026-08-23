## Overview

C# doc comments are XML embedded in `///` lines. The compiler "combines the structure of the C# code with the text of the comments into a single XML document" and "verifies that the comments match the API signatures for relevant tags" ([XML documentation comments](https://learn.microsoft.com/en-us/dotnet/csharp/language-reference/xmldoc/)). That verification is the point: `<param>` names, `cref` targets, and XML well-formedness are all checked at build time.

The XML is not metadata. "The compiler doesn't include them in the compiled assembly, so they're not accessible through reflection" -- they ship as a sidecar `.xml` file that IntelliSense, DocFX, and Sandcastle consume (ibid.).

## The Iron Rule

**Every publicly visible type and member gets at least a `<summary>`. Set `GenerateDocumentationFile` so CS1591 fails you when one is missing. Write complete sentences ending in full stops. Let the type system carry nullability -- document behavior, not `string?`.**

## Detection

```csharp
// RED FLAGS

public sealed class RetryPolicy { }          // public, no <summary>

/// <summary>Gets the name.</summary>        // tautological; adds nothing to `string Name`
public string Name { get; }

/// <summary>Sends the request.</summary>
public Task<Response> SendAsync(Request req, CancellationToken ct)
                                             // no <param>, no <returns>, no <exception>

/// <param name="reqest">The request.</param> // typo -> compiler warning
public Task<Response> SendAsync(Request req)

/// <summary>See https://example.com/docs</summary>   // bare URL; use <see href="...">

/// <summary>Returns null if not found.</summary>     // the `?` already said that
public Customer? Find(int id)

/// <summary>Uses List<T> internally.</summary>       // raw angle brackets: malformed XML
```

## Setup: `GenerateDocumentationFile` and CS1591

```xml
<PropertyGroup>
  <GenerateDocumentationFile>true</GenerateDocumentationFile>
  <TreatWarningsAsErrors>true</TreatWarningsAsErrors>
</PropertyGroup>
```

"You set either the **GenerateDocumentationFile** or **DocumentationFile** option... When you enable this option, the compiler generates the [CS1591](https://learn.microsoft.com/en-us/dotnet/csharp/language-reference/compiler-messages/cs1591) warning for any publicly visible member declared in your project without XML documentation comments" ([XML documentation comments](https://learn.microsoft.com/en-us/dotnet/csharp/language-reference/xmldoc/)).

CS1591 is a level-4 warning: *"Missing XML comment for publicly visible type or member 'Type_or_Member'."* This is the only place in the C# toolchain where docs are enforced -- without the property, nothing checks anything.

Scope the exemption rather than turning it off globally:

```csharp
#pragma warning disable 1591   // generated file; no docs by design
// ... generated code ...
#pragma warning restore 1591
```

Blanket `<NoWarn>1591</NoWarn>` in the `.csproj` disables the check for the whole project and is the usual reason a library ships with half its API undocumented.

Other compiler checks that come with the feature (ibid.):

- `<param>` -- "the compiler verifies that the parameter exists and that all parameters are described in the documentation. If the verification fails, the compiler issues a warning."
- `cref` -- "the compiler verifies that this code element exists... The compiler respects any `using` directives when it looks for a type described in the `cref` attribute."
- Well-formedness -- "If the XML isn't well formed, the compiler generates a warning."

## Structure

Microsoft's own recommendations ([Recommended XML tags](https://learn.microsoft.com/en-us/dotnet/csharp/language-reference/xmldoc/recommended-tags)):

- "For the sake of consistency, document all publicly visible types and their public members."
- "At a bare minimum, types and their members should have a `<summary>` tag."
- "Write documentation text using complete sentences that end with full stops."
- Documenting private members is possible but "exposes the inner (potentially confidential) workings of your library."

> **Contradiction alert.** C# wants **complete sentences**; Google Java Style wants a **fragment** ("a noun phrase or verb phrase, not a complete sentence"). Both use third-person declarative -- "Returns the ..." -- which is the opposite of Python's and Go's imperative/name-first style. When moving between the two, keep the mood and change the sentence-completeness.

```csharp
/// <summary>
/// Sends an HTTP request and returns the response body as a stream.
/// </summary>
/// <remarks>
/// Retries idempotent requests up to three times with exponential backoff.
/// The returned stream is owned by the caller and must be disposed.
/// </remarks>
/// <param name="request">The request to send. Its content is buffered before the first attempt.</param>
/// <param name="cancellationToken">A token that cancels the operation, including in-flight retries.</param>
/// <returns>The response body. Never empty; a zero-length body yields an empty stream.</returns>
/// <exception cref="HttpRequestException">The request failed after all retries.</exception>
/// <exception cref="OperationCanceledException"><paramref name="cancellationToken"/> was cancelled.</exception>
/// <seealso cref="SendAndBufferAsync(HttpRequestMessage, CancellationToken)"/>
public Task<Stream> SendAsync(HttpRequestMessage request, CancellationToken cancellationToken)
```

`<summary>` describes the member and drives IntelliSense; `<remarks>` "add[s] information about a type or a type member, supplementing the information specified with `<summary>`" and "can include more lengthy explanations" (Recommended tags). Keep `<summary>` to the one-line answer and push the caveats into `<remarks>`.

## Nullable Reference Types Replace Null Prose

With `<Nullable>enable</Nullable>`, the annotation *is* the documentation: "Every reference type variable is *non-nullable* by default. Append `?` to declare a *nullable* reference type... The `?` informs the compiler of your design intent" ([Nullable reference types](https://learn.microsoft.com/en-us/dotnet/csharp/nullable-references)).

```csharp
// DON'T -- prose duplicating the type
/// <summary>Finds a customer. Returns null if no customer matches.</summary>
/// <param name="name">The name. Can be null.</param>
public Customer? Find(string? name)

// DO -- the signature already carries nullability; document the behavior
/// <summary>Finds the customer with the given name.</summary>
/// <param name="name">The name to match, compared case-insensitively.</param>
/// <returns>The matching customer, or <see langword="null"/> when no customer matches.</returns>
public Customer? Find(string? name)
```

The `<returns>` line still earns its place: `Customer?` says null is possible, not *when*. What you should stop writing is "can be null" on a `string?` parameter.

For contracts the annotation cannot express -- "null only when the argument is null", "not-null when this returns true" -- use the nullable analysis attributes instead of prose, because the compiler enforces them and prose does not:

```csharp
public static bool IsPresent([NotNullWhen(true)] string? value) =>
    !string.IsNullOrEmpty(value);
```

"As of .NET 5, all .NET runtime APIs are annotated" (ibid.). Note the analysis "doesn't trace into the bodies of methods" -- if a method communicates null-state to callers, the attribute on the signature is the only channel.

## References: `cref`, `href`, `langword`, `paramref`

```csharp
/// <see cref="Stream.Dispose"/>                     <!-- compiler-checked code reference -->
/// <see cref="IDictionary{TKey, TValue}"/>          <!-- braces read as angle brackets -->
/// <see href="https://learn.microsoft.com">Docs</see>  <!-- external URL -->
/// <see langword="null"/>                           <!-- language keyword -->
/// <paramref name="request"/>                       <!-- refers to a parameter -->
/// <typeparamref name="TKey"/>                      <!-- refers to a type parameter -->
/// <seealso cref="SendAndBufferAsync"/>             <!-- "See Also" section -->
```

- `cref` "means 'code reference'"; the compiler checks the target exists and rewrites it to the canonical ID string. **Use `cref` for code, `href` for URLs** -- "`cref` is designed for code references and doesn't create clickable links for external URLs" (Recommended tags).
- Generic references may escape (`cref="List&lt;T&gt;"`) or use braces (`cref="List{T}"`); "the compiler parses the braces as angle brackets to make the documentation comment less cumbersome" ([XML documentation comments](https://learn.microsoft.com/en-us/dotnet/csharp/language-reference/xmldoc/)).
- `<seealso>` "can't [be] nest[ed]... inside the `summary` tag."
- Angle brackets in prose must be encoded: `/// This property always returns a value &lt; 1.`

## `<inheritdoc>`

```csharp
/// <inheritdoc/>
public override string ToString() => ...;

/// <inheritdoc cref="ISender.SendAsync(Request, CancellationToken)"/>
public Task<Response> SendAsync(Request r, CancellationToken ct) => ...;

/// <inheritdoc path="/summary|/param"/>   <!-- XPath filter -->
```

"By using `inheritdoc`, you eliminate unwanted copying and pasting of duplicate XML comments and automatically keep XML comments synchronized. When you add the `<inheritdoc>` tag to a type, all members inherit the comments as well." Inherited tags "don't override already defined tags on the current member," so you can add a `<remarks>` and inherit the rest (Recommended tags).

The trap: **Visual Studio's automatic inheritance is IDE-only.** "This automatic inheritance only applies within the Visual Studio IDE and doesn't affect the XML documentation file generated by the compiler... For public APIs in libraries that you distribute, explicitly use the `<inheritdoc>` tag or provide complete documentation" (ibid.). A member that looks documented in the editor can ship with an empty entry in the `.xml`.

Beyond overrides and interface implementations, the canonical use is keeping an async overload in sync with its synchronous twin.

## `<include>`

Pulls documentation from a separate XML file via XPath, so the docs can be versioned and reviewed apart from the code. "The .NET Runtime team uses the `<include>` tag extensively in its documentation" (Recommended tags).

```csharp
/// <returns>This comes from triple slash comments.</returns>
/// <include file="MyAssembly.xml" path="doc/members/member[@name='M:MyNamespace.MyType.MyMethod']/*" />
public int MyMethod(int p) => p;
```

The include file mirrors the compiler's own output shape:

```xml
<?xml version="1.0"?>
<doc>
  <members>
    <member name="M:MyNamespace.MyType.MyMethod">
      <param name="p">This is the description of p. It comes from the included file.</param>
      <summary>This is the summary of MyMethod. It comes from the included file.</summary>
    </member>
  </members>
</doc>
```

Inline tags and included tags merge. Recommended form: `<include file='filename' path='tagpath[@name="id"]' />`, with `filename` relative to the source file and single-quoted. Reach for it when the same prose is shared across many overloads or a localization pipeline owns the text; for ordinary code it is more indirection than it is worth.

## Formatting Tags

| Tag | Use |
|-----|-----|
| `<para>` | Double-spaced paragraph inside `<summary>`/`<remarks>`/`<returns>` |
| `<br/>` | Single line break |
| `<c>` | Inline code span |
| `<code>` | Multi-line code block |
| `<example>` | Usage example; normally wraps a `<code>` block |
| `<list type="bullet\|number\|table">` | List or definition table with `<listheader>`, `<item>`, `<term>`, `<description>` |
| `<value>` | What a property represents (properties only; `<summary>` says what it does) |

```csharp
/// <example>
/// This shows how to increment an integer.
/// <code>
///     var index = 5;
///     index++;
/// </code>
/// </example>
```

For DocFX, "using `CDATA` sections for markdown make[s] writing it more convenient. Tools such as [docfx](https://dotnet.github.io/docfx/) process the markdown text in `CDATA` sections" (Recommended tags).

## Generics

```csharp
/// <summary>Represents a read-through cache.</summary>
/// <typeparam name="TKey">The key type. Must be suitable as a dictionary key.</typeparam>
/// <typeparam name="TValue">The cached value type.</typeparam>
public sealed class Cache<TKey, TValue> where TKey : notnull
{
    /// <summary>
    /// Returns the value for <paramref name="key"/>, loading it if absent.
    /// </summary>
    /// <typeparam name="TState">State threaded through to the loader.</typeparam>
    public TValue GetOrAdd<TState>(TKey key, TState state, Func<TKey, TState, TValue> load) { ... }
}
```

`<typeparam>` is compiler-verified (marked `*` in the recommended-tags list), same as `<param>`. Note the constraint (`where TKey : notnull`) already documents itself -- describe the *role* of the type parameter, not its constraints.

## Delimiters

`///` is the form used by "the documentation examples and C# project templates." `/** */` is supported with fiddly common-prefix stripping rules and no upside ([XML documentation comments](https://learn.microsoft.com/en-us/dotnet/csharp/language-reference/xmldoc/)). Use `///`.

One trap: "If you write comments by using the single line XML comment delimiter, `///`, but don't include any tags, the compiler adds the text of those comments to the XML output file. However, the output doesn't include XML elements such as `<summary>`. Most tools that consume XML comments (including Visual Studio IntelliSense) don't read these comments" (ibid.). A bare `/// Does the thing.` is invisible to IntelliSense.

Partial types: "documentation information is concatenated into a single entry for each type. If both declarations of a partial member have documentation comments, the comments on the implementing declaration are written to the output XML" (Recommended tags).

Namespaces cannot be documented -- "You can't apply documentation comments to a namespace" -- though they can be `cref`-referenced.

## Pressure Resistance Protocol

1. **Turn on `GenerateDocumentationFile` in every shipped library.** Nothing else enforces documentation in C#.
2. **Scope CS1591 suppressions to generated code with `#pragma`.** Project-wide `NoWarn` deletes the check.
3. **`<summary>` on everything public; `<remarks>` for the rest.** Keep the tooltip's first line answerable at a glance.
4. **Let `?` say "nullable"; say *when* in `<returns>`.** Prose that restates the annotation is noise that goes stale.
5. **Encode contracts the annotation can't express as attributes, not prose.** `[NotNullWhen(true)]` is checked; "returns true if non-null" is not.
6. **`cref` for code, `href` for URLs.** A `cref` breaks the build on rename; a URL in a `<summary>` rots silently.
7. **Write `<inheritdoc/>` explicitly.** Visual Studio's implicit inheritance never reaches the shipped `.xml`.
8. **Document exceptions the caller can act on.** `<exception>` is the only place the throw contract is visible from the signature.

## Red Flags

| Anti-pattern | Problem | Fix |
|--------------|---------|-----|
| Public type with no `<summary>` | Empty IntelliSense tooltip; empty docs page | Add it; let CS1591 catch the rest |
| `<NoWarn>1591</NoWarn>` project-wide | Silently disables the only enforcement | `#pragma` around generated regions only |
| `/// Does the thing.` (no tags) | Not read by IntelliSense or most tools | Wrap in `<summary>` |
| `<summary>Gets the name.</summary>` on `string Name` | Restates the signature | Say why it exists, or drop it and let the name speak |
| "Can be null" on a `string?` parameter | Duplicates the annotation | Delete; document *when* null occurs, in `<returns>` |
| `<param>` name typo | Compiler warning, and the doc is orphaned | Fix -- the compiler already told you |
| Missing `<param>` on one of several parameters | Compiler warns on partial coverage | Document all or none |
| `<see cref="https://..."/>` | `cref` produces no clickable link for URLs | `<see href="https://...">text</see>` |
| Raw `<` or `List<T>` in prose | Malformed XML; compiler warns | `&lt;`, or `cref="List{T}"` |
| `<seealso>` nested inside `<summary>` | Not permitted | Move it to a sibling tag |
| Relying on VS implicit inheritance | Shipped `.xml` has an empty entry | Explicit `<inheritdoc/>` |
| `<exception>` omitted on a throwing method | Callers discover the throw at runtime | One `<exception cref="..."/>` per actionable failure |
| `<summary>` running to a paragraph | Only the summary shows in tooltips and indexes | One sentence; the rest goes in `<remarks>` |

## Common Rationalizations

### "IntelliSense already shows the signature."

Reality: the signature shows shape, not contract. It cannot show that the returned stream must be disposed by the caller, that the method retries, that a zero-length body is legal, or which exception survives a cancellation. Every one of those belongs in the comment.

### "Nullable reference types made docs unnecessary."

Reality: they made *null prose* unnecessary. `Customer?` tells the caller a null is possible; only `<returns>` tells them it means "no match" rather than "lookup failed." Delete the "can be null" lines, keep the behavioral ones.

### "It's an internal library."

Reality: `GenerateDocumentationFile` costs one line and CS1591 costs nothing once the API is documented, and the `.xml` drives your own team's IntelliSense. If some namespace genuinely does not warrant docs, exempt it with a scoped `#pragma` and a reason -- visibly, not by deleting the check.

### "`<inheritdoc/>` shows up in the editor without me writing it."

Reality: that behavior "only applies within the Visual Studio IDE and doesn't affect the XML documentation file generated by the compiler." Consumers of your NuGet package get nothing. Write the tag.

### "I'll add `<example>` later."

Reality: `<example>` blocks are never compiled by anything in the C# toolchain, so a late-written one is both wrong and unverifiable. Write it while the API is fresh, and prefer linking a real sample project over an inline `<code>` block that nothing builds.

## Quick Reference

| Tag | Use for |
|-----|---------|
| `<summary>` | What the type or member is. Required minimum; shown in IntelliSense |
| `<remarks>` | Supplemental detail, caveats, longer explanation |
| `<param name="x">` | One per parameter. Compiler-verified |
| `<paramref name="x"/>` | Refer to a parameter from prose |
| `<typeparam name="T">` | One per type parameter. Compiler-verified |
| `<typeparamref name="T"/>` | Refer to a type parameter from prose |
| `<returns>` | The return value, including special cases |
| `<value>` | What a property represents |
| `<exception cref="T">` | An exception the member can throw. Compiler-verified |
| `<see cref="M"/>` | Inline link to a code element |
| `<see href="url">` | Inline link to a web page |
| `<see langword="null"/>` | A language keyword |
| `<seealso cref="M"/>` | "See Also" entry (not nestable in `<summary>`) |
| `<inheritdoc [cref] [path]/>` | Inherit docs from a base, interface, or named member |
| `<include file path/>` | Pull docs from an external XML file via XPath |
| `<example>` / `<code>` / `<c>` | Usage example / code block / inline code |
| `<para>` / `<br/>` | Double-spaced paragraph / single line break |
| `<list type="bullet\|number\|table">` | List or definition table |
| `GenerateDocumentationFile` | Emit the `.xml` and enable CS1591 |
| `#pragma warning disable 1591` | Scoped exemption for generated code |

## The Bottom Line

C#'s compiler is the only doc-comment tooling that checks parameter names and code references for you -- but only after you set `GenerateDocumentationFile`. Turn it on, write a complete-sentence `<summary>` on everything public, let the `?` and the nullable attributes carry nullability while the prose carries behavior, use `cref` for code and `href` for links, and write `<inheritdoc/>` explicitly so the shipped `.xml` is not emptier than your editor suggests.
