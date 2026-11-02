# hq-lib-retrier-go

![made with go](https://img.shields.io/badge/made%20with-Go-1E90FF.svg) [![go reference](https://pkg.go.dev/badge/github.com/hueristiq/hq-lib-retrier-go.svg)](https://pkg.go.dev/github.com/hueristiq/hq-lib-retrier-go) [![license](https://img.shields.io/badge/license-MIT-gray.svg?color=1E90FF)](https://github.com/hueristiq/hq-lib-retrier-go/blob/main/LICENSE) ![maintenance](https://img.shields.io/badge/maintained%3F-yes-1E90FF.svg) [![open issues](https://img.shields.io/github/issues-raw/hueristiq/hq-lib-retrier-go.svg?style=flat&color=1E90FF)](https://github.com/hueristiq/hq-lib-retrier-go/issues?q=is:issue+is:open) [![closed issues](https://img.shields.io/github/issues-closed-raw/hueristiq/hq-lib-retrier-go.svg?style=flat&color=1E90FF)](https://github.com/hueristiq/hq-lib-retrier-go/issues?q=is:issue+is:closed)

`hq-lib-retrier-go` is a [Go](https://golang.org/) package for retrying operations that may fail transiently.

## Resources

- [Features](#features)
- [Installation](#installation)
- [Usage](#usage)
	- [Retry](#retry)
	- [Retry with data](#retry-with-data)
	- [Non-retryable errors](#non-retryable-errors)
	- [Custom backoff and jitter](#custom-backoff-and-jitter)
- [Contributing](#contributing)
- [Licensing](#licensing)

## Features

- **Configurable policy:** max attempts, min/max delays, and backoff strategy.
- **Result-carrying operations:** `RetryWithData` retries operations that return a value.
- **Context support:** cancellation and deadlines are checked before and between attempts and during waits.
- **Pluggable backoff and jitter:** `WithBackoff` accepts constant, exponential, equal-jitter, full-jitter, or decorrelated strategies.
- **Retry hooks:** `WithRetryOn` stops on non-retryable errors, and `WithOnRetry` observes each attempt.

## Installation

To install `hq-lib-retrier-go`, run the following command in your Go project:

```bash
go get github.com/hueristiq/hq-lib-retrier-go@latest
```

Requires Go 1.22.0 or later.

## Usage

The examples import the package under the `hqgoretrier` alias. Subpackages use `hqgoretrierbackoff` and `hqgoretrierjitter`.

### Retry

Use `Retry` for operations that return only an error. Use `RetryWithData` for operations that also return a value.

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	hqgoretrier "github.com/hueristiq/hq-lib-retrier-go"
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
		hqgoretrier.WithOnRetry(func(event hqgoretrier.RetryEvent) {
			fmt.Printf("attempt %d failed: %v; next attempt in %s\n",
				event.Attempt, event.Err, event.Wait)
		}),
	)
	if err != nil {
		log.Fatal(err)
	}

	result, err := hqgoretrier.RetryWithData(ctx, func() (string, error) {
		return "payload", nil
	}, hqgoretrier.WithMaxAttempts(3))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(result)
}
```

### Retry with data

`RetryWithData` returns the last value alongside the error. On failure, treat the value as invalid even when the operation produced a partial result.

```go
fetchData := func() (string, error) {
	attempts++

	if attempts < 2 {
		return "", fmt.Errorf("failed to fetch data (attempt %d)", attempts)
	}

	return "payload", nil
}

result, err := hqgoretrier.RetryWithData(ctx, fetchData,
	hqgoretrier.WithMaxAttempts(5),
	hqgoretrier.WithWaitMin(200*time.Millisecond),
	hqgoretrier.WithWaitMax(3*time.Second),
)
if err != nil {
	log.Fatal(err)
}

fmt.Printf("data fetched: %s\n", result)
```

### Non-retryable errors

Not every failure deserves another attempt. `WithRetryOn(fn)` classifies errors after each failed attempt. When it returns `false`, the retry loop stops and returns the offending error.

```go
err := hqgoretrier.Retry(ctx, operation,
	hqgoretrier.WithRetryOn(func(err error) bool {
		return !errors.Is(err, sql.ErrNoRows)
	}),
)
```

### Custom backoff and jitter

`WithBackoff` takes a strategy constructor. The `backoff` package provides `Constant`, `Exponential`, `ExponentialWithEqualJitter`, `ExponentialWithFullJitter`, and `ExponentialWithDecorrelatedJitter`. The `jitter` package exposes the raw transforms `Equal`, `Full`, and `Decorrelated` for composing custom strategies.

```go
strategy := func(minDelay, maxDelay time.Duration) hqgoretrierbackoff.Backoff {
	var previous time.Duration

	return func(_ int) time.Duration {
		delay := hqgoretrierjitter.Decorrelated(minDelay, maxDelay, previous)

		previous = delay

		return delay
	}
}

err := hqgoretrier.Retry(ctx, operation, hqgoretrier.WithBackoff(strategy))
```

## Contributing

Contributions are welcome and encouraged! Feel free to submit [Pull Requests](https://github.com/hueristiq/hq-lib-retrier-go/pulls) or report [Issues](https://github.com/hueristiq/hq-lib-retrier-go/issues).

A big thank you to all the [contributors](https://github.com/hueristiq/hq-lib-retrier-go/graphs/contributors) for your ongoing support!

![contributors](https://contrib.rocks/image?repo=hueristiq/hq-lib-retrier-go&max=500)

## Licensing

This package is licensed under the [MIT license](https://opensource.org/license/mit). You are free to use, modify, and distribute it, as long as you follow the terms of the license. You can find the full license text in the repository - [Full MIT license text](https://github.com/hueristiq/hq-lib-retrier-go/blob/main/LICENSE).
