## Overview

PHP docblocks (`/** ... */`) are read by three different audiences with different needs: static analyzers (PHPStan, Psalm), IDEs, and doc generators (phpDocumentor). Today the first audience dominates -- since PHP 7.0/7.4/8.0 gave the language real parameter, return, and property types, the docblock's job shifted from *declaring* types to expressing the types PHP cannot express: array shapes, generics, `list<T>`, `class-string<T>`, integer ranges, and literal unions.

There is no accepted standard here. **PSR-5 (PHPDoc) and PSR-19 (PHPDoc Tags) are DRAFTS** -- they live in the `proposed/` directory of php-fig/fig-standards and have never been voted in ([PSR-5](https://github.com/php-fig/fig-standards/blob/master/proposed/phpdoc.md), [PSR-19](https://github.com/php-fig/fig-standards/blob/master/proposed/phpdoc-tags.md)). The de facto standards are the [phpDocumentor guide](https://docs.phpdoc.org/guide/guides/docblocks.html) plus whatever [PHPStan](https://phpstan.org/writing-php-code/phpdoc-types) and [Psalm](https://psalm.dev/docs/annotating_code/supported_annotations/) accept. Write for the analyzer; the generated docs follow.

## The Iron Rule

**Never write a docblock tag that restates a native type declaration. Write the docblock only when it carries information the signature cannot: a description, a narrowed type (`array<int, User>`, `non-empty-string`, `int<0, max>`), `@throws`, or a deprecation.**

Symfony states this as a rule: add PHPDoc blocks "only when they add relevant information that does not duplicate the name, native type declaration or context" ([symfony.com/doc/current/contributing/code/standards.html](https://symfony.com/doc/current/contributing/code/standards.html)). Laravel states it as a permission: "When the `@param` or `@return` attributes are redundant due to the use of native types, they can be removed" ([laravel.com/docs/13.x/contributions](https://laravel.com/docs/13.x/contributions)).

## Detection

```php
// RED FLAGS

/**
 * @param string $name          <- pure duplication of the native type
 * @param int $age              <- same
 * @return bool                 <- same
 */
public function register(string $name, int $age): bool

/**
 * Gets the user.               <- tautological; adds zero information
 */
public function getUser(): User

/**
 * @return array                <- native type already says `array`; says nothing about contents
 */
public function attachments(): array

/**
 * @author  Jane Doe            <- git blame owns this
 * @version 1.4.2               <- the VCS tag owns this
 * @package App\Service         <- the namespace owns this
 */
class Mailer

/** @var Foo $foo */            <- inline @var used to paper over a bad upstream type
$foo = $container->get('foo');
```

## Structure and Placement

A DocBlock is a `/**` DocComment holding, in order: **summary**, optional **description**, then **tags** ([phpDocumentor](https://docs.phpdoc.org/guide/guides/docblocks.html)). Order is enforced -- prose placed after the tags is parsed as part of the last tag.

```php
/**
 * Charges the customer's default payment method.
 *
 * Retries idempotently on network failure. The caller is responsible for
 * ensuring the customer has a payment method on file; this method does not
 * check.
 *
 * @throws PaymentDeclined when the issuer rejects the charge
 */
public function charge(Customer $customer, Money $amount): Receipt
```

Rules:

- `/**` opens the block; `//` and plain `/* */` are **not** docblocks ([PSR-5 draft](https://github.com/php-fig/fig-standards/blob/master/proposed/phpdoc.md)).
- The block directly precedes the structural element (class, interface, trait, enum, function, method, property, constant).
- The summary is a short headline and "cannot hold formatting or inline tags"; separate it from the description with a blank `*` line.
- Descriptions support Markdown; summaries do not.
- **Docblock comes before attributes**: PER Coding Style requires that "the comment block MUST come first, followed by any attributes, followed by the structure itself," with no blank lines between them ([PER Coding Style](https://www.php-fig.org/per/coding-style/)).
- Do not use one-line docblocks on classes, methods, or functions -- not even for a single tag (Symfony standards).

```php
// DO
/**
 * Handles the inbound webhook.
 */
#[AsController]
#[Route('/webhook', methods: ['POST'])]
public function __invoke(Request $request): Response

// DON'T -- attributes above the docblock, one-line block
#[AsController]
/** @return Response */
public function __invoke(Request $request): Response
```

## Native Types First

PHP 8 covers most of what `@param`/`@return`/`@var` used to declare. Delete the tag and write the type in the signature.

```php
// DO
/**
 * Transforms the input given as the first argument.
 *
 * @param $options an options collection to be used within the transformation
 *
 * @throws \RuntimeException when an invalid option is provided
 */
private function transformText(bool|string $dummy, array $options = []): ?string

// DON'T
/**
 * @param bool|string $dummy
 * @param array $options
 * @return string|null
 * @throws \RuntimeException
 */
private function transformText(bool|string $dummy, array $options = []): ?string
```

Note the Symfony form: `@param $name description` -- name and description, **no type**, because the signature already has it. Also from Symfony: omit `@return` entirely for `void` methods; group same-type tags together and separate different tag groups by one blank line.

Native constructs that make docblocks unnecessary: property types (7.4), union types, `static` return, `never`, `readonly` properties, promoted constructor properties, and enums. A `readonly` promoted property with a native type needs no `@var` at all.

## When the Docblock Still Earns Its Place

Everything below is invisible to the PHP type system. These are the tags worth writing.

```php
/** @return array<int, Attachment> */          // element types
/** @return list<Attachment> */                 // sequential int keys from 0
/** @return non-empty-list<string> */
/** @param array{host: string, port: int} $cfg */          // array shape
/** @param array{host: string, port?: int} $cfg */         // optional key
/** @return array{0: string, 1: int} */                    // tuple: array{string, int}
/** @param class-string<Command> $class */                 // FQCN of a subtype
/** @param int<1, 100> $percent */                         // integer range
/** @param positive-int $limit */
/** @param non-empty-string $key */
/** @param 'asc'|'desc' $direction */                      // literal union
/** @param key-of<self::WHEELS> $type */
/** @param value-of<Suit> $suit */
/** @return ($id is null ? null : User) */                 // conditional return
/** @return iterable<string, Row> */                       // key and value of a foreach
```

Syntax and semantics: [phpstan.org/writing-php-code/phpdoc-types](https://phpstan.org/writing-php-code/phpdoc-types). `list<T>` means "arrays with sequential integer keys starting at 0" -- prefer it over `array<int, T>` whenever that is the actual contract, because it is the stronger claim.

Reusable shapes get a local alias instead of being copy-pasted:

```php
/**
 * @phpstan-type UserAddress array{street: string, city: string, zip: string}
 */
final class User {}

/**
 * @phpstan-import-type UserAddress from User
 */
final class Mailer
{
    /** @param UserAddress $address */
    public function send(array $address): void {}
}
```

Psalm's equivalents are `@psalm-type` / `@psalm-import-type`.

## Generics: @template

Neither PSR-5 nor PSR-19 defines `@template` -- generics are entirely a PHPStan/Psalm invention, and are the single strongest reason a modern PHP codebase writes docblocks at all.

```php
/**
 * @template T of Model
 */
final class Repository
{
    /** @param class-string<T> $class */
    public function __construct(private string $class) {}

    /** @return T|null */
    public function find(int $id): ?Model {}

    /** @return list<T> */
    public function all(): array {}
}

/** @extends Repository<User> */
final class UserRepository extends Repository {}
```

- Bounds: `@template T of Animal`. Defaults: `@template T = string` ([PHPStan](https://phpstan.org/writing-php-code/phpdocs-basics)).
- Variance: `@template-covariant T` (never in a parameter position), `@template-contravariant T` (never in a return position). Symfony's standards allow generics but explicitly disallow `@template-covariant`.
- Binding a parent's parameters: `@extends`, `@implements`, `@use` (the `@use` goes above the in-body `use` statement). Every declared parameter must be supplied.
- Template names must not collide with existing class names.
- Psalm's covariance caveat: a covariant collection used as function input still errors, because that implies mutation. Pair `@template-covariant` with `@psalm-immutable` and return new instances ([psalm.dev](https://psalm.dev/docs/annotating_code/templated_annotations/)).

## @throws

`@throws` is not decoration -- PHPStan uses it for "precise analysis of try-catch-finally" and for checked-exception rules ([phpstan.org/writing-php-code/phpdocs-basics](https://phpstan.org/writing-php-code/phpdocs-basics)). The documented type must be a subtype of `Throwable`, and the PSR-19 draft recommends a tag per distinct throw site so callers see the full failure surface.

```php
// DO -- name the condition, not just the class
/**
 * @throws OrderNotFound when no order matches $id
 * @throws PaymentDeclined when the issuer rejects the charge
 */
public function refund(OrderId $id): Refund

// DON'T -- bare, uninformative, and unhelpfully broad
/** @throws \Exception */
public function refund(OrderId $id): Refund
```

## @var on Properties and Inline

On a property, `@var` is worth writing only when it narrows the native type:

```php
// DO
/** @var array<string, Handler> */
private array $handlers = [];

// DON'T
/** @var Logger */
private Logger $logger;
```

Inline `@var` is a last resort. PHPStan: it "should be used only as a last resort," because PHPStan "always trusts it" -- a wrong annotation silently corrupts the analysis below it -- and because it "needs to be repeated above all usages of the symbol which leads to repetition in the codebase" ([phpdocs-basics](https://phpstan.org/writing-php-code/phpdocs-basics)). Fix the type at the source (stub files, generics, a return-type extension) or use a runtime `assert()` that actually fails.

```php
// DON'T
/** @var UserRepository $repo */
$repo = $container->get(UserRepository::class);

// DO
$repo = $container->get(UserRepository::class);
assert($repo instanceof UserRepository);
```

Psalm has `@psalm-ignore-var` precisely for the case where an `@var` was written for IDE autocompletion and is now weakening the checker.

## @deprecated, @internal, @api

```php
/**
 * @deprecated 3.4 use {@see SumContext} instead, which respects cancellation
 */
public function sum(array $v): int
```

- `@deprecated` optionally carries a version plus the reason; when a replacement exists the PSR-19 draft recommends pairing it with a `@see` to the successor. PHPStan, Psalm, and IDEs all surface it at call sites.
- `@internal` marks an element used only inside the owning library. Per the PSR-19 draft, maintainers may treat breaking changes to it as "exempt from semantic versioning," analyzers may warn on external use, and generators hide it by default. Psalm adds `@psalm-internal Namespace\Foo` to scope the restriction.
- `@api` marks the opposite -- the supported surface. Useful in libraries where "public" and "supported" are not the same set.

## @see and @link

`@see` points at an FQSEN or URI; `@link` points at an absolute URI only. Both have inline forms usable inside a description.

```php
/**
 * Normalizes the payload before dispatch.
 *
 * The wire format is described in {@see PayloadSchema::VERSION_2}.
 *
 * @see Dispatcher::dispatch()
 * @link https://example.com/docs/wire-format
 */
```

Address a class member by appending `::` plus the member name. Prefer `@see` for anything inside the project and `@link` for the outside world.

## Legacy Noise

| Tag | Verdict |
|-----|---------|
| `@author` | Git owns authorship. Symfony permits it as an optional contact hint that the core team may remove on request; most projects should not add it. |
| `@version` | The VCS tag owns this. The PSR-19 draft itself says it should not record when something was introduced or changed -- that is `@since`. In practice, drop both from application code. |
| `@copyright`, `@license` | File-header only, and only in libraries that need the notice. |
| `@package` / `@subpackage` | Superseded by namespaces. The PSR-19 draft concedes that where logical and functional groupings coincide, using the tag is discouraged "to avoid maintenance burden" -- with PSR-4 autoloading they always coincide. |
| File-level docblock | PER Coding Style defines *where* one goes if present (after `<?php`, before `declare`), not that you need one. Add it only for a license header. |
| `@inheritDoc` | Redundant as a whole-block replacement -- analyzers inherit `@param`/`@return`/`@throws` automatically when the child omits them. Use the inline `{@inheritDoc}` only to *extend* a parent description. |
| `{@inheritdoc}` on one line | Explicitly banned by Symfony's standards. |

## Attributes Are Not a Docblock Replacement

`#[...]` attributes are "structured, machine-readable metadata ... inspected at runtime via the Reflection API" ([php.net](https://www.php.net/manual/en/language.attributes.overview.php)). They are a *runtime configuration* mechanism -- routing, ORM mapping, DI wiring. They replace **annotation** libraries (Doctrine Annotations), not documentation.

```php
// DO -- attribute configures behavior; docblock documents contract and types
/**
 * Rows matching the active subscription window.
 *
 * @return list<Subscription>
 */
#[Route('/subscriptions', methods: ['GET'])]
public function list(): array
```

There is no attribute for `array<int, User>`, no attribute PHPStan reads as a generic bound, and no attribute that renders as prose in an IDE tooltip. Migrating Doctrine annotations to attributes does not empty your docblocks.

## Vendor Prefixes

PHPStan and Psalm both accept prefixed variants (`@phpstan-param`, `@psalm-return`, ...) and prefer them over the plain tag when both are present. PHPStan's stated rationale: "IDEs and other PHP tools might not understand the advanced types that PHPStan takes advantage of," so keep a plain tag for other tools and put the advanced syntax in the prefixed one ([phpdocs-basics](https://phpstan.org/writing-php-code/phpdocs-basics)).

```php
/**
 * @param array $config
 * @phpstan-param array{host: string, port: int<1, 65535>} $config
 */
```

Use the prefix when (a) the syntax is analyzer-specific, or (b) you need to override what a plain tag says for one analyzer. Otherwise write the plain tag -- both analyzers read it, and so does the IDE.

## Pressure Resistance Protocol

1. **Write the native type first, then ask whether a docblock adds anything.** If the answer is "it repeats the signature," delete it.
2. **Never write a bare `array`.** `array<K, V>`, `list<T>`, or an `array{...}` shape -- always.
3. **Prefer `list<T>` over `array<int, T>`** when the keys really are sequential. The stronger claim catches more bugs.
4. **`@throws` names conditions, not just classes.** "when the issuer rejects the charge" is the useful half.
5. **Inline `@var` is a bug report against your types.** Fix the source or `assert()`; do not annotate over it.
6. **Run the analyzer.** A docblock nothing verifies rots exactly as fast as a comment. PHPStan level 6+ or Psalm is what keeps these true.
7. **Use `@deprecated`, not a prose "don't use this."** Only the tag reaches call sites.

## Red Flags

| Anti-pattern | Problem | Fix |
|--------------|---------|-----|
| `@param string $x` over `string $x` | Pure duplication; drifts when the signature changes | Delete the tag, or keep name + description only |
| `@return array` | Says nothing the signature did not | `list<T>`, `array<K, V>`, or `array{...}` |
| `/** Gets the user. */` on `getUser()` | Tautology | Delete it, or document *why*/edge cases |
| `@throws \Exception` | Too broad to act on | Name the concrete class and the condition |
| Inline `@var` above every usage | PHPStan trusts it blindly; repeated everywhere | Fix at the source, or `assert()` |
| `@author` / `@version` in app code | Git and tags already know; goes stale immediately | Remove |
| `@package App\Foo` | Duplicates the PSR-4 namespace | Remove |
| One-line `/** {@inheritdoc} */` | Adds nothing; banned by Symfony standards | Delete the whole block |
| Attribute above the docblock | Violates PER ordering | Docblock, then attributes, then the declaration |
| Docblock with no analyzer running | Unverified, therefore untrusted | Add PHPStan/Psalm to CI |

## Common Rationalizations

### "PSR-5 says to do it this way."

PSR-5 and PSR-19 are drafts in `proposed/`; neither was ever accepted, and PSR-19 does not even define `@template`. Where a draft and PHPStan/Psalm disagree, the analyzer wins -- it is the thing actually reading your code.

### "The `@param` tags document the parameters."

A `@param` with only a type documents nothing a reader could not get from the signature, and it silently lies the moment someone changes the signature and not the block. A `@param` with a *description* documents something. Write that one.

### "I'll add the array shape later."

Later the array has three call sites making different assumptions and you no longer know which is authoritative. The shape is knowable exactly once: while you are writing the function.

### "We generate docs, so we need every tag."

phpDocumentor renders native types perfectly well. Duplicating them in tags produces identical output from twice the source.

### "Attributes replaced docblocks in PHP 8."

Attributes replaced *annotations* -- runtime metadata parsed out of comments. Types, prose, and `@throws` never had an attribute equivalent, and still do not.

## Quick Reference

| Tag | Use for |
|-----|---------|
| `@param $name description` | Parameter description when the native type is already declared |
| `@param array<int, T> $x` | Narrowing an `array`/`iterable` parameter |
| `@return list<T>` \| `array{...}` \| `class-string<T>` | Narrowing beyond the native return type |
| `@var array<string, T>` | Property whose native type is too loose |
| `@throws Fqcn when ...` | Each distinct failure mode |
| `@template T of Bound` | Declare a generic parameter |
| `@extends`/`@implements`/`@use` | Bind a parent's generic parameters |
| `@phpstan-type` / `@phpstan-import-type` | Named, reusable array shape |
| `@deprecated x.y use {@see Other}` | Mark for removal; surfaced at call sites |
| `@internal` / `@psalm-internal Ns` | Outside the BC promise |
| `@api` | Explicitly supported surface |
| `@see Fqsen::member()` / `@link https://` | Cross-reference in-project / external |
| `@property`, `@property-read`, `@method`, `@mixin` | Magic `__get`/`__set`/`__call` surfaces |
| `@phpstan-*` / `@psalm-*` | Analyzer-specific syntax an IDE would choke on |

## The Bottom Line

PHP's type declarations took over the docblock's original job. What is left is what the type system cannot say -- shapes, generics, ranges, failure modes -- plus prose explaining *why*. Write those, run PHPStan or Psalm so they stay honest, and delete everything that merely echoes the signature.
