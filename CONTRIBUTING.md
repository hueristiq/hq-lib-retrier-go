# Contributing

Contributions are welcome and encouraged!

Whether you’re fixing a bug, adding a new feature, improving documentation, or proposing changes to these guidelines, your efforts help improve the project for everyone. Please review the following guidelines before contributing. If you have suggestions to improve these guidelines, feel free to update this file and submit a pull request.

- [I have a question...](#have-a-question)
- [I found a bug...](#found-a-bug)
- [I have a feature request...](#have-a-feature-request)
- [I have a contribution to share...](#ready-to-contribute)
- [Code Style](#code-style)
  - [Documentation](#documentation)
  - [Errors](#errors)
  - [Configuration and package state](#configuration-and-package-state)
  - [Interfaces](#interfaces)
  - [Naming](#naming)
  - [Package organisation](#package-organisation)
  - [Tests](#tests)
  - [Examples](#examples)
  - [Formatting and imports](#formatting-and-imports)
  - [Enforcement summary](#enforcement-summary)
  - [Review checklist](#review-checklist)

## Have a Question?

If you have any questions about how to use `hq-lib-retrier-go`, please use other communication channels (such as discussion forums or community chats) instead of opening a GitHub issue.

> [!CAUTION]
> Our issue tracker is reserved for bug reports and feature requests. Questions that are not directly related to a bug or feature request may be closed to keep the issue tracker focused.

## Found a Bug?

If you've identified a bug in `hq-lib-retrier-go`, please [create an issue](#create-an-issue). If you are able to fix the bug, feel free to [submit a pull request](#create-a-pull-request) with your solution. Reference the bug issue in your PR description so that it can be linked automatically.

## Have a Feature Request?

If you have an idea for a new feature or an enhancement:

- Document Your Request:

Start by [submitting an issue](#create-an-issue) that outlines your proposed feature.

- Discuss and Refine:

Engage in the discussion within the issue. This helps clarify the scope and requirements of the feature.

- Contribute Code:

If you decide to implement the feature, [submit a pull request](#create-a-pull-request) that references the issue (for example, using `fixes #<issue-number>`) so that the issue is automatically closed once your PR is merged.

## Ready to Contribute

### Create an issue

- Search Existing Issues:

Before submitting an issue, search our [issue tracker](https://github.com/hueristiq/hq-lib-retrier-go/issues) to see if the issue has already been submitted.

- Submit an Issue:

Assuming no existing issues exist, please [open a new issue](https://github.com/hueristiq/hq-lib-retrier-go/issues/new), ensure you include required information when submitting the issue to ensure we can quickly reproduce your issue. We may have additional questions and will communicate through the GitHub issue, so please respond back to our questions to help reproduce and resolve the issue as quickly as possible.

### Create a Pull Request

Pull requests should target the `dev` branch. Please also reference the issue from the description of the pull request using [special keyword syntax](https://help.github.com/articles/closing-issues-via-commit-messages/) to auto close the issue when the PR is merged. For example, include the phrase `fixes #14` in the PR description to have issue `#14` auto close.

## Code Style

These conventions define the house style of the hueristiq Go libraries. They
exist so consumers get a predictable, documented API and contributors know
exactly what a change must satisfy.

The rules are enforced in three layers:

1. The repo-pinned `golangci-lint` configuration covers everything that can be
   checked mechanically.
2. `go test -race ./...` runs the tests and every godoc example, and
   `go build ./examples/...` keeps the runnable programs compiling.
3. The review checklist at the end covers the rules a linter cannot express.

When a rule is review-only, the enforcement summary says so explicitly.

### Documentation

Every exported symbol carries a doc comment that begins with the symbol name.
Write full sentences, end each comment with a period, and use the standard
`Parameters:`, `Returns:`, `Fields:`, and `Methods:` blocks so readers can find
information in the same place every time.

Package documentation opens with a one-sentence summary, followed by
sentence-case headings. Root packages of public modules include
`# Entry points`, `# Configuration`, and `# Concurrency`. Leaf packages include
the sections that apply; if a section would be empty, omit it rather than
writing a placeholder.

Functions and methods document their contract in this order:

- `Type Parameters:` for generic parameters, if any.
- `Parameters:` when the function takes arguments.
- `Returns:` when it returns values.

Use one `  - name (type): description` bullet per parameter and return value.
The receiver is not a parameter. Return values are named in the signature.
Descriptions are lowercase and end with a period. Boolean results are described
as predicates (`whether ...`), not as "true/false".

```go
// Parse converts raw into a URL using the parser's configured settings.
//
// Parameters:
//   - raw (string): the raw URL string.
//
// Returns:
//   - parsed (*URL): the parsed URL.
//   - err (error): non-nil when parsing fails.
func Parse(raw string) (parsed *URL, err error) {
	// ...
}
```

This is not acceptable:

```go
// parses a url
func Parse(raw string) (*URL, error) {
	// ...
}
```

Document struct fields in a `Fields:` block, not as inline comments:

```go
// Options controls the HTTP client.
//
// Fields:
//   - Timeout (time.Duration): the request timeout; zero uses the default.
//   - Retries (int): the number of retries after the first attempt.
type Options struct {
	Timeout time.Duration
	Retries int
}
```

Do not repeat the same information inline:

```go
type Options struct {
	Timeout time.Duration // request timeout
	Retries int           // number of retries
}
```

Document interface methods under `Methods:` when the interface has more than
one method, or with a per-method comment when the interface is short:

```go
// Searcher performs directory searches.
//
// Methods:
//   - Search(searchRequest *SearchRequest) (*SearchResult, error): executes a
//     search and returns the accumulated result.
type Searcher interface {
	Search(searchRequest *SearchRequest) (result *SearchResult, err error)
}
```

Grouped constants may share one block comment that documents the set. Add an
individual comment when a constant's meaning is not obvious from its name.

### Errors

Error strings are lowercase, carry no trailing punctuation, and name the
producing function: `package.Func: message`. Sentinel errors are package-level
vars named `ErrX` with plain messages. Wrap underlying errors with `%w`, and
use `errors.Is`/`errors.As` rather than comparing strings or types. Wrap context
causes with the entry point's name.

```go
var ErrNotFound = errors.New("not found")

func Load(id string) (*User, error) {
	user, err := fetch(id)
	if err != nil {
		return nil, fmt.Errorf("store.Load(%q): %w", id, err)
	}

	return user, nil
}
```

Do not capitalise the message, add punctuation, or discard the cause:

```go
var ErrNotFound = errors.New("Not Found.")

func Load(id string) (*User, error) {
	user, err := fetch(id)
	if err != nil {
		return nil, fmt.Errorf("load failed: %s", err)
	}

	return user, nil
}
```

`errorlint` checks `%w` and `errors.Is`/`errors.As`; `errname` checks sentinel
naming; `revive` checks the mechanical error-string rules. The
`package.Func:` prefix is a review rule.

### Configuration and package state

Functional options are named `OptionFunc`, ignore nil, and normalize invalid
values. The option target is an exported `Options` (or domain) type only when
the package documents third-party options; otherwise the target is sealed and
the package docs say so.

Sealed options are the default because they keep the public surface small:

```go
type OptionFunc func(*config)

type config struct {
	timeout time.Duration
}

func WithTimeout(timeout time.Duration) OptionFunc {
	return func(cfg *config) {
		if timeout > 0 {
			cfg.timeout = timeout
		}
	}
}
```

Export the target only when callers are expected to write their own options:

```go
// Options holds the client configuration. Callers may write custom
// OptionFunc values.
type Options struct {
	Timeout time.Duration
}

type OptionFunc func(*Options)
```

No mutable package-level configuration. Settings flow through constructors,
functional options, or exported value structs. A process-wide knob is allowed
only when it is documented as such and reviewed explicitly; prefer an option.
Do not add exported maps, slices, or variables that callers can mutate.

`gochecknoglobals` is deliberately not enabled because sentinel errors,
immutable lookup tables, and generated data are legitimate package globals.
The no-mutable-configuration rule is enforced in review.

### Interfaces

Keep interfaces small and consumer-owned. Prefer a narrow role interface over
one large interface; if a type exposes several capabilities, define separate
interfaces and let callers accept the one they need.

A concrete type that implements a non-trivial interface declares a compile-time
assertion:

```go
var _ Searcher = (*Conn)(nil)
```

Do not export an interface solely to return an unexported implementation, and
do not add methods to an interface speculatively. `interfacebloat` and `iface`
check size and pollution; assertions are a review rule.

### Naming

Use MixedCaps with uniform-case initialisms (`URL`, `ID`, `TLS`, `HTTP`):
`NewHTTPClient`, not `NewHttpClient`; `userID`, not `userId`.

Accessors are not prefixed with `Get`: use `Config()`, not `GetConfig()`.
Package and directory names are singular, lowercase, and functional. Never use
`utils`, `helpers`, `models`, or `types` as package names.

### Package organisation

A package is flat unless a subpackage is independently consumable. Use
`internal/` for private implementation, root `internal/` only for helpers
shared by all versioned subpackages, and `vN/internal/` for version-local
helpers.

Protocol packages use the facade + internal engine pattern: the public package
exposes thin types (aliases where useful), the engine lives under `internal/`,
and engine types are never re-exported. Do not create a `pkg/` directory.
Generators live under `cmd/`; data only an implementation needs lives under
`internal/`.

```text
hq-lib-example-go/
├── doc.go
├── example.go
├── internal/
│   └── engine/
├── examples/
│   └── basic/main.go
└── cmd/
    └── gen/main.go
```

### Tests

API-level tests live in `_test` packages. Tests that need unexported behavior
stay in the package and carry a file-level directive with the reason:

```go
//nolint:testpackage // tests exercise unexported frame trimming.
package errors
```

Use table-driven subtests and `t.Parallel()` where safe:

```go
func TestParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "scheme", raw: "https://example.com", want: "https"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parsed, err := Parse(tt.raw)
			if err != nil {
				t.Fatalf("Parse(%q): %v", tt.raw, err)
			}

			if parsed.Scheme != tt.want {
				t.Fatalf("scheme = %q, want %q", parsed.Scheme, tt.want)
			}
		})
	}
}
```

Run `go test -race ./...`. Add fuzz targets for parsers and benchmarks for hot
paths. `testpackage`, `paralleltest`, and `tparallel` check the mechanical
parts; the choice between `_test` and in-package is reviewed.

### Examples

Every major exported behavior gets a godoc `Example` with a deterministic
`// Output:` block:

```go
func ExampleParse() {
	parsed, err := Parse("https://example.com")
	if err != nil {
		panic(err)
	}

	fmt.Println(parsed.Host)
	// Output: example.com
}
```

Add an `examples/<topic>/main.go` program when the API needs setup or several
calls. Examples must be deterministic and offline: fixed inputs, `httptest` for
HTTP, fixed keys, and no wall-clock or random output.

Mirror the runnable examples and their expected output in
`docs/03-usage.md` (and `docs/04-use-cases.md` where relevant). Because
`docs/` is gitignored in these repositories, also link to `examples/` from the
tracked README so readers can find the programs.

### Formatting and imports

Code is `gofmt` and `goimports` clean. Group imports as standard library,
third-party, then `github.com/hueristiq`:

```go
import (
	"context"
	"fmt"

	"github.com/stretchr/testify/assert"

	hqgoexample "github.com/hueristiq/hq-lib-example-go"
)
```

Use the `hqgo*` aliases for sibling modules (`importas` enforces them). Run
`make go-lint` before submitting; CI runs the same pinned configuration.

### Enforcement summary

| Area | Enforced by | Where |
| --- | --- | --- |
| Doc comments start with the symbol name, package docs, duplicate docs | `godoclint`, `revive` | `make go-lint`, CI |
| Comments end with a period | `godot` | `make go-lint`, CI |
| Error strings, sentinel naming | `revive`, `errname` | `make go-lint`, CI |
| `%w` wrapping, `errors.Is`/`errors.As` | `errorlint` | `make go-lint`, CI |
| `_test` package layout | `testpackage` | `make go-lint`, CI |
| `t.Parallel` and test parallelism | `paralleltest`, `tparallel` | `make go-lint`, CI |
| Godoc examples have output | `testableexamples` | `go test ./...`, CI |
| Imports and aliases | `gofmt`, `goimports`, `importas` | `make go-lint`, CI |
| No printing in library code | `forbidigo` | `make go-lint`, CI |
| Interface size and pollution | `interfacebloat`, `iface` | `make go-lint`, CI |
| Correctness and security | `staticcheck`, `gocritic`, `govet`, `gosec` | `make go-lint`, CI |
| Option-target seal/export | Review | Pull request |
| No mutable package-level config | Review | Pull request |
| Package layout and naming | Review | Pull request |
| Compile-time interface assertions | Review | Pull request |
| Error `package.Func:` prefix | Review | Pull request |
| Examples compile and match docs | Review, `go test`, `go build` | CI |

### Review checklist

- [ ] Every exported symbol is documented and the comment starts with its name.
- [ ] Package docs include the applicable `# Entry points`, `# Configuration`,
      and `# Concurrency` sections.
- [ ] Functions document parameters and named returns in the standard order.
- [ ] Struct fields use a `Fields:` block; interface methods use `Methods:` or
      per-method comments.
- [ ] Errors are lowercase, unprefixed only when reviewed, wrapped with `%w`,
      and inspectable with `errors.Is`/`errors.As`.
- [ ] Options are `OptionFunc`, ignore nil, normalize invalid values, and use a
      sealed or documented exported target.
- [ ] Interfaces are small, consumer-owned, and have compile-time assertions
      where appropriate.
- [ ] API tests use `_test` packages; in-package tests carry the
      `testpackage` reason directive.
- [ ] Every major behavior has a godoc example with output and, where useful, a
      runnable `examples/<topic>/main.go`.
- [ ] `make go-lint`, `go test -race ./...`, and `go build ./examples/...`
      pass.
