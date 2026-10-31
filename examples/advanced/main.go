package main

import (
	"context"
	"fmt"

	hqgoretrier "github.com/hueristiq/hq-lib-retrier-go"
)

func main() {
	result, err := hqgoretrier.RetryWithData(context.Background(), func() (string, error) {
		return "ready", nil
	}, hqgoretrier.WithMaxAttempts(2))

	fmt.Println(result, err)
}
