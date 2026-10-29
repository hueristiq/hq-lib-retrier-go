# hq-lib-retrier-go

![made with go](https://img.shields.io/badge/made%20with-Go-1E90FF.svg) [![go reference](https://pkg.go.dev/badge/github.com/hueristiq/hq-lib-retrier-go.svg)](https://pkg.go.dev/github.com/hueristiq/hq-lib-retrier-go) [![license](https://img.shields.io/badge/license-MIT-gray.svg?color=1E90FF)](https://github.com/hueristiq/hq-lib-retrier-go/blob/main/LICENSE) ![maintenance](https://img.shields.io/badge/maintained%3F-yes-1E90FF.svg) [![open issues](https://img.shields.io/github/issues-raw/hueristiq/hq-lib-retrier-go.svg?style=flat&color=1E90FF)](https://github.com/hueristiq/hq-lib-retrier-go/issues?q=is:issue+is:open) [![closed issues](https://img.shields.io/github/issues-closed-raw/hueristiq/hq-lib-retrier-go.svg?style=flat&color=1E90FF)](https://github.com/hueristiq/hq-lib-retrier-go/issues?q=is:issue+is:closed) [![contribution](https://img.shields.io/badge/contributions-welcome-1E90FF.svg)](https://github.com/hueristiq/hq-lib-retrier-go/blob/main/CONTRIBUTING.md)

`hq-lib-retrier-go` is a [Go](https://golang.org/) package for retrying operations that may fail transiently.

## Table of Contents

- [Features](#features)
- [Installation](#installation)
- [Usage](#usage)
	- [Retry](#retry)
	- [Retry With Data](#retry-with-data)
	- [Non-Retryable Errors](#non-retryable-errors)
- [Contributing](#contributing)
- [Licensing](#licensing)

## Features

- **Configurable Retry Policy:** Max attempts, min/max delays, and backoff strategy.
- **Result-Carrying Operations:** `RetryWithData` retries operations that return a value.
- **Context Support:** Cancellation and deadlines are checked before and between attempts and during waits.

## Installation

```bash
go get github.com/hueristiq/hq-lib-retrier-go@latest
```

Requires Go 1.23 or later.

## Usage

The examples import the package under the `hqgoretrier` alias:

```go
import hqgoretrier "github.com/hueristiq/hq-lib-retrier-go"
```

### Retry

Use `Retry` for operations that return only an error:

```go
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
		fmt.Printf("Attempt %d failed: %v. Next attempt in %v.\n", event.Attempt, event.Err, event.Wait)
	}),
)
if err != nil {
	fmt.Printf("Operation failed after retries: %v\n", err)

	return
}

fmt.Println("Operation succeeded!")
```

### Retry With Data

Use `RetryWithData` for operations that return both a result and an error:

```go
attempts := 0

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
	fmt.Printf("Failed to fetch data after retries: %v\n", err)

	return
}

fmt.Printf("Data fetched successfully: %s\n", result)
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

## Contributing

Contributions are welcome and encouraged! Feel free to submit [Pull Requests](https://github.com/hueristiq/hq-lib-retrier-go/pulls) or report [Issues](https://github.com/hueristiq/hq-lib-retrier-go/issues). For more details, check out the [contribution guidelines](https://github.com/hueristiq/hq-lib-retrier-go/blob/main/CONTRIBUTING.md).

A big thank you to all the [contributors](https://github.com/hueristiq/hq-lib-retrier-go/graphs/contributors) for their ongoing support!

![contributors](https://contrib.rocks/image?repo=hueristiq/hq-lib-retrier-go&max=500)

## Licensing

This package is licensed under the [MIT license](https://opensource.org/license/mit). You are free to use, modify, and distribute it, as long as you follow the terms of the license. You can find the full license text in the repository - [Full MIT license text](https://github.com/hueristiq/hq-lib-retrier-go/blob/main/LICENSE).
