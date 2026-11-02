## Overview

Python docstrings are runtime objects, not comments. A string literal that is the first statement in a module, function, class, or method becomes that object's `__doc__` ([PEP 257](https://peps.python.org/pep-0257/)). `help()`, `pydoc`, IDE hovers, and Sphinx all read the same string.

PEP 257 fixes the *skeleton* (quotes, blank lines, summary line, indentation). It does not define sections. Sections come from one of three competing conventions -- Google, NumPy, or reST/Sphinx -- and the choice is per project, never per file.

## The Iron Rule

**Every public module, class, function, and method gets a docstring in `"""triple double quotes"""`, opening with a one-line summary that fits on one line and ends in a period. Pick one section convention per project and enforce it with `ruff` (`D` rules). Types live in annotations, not in the docstring.**

Source: [PEP 8, Documentation Strings](https://peps.python.org/pep-0008/#documentation-strings) -- "Write docstrings for all public modules, functions, classes, and methods."

## Detection

```python
# RED FLAGS

def get_user(id):                      # <- public, zero docstring (D103)
    ...

def fetch(url):
    '''Fetches a url.'''               # <- single quotes (D300); tautological

def parse(s: str) -> Config:
    """parse(s) -> Config"""           # <- signature reiteration (D402)

def load(path: str) -> bytes:
    """
    Args:
        path (str): the path.          # <- type duplicated from annotation
    """                                # <- no summary line at all (D205/D212)

def size(self) -> int:
    """This method returns the size."""  # <- "This" (D404); descriptive under
                                         #    a convention that wants imperative
```

## Quotes, Blank Lines, and Indentation (PEP 257)

These rules are format-agnostic -- they hold under Google, NumPy, and reST alike. All from [PEP 257](https://peps.python.org/pep-0257/).

- "For consistency, always use `"""triple double quotes"""` around docstrings." Use `r"""raw triple double quotes"""` if the text contains backslashes (ruff `D300`, `D301`).
- One-liners: closing quotes on the same line, **no blank line before or after**.
- Multi-line: summary line, **blank line**, then the elaboration. "Unless the entire docstring fits on a line, place the closing quotes on a line by themselves."
- Class docstrings are followed by a blank line, to offset them from the first method.
- Indentation is stripped by tooling: the first line's leading whitespace is dropped, and a uniform indent "equal to the minimum indentation of all non-blank lines after the first line" is removed from the rest. Relative indentation survives. See [`inspect.cleandoc`](https://docs.python.org/3/library/inspect.html#inspect.cleandoc).

```python
# DO -- one-liner
def kos_root() -> str:
    """Return the pathname of the KOS root directory."""

# DO -- multi-line
def complex(real: float = 0.0, imag: float = 0.0) -> Complex:
    """Form a complex number.

    Only the real part is required; the imaginary part defaults to zero.
    """

# DON'T -- blank line after a one-liner (D202), closing quotes hoisted (D209)
def kos_root() -> str:
    """Return the pathname of the KOS root directory."""

    ...
```

`inspect.getdoc()` also **inherits** the docstring from the MRO for classes, methods, properties, and descriptors when the subclass has none (Python 3.5+) -- so an override with no docstring is not undocumented at runtime. ([docs](https://docs.python.org/3/library/inspect.html#inspect.getdoc))

## Imperative Mood -- and Where the Rule Comes From

[PEP 257](https://peps.python.org/pep-0257/) says the summary "prescribes the function or method's effect as a command (\"Do this\", \"Return that\"), not as a description; e.g. don't write \"Returns the pathname ...\"."

```python
# DO
"""Return the mean of the given values."""

# DON'T
"""Returns the mean of the given values."""
```

But this is convention-scoped, not universal:

- Ruff enforces it as [`D401` non-imperative-mood](https://docs.astral.sh/ruff/rules/non-imperative-mood/), which is **on under `pep257` and `numpy`, and off under `google`** ([convention lists](https://docs.astral.sh/ruff/settings/#lint_pydocstyle_convention)).
- The [Google Python Style Guide 3.8.3](https://google.github.io/styleguide/pyguide.html) explicitly allows either: `"""Fetches rows from a Bigtable."""` or `"""Fetch rows from a Bigtable."""` -- "but the style should be consistent within a file."

So: imperative is the PEP 257 default; descriptive is legal if the project has chosen Google. Never mix within a file.

Two exceptions that survive every convention:

- **Properties** are documented like attributes, not actions: `"""The Bigtable path."""`, not `"""Returns the Bigtable path."""` (Google 3.8.3; ruff `D421` flags a property docstring starting with a verb).
- The summary must not be a signature restatement -- `"""parse(s) -> Config"""` (ruff `D402`). Signature-style docstrings exist only for C builtins where introspection fails.

## The Three Formats

Pick one per project. Napoleon's own docs call the choice "largely aesthetic" but warn the styles "should not be mixed." ([sphinx.ext.napoleon](https://www.sphinx-doc.org/en/master/usage/extensions/napoleon.html))

### Google (indentation-delimited)

Most common in application code; least vertical space. Section headings end in a colon; content is hang-indented 2 or 4 spaces. ([Google 3.8.3](https://google.github.io/styleguide/pyguide.html))

```python
def fetch_rows(
    table: smalltable.Table,
    keys: Sequence[bytes | str],
    require_all_keys: bool = False,
) -> Mapping[bytes, tuple[str, ...]]:
    """Fetches rows from a Smalltable.

    Retrieves rows pertaining to the given keys from the Table instance
    represented by table_handle. String keys will be UTF-8 encoded.

    Args:
        table: An open smalltable.Table instance.
        keys: A sequence of strings representing the key of each table
          row to fetch.
        require_all_keys: If True, only rows with values set for all keys
          will be returned.

    Returns:
        A dict mapping keys to the corresponding table row data fetched.
        Returned keys are always bytes.

    Raises:
        IOError: An error occurred accessing the smalltable.
    """
```

### NumPy (underline-delimited)

Standard in scientific/array libraries. Verbose but scans well for long parameter lists. Headings are underlined with hyphens; entries are `name : type`. ([numpydoc](https://numpydoc.readthedocs.io/en/latest/format.html))

```python
def histogram(a, bins=10, *, density=False):
    """Compute the histogram of a dataset.

    Parameters
    ----------
    a : array_like
        Input data.
    bins : int or sequence of scalars, optional
        Number of equal-width bins.
    density : bool, default False
        If True, normalize to a probability density.

    Returns
    -------
    hist : ndarray
        The values of the histogram.
    bin_edges : ndarray
        The bin edges, of length ``len(hist) + 1``.

    See Also
    --------
    bincount, digitize
    """
```

numpydoc rules worth knowing: the short summary "does not use variable names or the function name"; "the type of each return value is always required"; a `Receives` section requires a `Yields` section; `Raises` is "only for errors that are non-obvious or have a large chance of getting raised"; lines are kept to 75 characters; sections must appear in a fixed order (ruff `D420` checks this).

### reST / Sphinx info field lists

The native Sphinx form -- no preprocessor needed, and types hyperlink automatically. Dense to read, which is precisely why Napoleon exists. ([Sphinx info field lists](https://www.sphinx-doc.org/en/master/usage/domains/python.html#info-field-lists))

```python
def send_message(sender, recipient, body, priority=1):
    """Send a message to a recipient.

    :param str sender: The person sending the message.
    :param recipient: The recipient of the message.
    :param priority: Message priority, 1-5.
    :type priority: int or None
    :return: The message id.
    :rtype: int
    :raises ValueError: If the body exceeds 160 characters.
    """
```

Recognized fields: `param`/`parameter`/`arg`/`argument`/`key`/`keyword`, `type`, `raises`/`raise`/`except`/`exception`, `var`/`ivar`/`cvar`, `vartype`, `returns`/`return`, `rtype`, and `meta` (e.g. `:meta private:`, which autodoc uses for member filtering). Use it when you are writing Sphinx docs directly and want `:meta:` and automatic type cross-references; otherwise Google or NumPy plus Napoleon reads better.

Note: reST is "*a* standard, not *the only* standard" for docstring markup, and adoption is optional ([PEP 287](https://peps.python.org/pep-0287/), status Active/Informational).

## Do Not Repeat Types in the Docstring

Annotations ([PEP 484](https://peps.python.org/pep-0484/) for signatures, [PEP 526](https://peps.python.org/pep-0526/) for variables and attributes) are what mypy, pyright, and IDEs actually read. A type written in the docstring is unchecked prose that will drift.

Google 3.8.3 states it directly: the argument description "should include required type(s) if the code does not contain a corresponding type annotation." Napoleon likewise supports annotating the signature and omitting types from the docstring, and `napoleon_attr_annotations` (default `True`) pulls attribute types from PEP 526 annotations. Ruff has `lint.pydocstyle.ignore-var-parameters` for the `*args`/`**kwargs` case.

```python
# DO -- annotation is the single source of truth
def load(path: Path, *, retries: int = 3) -> bytes:
    """Read a file, retrying on transient IO errors.

    Args:
        path: File to read.
        retries: Attempts before giving up. Must be positive.

    Returns:
        The file contents.
    """

# DON'T -- type stated twice, checked once
def load(path: Path, *, retries: int = 3) -> bytes:
    """Read a file.

    Args:
        path (Path): File to read.
        retries (int): Attempts before giving up.

    Returns:
        bytes: The file contents.
    """
```

Exception: NumPy-style docstrings in the scientific ecosystem still carry types by convention (`array_like`, `int or tuple of int`) because those describe accepted *duck types* broader than any annotation. Follow the surrounding project.

There is no blessed alternative to prose parameter docs: [PEP 727](https://peps.python.org/pep-0727/) (`typing.Doc` inside `Annotated`) is **Withdrawn** -- "The reception of this PEP was mostly negative, with concerns raised about verbosity and readability." Do not adopt it.

## Returns, Yields, Raises, Attributes

- **Returns:** Omit entirely if the function returns only `None`, or if the summary already begins "Return"/"Returns" and says enough (Google 3.8.3). For multiple values, describe the tuple: "A tuple `(mat_a, mat_b)`, where `mat_a` is ...".
- **Yields:** For generators, document "the object returned by `next()`", not the generator object.
- **Raises:** Document only exceptions that are part of the interface. Google 3.8.3: do not document exceptions raised when the API contract is violated, "because this would paradoxically make behavior under violation of the API part of the API." numpydoc: only non-obvious errors, or ones with a high chance of being raised.
- **Attributes:** Public attributes go in an `Attributes:` section on the *class* docstring, formatted like `Args:`. Properties are excluded -- they carry their own docstrings.

## Modules, Packages, Classes, `__init__`, Dunders, Overloads

**Module** -- first statement in the file; describes contents and usage, with a summary line, blank line, description, and optionally a "Typical usage example". A **package** is documented by the `__init__.py` module docstring; PEP 257 asks it to list the exported modules and subpackages. Ruff: `D100` (module), `D104` (package).

**Class** -- the summary says what an *instance represents*, not that it is a class. For exceptions, describe what the exception *represents*, not the context it occurs in (Google 3.8.4):

```python
# DO
class CheeseShopAddress:
    """The address of a cheese shop."""

class OutOfCheeseError(Exception):
    """No more cheese is available."""

# DON'T
class CheeseShopAddress:
    """Class that describes the address of a cheese shop."""

class OutOfCheeseError(Exception):
    """Raised when no more cheese is available."""
```

**`__init__` vs class docstring** -- the two conventions genuinely disagree, so follow the project:

| Convention | Constructor args documented on |
|---|---|
| PEP 257 / Google | `__init__` docstring (`Args:` there); class docstring gets `Attributes:` |
| NumPy | **Class** docstring -- its `Parameters` section describes `__init__`'s arguments; a separate `__init__` docstring is optional |

Ruff encodes this: `D107` (missing `__init__` docstring) is enabled under `pep257` and `google`, and **disabled under `numpy`**.

**Dunder methods** -- `D105` covers magic methods. A `__repr__` or `__eq__` doing the obvious thing needs nothing; document a dunder only when its semantics are surprising.

**Overloads** -- put the docstring on the implementation, never on the `@overload` stubs ([`D418`](https://docs.astral.sh/ruff/rules/overload-with-docstring/)). `__doc__` resolves to the implementation's either way, so the stub copies are pure duplication. Does not apply to `.pyi` stub files, which have no implementation.

```python
# DO
@overload
def factorial(n: int) -> int: ...
@overload
def factorial(n: float) -> float: ...

def factorial(n):
    """Return the factorial of n."""
```

**Overriding methods** -- PEP 257 asks you to say that behavior is inherited and summarize the differences, using "override" (replaces without calling super) and "extend" (calls super, then adds) precisely. Google 3.8.3.1: a method decorated with `@typing.override` needs no docstring "unless the overriding method's behavior materially refines the base method's contract"; `"""See base class."""` is then noise.

**Scripts** -- PEP 257: a script's module docstring "should be usable as its 'usage' message", covering the function, command-line syntax, environment variables, and files.

## When NOT to Write a Docstring

Docstring coverage is not a goal; a docstring that adds nothing is a broken window with quotes around it.

- **Non-public helpers.** PEP 8: docstrings are "not necessary for non-public methods, but you should have a comment that describes what the method does," placed after the `def` line. Google 3.8.3 requires one only when the function is part of the public API, of nontrivial size, or has non-obvious logic.
- **Test modules.** Google 3.8.2.1: module docstrings for test files "are not required. They should be included only when there is additional information that can be provided" -- how to run it, unusual setup, external dependencies. `"""Tests for foo.bar."""` is an anti-example: "Docstrings that do not provide any new information should not be used."
- **`@overload` stubs** (`D418`, above).
- **Trivial `@override` methods** with `@typing.override` present.
- **Implementation detail.** Google 3.8.3: subtleties that do not affect callers "are better expressed as comments alongside the code than within the function's docstring." A docstring should let a reader "write a call to the function without reading the function's code" -- nothing more.

An empty docstring is worse than none: ruff flags `D419`.

## Doctests

Doctests in an `Examples` section are executable documentation -- they cannot silently rot.

```python
def add(a: int, b: int) -> int:
    """Return the sum of a and b.

    Examples
    --------
    >>> add(2, 3)
    5
    """
```

numpydoc is emphatic that examples are "optional, but very strongly encouraged" and equally emphatic about their scope: "This section is meant to illustrate usage, not to provide a testing framework" -- real tests belong in `tests/`. Continuation lines use `... `; nondeterministic output gets a trailing `#random`; imports other than the ambient project alias must be shown explicitly.

## Tooling Enforcement

Enable the `D` rules and pin a convention. Enabling a convention **disables every rule not in it**, so the workflow is: set the convention, then selectively add or ignore on top ([ruff settings](https://docs.astral.sh/ruff/settings/#lint_pydocstyle_convention)).

```toml
[tool.ruff.lint]
select = ["D"]
ignore = ["D417"]  # on top of google: don't require every param documented

[tool.ruff.lint.pydocstyle]
convention = "google"
```

Rules excluded by each convention (verbatim from the ruff settings page):

| Convention | Excludes |
|---|---|
| `pep257` | D203, D212, D213, D214, D215, D404, D405, D406, D407, D408, D409, D410, D411, D413, D415, D416, D417, D420 |
| `numpy` | D107, D203, D212, D213, D402, D413, D415, D416, D417 |
| `google` | D203, D204, D213, D215, D400, D401, D404, D406, D407, D408, D409, D413 |

`D203`/`D211` and `D212`/`D213` are mutually exclusive pairs -- every convention drops one of each, which is why selecting bare `D` without a convention produces unsatisfiable warnings.

Other settings: `lint.pydocstyle.property-decorators` (extra decorators treated as properties for `D401`/`D421`), `ignore-decorators` (skip checks on decorated functions), `ignore-var-parameters` (don't require `*args`/`**kwargs` in `Args:`).

## Pressure Resistance Protocol

1. **Pick the convention once, in `pyproject.toml`.** Format arguments are not worth having twice; the linter settles them.
2. **Summary line first, always.** One physical line, ends in a period, then a blank line before anything else.
3. **Imperative unless the project is Google.** `"Return the ..."`, not `"Returns the ..."` -- and never mix moods inside one file.
4. **Types in annotations, prose in docstrings.** If you typed it in the signature, do not retype it in `Args:`.
5. **Document the contract, not the implementation.** Enough to write the call without reading the body; side effects and mutated arguments are contract.
6. **Skip the docstring when it would be filler.** Private helper, trivial override, `@overload` stub, `"""Tests for foo.bar."""` -- write nothing rather than nothing-in-quotes.
7. **Prefer doctests to prose examples** for small, deterministic APIs; they get executed.

## Red Flags

| Anti-pattern | Problem | Fix |
|---|---|---|
| Missing docstring on public API | `D100`-`D104`; `help()` shows nothing | Add a summary line |
| `'''single quotes'''` | `D300` | Use `"""` |
| Backslashes in a plain docstring | `D301`; escapes get interpreted | Use `r"""` |
| No blank line after the summary | `D205`; indexers take the whole blob | Blank line, then the body |
| Closing `"""` on the last text line | `D209` | Own line for multi-line docstrings |
| `"""Returns the mean."""` under pep257/numpy | `D401` non-imperative | `"""Return the mean."""` |
| `"""This function ..."""` | `D404`; wastes the summary line | Start with the verb or the noun |
| `"""func(a, b) -> int"""` | `D402` signature reiteration | Describe behavior; the signature is visible |
| `path (str):` alongside `path: str` | Type duplicated, unchecked, will drift | Drop the type from the docstring |
| Docstring on each `@overload` | `D418`; duplicated, `__doc__` ignores them | One docstring on the implementation |
| `"""See base class."""` on an `@override` | Pure noise | Delete it |
| `"""Tests for foo.bar."""` | Adds no information (Google 3.8.2.1) | Delete, or say how to run/update it |
| Documenting exceptions from contract violations | Makes misuse part of the API | Document only interface exceptions |
| `"""Class that represents a User."""` | Restates that a class is a class | `"""A registered user."""` |
| `select = ["D"]` with no convention | D203/D211 and D212/D213 conflict | Set `convention` |

## Common Rationalizations

### "The type annotations already document it."

Reality: annotations give the shape, never the contract. `def withdraw(account: Account, amount: Decimal) -> Decimal` does not say whether `amount` may be negative, whether `account` is mutated, what the return value means, or which exception a shortfall raises. Those are exactly what the docstring is for.

### "I'll use NumPy style here, Google style there."

Reality: Napoleon parses both, so it appears to work -- until you enable a ruff convention and half the file lights up, or a reader hits `Parameters\n----------` in one function and `Args:` in the next. Napoleon's own docs say the styles "should not be mixed."

### "It's a private helper, no one reads it."

Reality: PEP 8 agrees you can skip the docstring -- and then asks for a comment after the `def` line instead. The exemption is from the *format*, not from explaining the function.

### "Imperative vs descriptive is bikeshedding."

Reality: it is, which is why it should cost zero ongoing thought. Set `convention` in `pyproject.toml` and let `D401` decide it once and for all.

### "Docstring coverage is at 100%, we're good."

Reality: `"""Tests for foo.bar."""`, `"""See base class."""`, and `"""Initializes the instance."""` all count toward coverage and inform no one. `D419` catches empty docstrings; nothing catches vacuous ones except review.

## Quick Reference

| Form | Use for |
|---|---|
| `"""One line."""` | Obvious cases; closing quotes on the same line |
| `"""Summary."""` (multi-line) | Everything else; blank line after the summary |
| `r"""..."""` | Any docstring containing backslashes |
| `Args:` / `Parameters\n----------` / `:param x:` | Parameters (Google / NumPy / reST) |
| `Returns:` / `Yields:` | Return value; the `next()` object for generators |
| `Raises:` | Interface exceptions only |
| `Attributes:` | Public class attributes (on the class docstring) |
| `Examples` + `>>>` | Executable usage illustration |
| `.. deprecated::` | numpydoc deprecation (a directive, not a section header) |
| `:meta private:` | Hide a member from Sphinx autodoc |
| `@overload` + `...` | No docstring on stubs; one on the implementation |
| `convention = "google"` | Pin the section format for ruff |

## The Bottom Line

PEP 257 governs the skeleton -- triple double quotes, a one-line summary, a blank line, closing quotes on their own line -- and that part is not negotiable. Everything past the summary line is convention, so pick Google, NumPy, or reST once, pin it with `ruff`'s `convention` setting, and stop relitigating it. Then write only what a caller cannot get from the signature: contract, side effects, error semantics, and a worked example. Types belong in annotations, where a type checker will keep them honest.
