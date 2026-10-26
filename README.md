# hq-lib-retrier-go

![made with go](https://img.shields.io/badge/made%20with-Go-1E90FF.svg) [![go reference](https://pkg.go.dev/badge/github.com/hueristiq/hq-lib-retrier-go.svg)](https://pkg.go.dev/github.com/hueristiq/hq-lib-retrier-go) [![license](https://img.shields.io/badge/license-MIT-gray.svg?color=1E90FF)](https://github.com/hueristiq/hq-lib-retrier-go/blob/main/LICENSE) ![maintenance](https://img.shields.io/badge/maintained%3F-yes-1E90FF.svg) [![open issues](https://img.shields.io/github/issues-raw/hueristiq/hq-lib-retrier-go.svg?style=flat&color=1E90FF)](https://github.com/hueristiq/hq-lib-retrier-go/issues?q=is:issue+is:open) [![closed issues](https://img.shields.io/github/issues-closed-raw/hueristiq/hq-lib-retrier-go.svg?style=flat&color=1E90FF)](https://github.com/hueristiq/hq-lib-retrier-go/issues?q=is:issue+is:closed) [![contribution](https://img.shields.io/badge/contributions-welcome-1E90FF.svg)](https://github.com/hueristiq/hq-lib-retrier-go/blob/main/CONTRIBUTING.md)

`hq-lib-retrier-go` is a [Go](https://golang.org/) package for retrying operations that may fail transiently.

## Resources

- [Features](#features)
- [Installation](#installation)
- [Usage](#usage)
	- [Retry](#retry)
	- [Retry With Data](#retry-with-data)
	- [Non-Retryable Errors](#non-retryable-errors)
- [Configuration](#configuration)
- [Contributing](#contributing)
- [Licensing](#licensing)

## Features

- **Configurable Retry Policy:** Max attempts, min/max delays, and backoff strategy.
- **Result-Carrying Operations:** `RetryWithData` retries operations that return a value.
- **Context Support:** Cancellation and deadlines are checked before and between attempts and during waits.

## Installation

To install `hq-lib-retrier-go`, run the following command in your Go project:

```bash
go get github.com/hueristiq/hq-lib-retrier-go@latest
```

Requires Go 1.26.3 or later.

## Usage

The examples below import the package under the `hqgoretrier` alias.

```go
import hqgoretrier "github.com/hueristiq/hq-lib-retrier-go"
```

The snippets below are illustrative.

### Retry

Use `Retry` for operations that return only an error:

```go
package main

import (
	"context"
	"fmt"
	"time"

	hqgoretrier "github.com/hueristiq/hq-lib-retrier-go"
	hqgoretrierbackoff "github.com/hueristiq/hq-lib-retrier-go/backoff"
)

func main() {
	attempts := 0

	operation := func() error {
		attempts++

		if attempts < 3 {
			return fmt.Errorf("temporary failure (attempt %d)", attempts)
		}

		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	defer cancel()

	err := hqgoretrier.Retry(ctx, operation,
		hqgoretrier.WithMaxAttempts(5),
		hqgoretrier.WithWaitMin(100*time.Millisecond),
		hqgoretrier.WithWaitMax(2*time.Second),
		hqgoretrier.WithBackoff(hqgoretrierbackoff.ExponentialWithFullJitter),
		hqgoretrier.WithOnRetry(func(event hqgoretrier.RetryEvent) {
			fmt.Printf("Attempt %d failed: %v. Next attempt in %v.\n", event.Attempt, event.Err, event.Wait)
		}),
	)
	if err != nil {
		fmt.Printf("Operation failed after retries: %v\n", err)
	} else {
		fmt.Println("Operation succeeded!")
	}
}
```

### Retry With Data

Use `RetryWithData` for operations that return both a result and an error:

```go
package main

import (
	"context"
	"fmt"
	"time"

	hqgoretrier "github.com/hueristiq/hq-lib-retrier-go"
	hqgoretrierbackoff "github.com/hueristiq/hq-lib-retrier-go/backoff"
)

func main() {
	attempts := 0

	fetchData := func() (string, error) {
		attempts++

		if attempts < 2 {
			return "", fmt.Errorf("failed to fetch data (attempt %d)", attempts)
		}

		return "payload", nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	defer cancel()

	result, err := hqgoretrier.RetryWithData(ctx, fetchData,
		hqgoretrier.WithMaxAttempts(5),
		hqgoretrier.WithWaitMin(200*time.Millisecond),
		hqgoretrier.WithWaitMax(3*time.Second),
		hqgoretrier.WithBackoff(hqgoretrierbackoff.Exponential),
		hqgoretrier.WithOnRetry(func(event hqgoretrier.RetryEvent) {
			fmt.Printf("Attempt %d failed: %v, waiting %v.\n", event.Attempt, event.Err, event.Wait)
		}),
	)
	if err != nil {
		fmt.Printf("Failed to fetch data after retries: %v\n", err)

		return
	}

	fmt.Printf("Data fetched successfully: %s\n", result)
}
```

### Non-Retryable Errors

Not every failure deserves another attempt. `WithRetryOn(fn)` classifies errors after each failed attempt; when it returns `false`, the retry loop stops immediately and returns the offending error instead of scheduling the next one:

```go
err := hqgoretrier.Retry(ctx, operation,
	hqgoretrier.WithRetryOn(func(err error) bool {
		return !errors.Is(err, sql.ErrNoRows) // missing rows are not worth retrying
	}),
)
```

## Configuration

Behavior is set through functional options passed to `Retry` or `RetryWithData`. Any option left unset falls back to its default. The `With*` helpers cover the common cases; because the option receives the exported `Options` type, callers can also write custom `OptionFunc`s.

| Option | Description | Default |
| --- | --- | --- |
| `WithMaxAttempts(n)` | Maximum number of attempts, **including the initial call** (so `3` means the first call plus up to two retries). A value `<= 0` falls back to the default; pass `math.MaxInt` for effectively unbounded retries bounded only by the context. | `DefaultMaxAttempts` (`3`) |
| `WithWaitMin(d)` | Base delay for the built-in exponential strategies (the first retry waits `2×` this value). A value `<= 0` falls back to the default. | `DefaultWaitMin` (`1s`) |
| `WithWaitMax(d)` | Upper bound for the delay between attempts. A value `<= 0` falls back to the default; a value below the minimum is raised to the minimum. | `DefaultWaitMax` (`30s`) |
| `WithBackoff(fn)` | Constructor for the strategy that computes each delay (see below). It is called once per retry loop with the normalized bounds, so stateful strategies always get fresh state. Passing `nil` selects the default; a constructor that returns `nil` panics. | `backoff.ExponentialWithDecorrelatedJitter` |
| `WithRetryOn(fn)` | Predicate deciding whether a failed attempt's error is retryable; returning `false` stops retrying and returns that error. | retry all errors |
| `WithOnRetry(fn)` | Callback invoked after each failed attempt that will be retried, receiving a RetryEvent with the attempt number, the error, and the next delay. | none |

Invalid values are normalized before the first attempt, and any non-positive delay a strategy computes is clamped to a 1 ms floor, so the retry loop never spins at full CPU.

## Contributing

Contributions are welcome and encouraged! Feel free to submit [Pull Requests](https://github.com/hueristiq/hq-lib-retrier-go/pulls) or report [Issues](https://github.com/hueristiq/hq-lib-retrier-go/issues). For more details, check out the [contribution guidelines](https://github.com/hueristiq/hq-lib-retrier-go/blob/main/CONTRIBUTING.md).

A big thank you to all the [contributors](https://github.com/hueristiq/hq-lib-retrier-go/graphs/contributors) for your ongoing support!

![contributors](https://contrib.rocks/image?repo=hueristiq/hq-lib-retrier-go&max=500)

## Licensing

This package is licensed under the [MIT license](https://opensource.org/license/mit). You are free to use, modify, and distribute it, as long as you follow the terms of the license. You can find the full license text in the repository - [Full MIT license text](https://github.com/hueristiq/hq-lib-retrier-go/blob/main/LICENSE).
