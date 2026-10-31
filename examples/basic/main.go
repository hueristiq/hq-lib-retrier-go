package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	hqgoretrier "github.com/hueristiq/hq-lib-retrier-go"
)

func main() {
	attempts := 0

	err := hqgoretrier.Retry(context.Background(), func() error {
		attempts++
		if attempts < 3 {
			return errors.New("transient failure")
		}

		return nil
	},
		hqgoretrier.WithMaxAttempts(3),
		hqgoretrier.WithWaitMin(time.Millisecond),
		hqgoretrier.WithWaitMax(5*time.Millisecond),
	)
	fmt.Println("attempts:", attempts, "error:", err)
}
