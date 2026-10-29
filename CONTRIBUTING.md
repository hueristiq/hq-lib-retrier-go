# Contributing

Contributions are welcome and encouraged!

Whether you’re fixing a bug, adding a new feature, improving documentation, or proposing changes to these guidelines, your efforts help improve the project for everyone. Please review the following guidelines before contributing. If you have suggestions to improve these guidelines, feel free to update this file and submit a pull request.

- [I have a question...](#have-a-question)
- [I found a bug...](#found-a-bug)
- [I have a feature request...](#have-a-feature-request)
- [I have a contribution to share...](#ready-to-contribute)

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

This package follows the shared code-style conventions of the hueristiq Go libraries:

- **Documentation.** Every exported symbol carries a doc comment that begins with the symbol name. Package docs use sentence-case headings and include `# Entry points`, `# Configuration`, and `# Concurrency` sections. Functions document generics under `Type Parameters:`, then `Parameters:`, then `Returns:`, using one `  - name (type): description` bullet per parameter and result (the receiver is not a parameter) and named return parameters. Descriptions are lowercase and boolean results use `true`/`false`. Document every parameter and result: include a `Parameters:` block when the function or method takes arguments and a `Returns:` block when it returns values. Omit a block when the list is empty rather than writing a `none.` placeholder. Document struct fields under a `Fields:` block and interface methods under a `Methods:` block, one `  - Name (type): description` bullet per entry, with lowercase descriptions ending in a period. Do not repeat the entries as inline field comments. Grouped constants may share one block comment that documents the set.
- **Errors.** Error strings are lowercase, carry no trailing punctuation, and are prefixed with the producing function (for example `package.Func: message`). Sentinel errors live in package-level vars with plain messages; underlying errors are wrapped with `%w`. Context causes are wrapped with the entry point's name.
- **Configuration.** Functional options use an exported parameter type (`Options` or a domain type), ignore nil, and let callers write custom `OptionFunc`s; the `With*` helpers cover common settings.
- **Interfaces.** A type that implements an interface declares a compile-time assertion (`var _ Iface = (*T)(nil)`).
- **Naming.** MixedCaps with uniform-case initialisms (`URL`, `ID`, `TLS`); accessors are not prefixed with `Get`.
- **Tests.** API tests live in `_test` packages; tests that need unexported behavior live in the package. Use `t.Parallel` where safe and run `go test -race ./...`.
- **Examples.** Add runnable `Example` functions with deterministic `// Output` blocks for new public behavior.
- **Formatting.** Code is `gofmt`/`goimports` clean, with imports grouped standard library / external / `github.com/hueristiq`. Style is enforced mechanically by the repo-pinned `golangci-lint` configuration — run `make go-lint` before submitting.
