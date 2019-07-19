package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/rhomel/backoff"
)

// This example tries to do a HTTP GET on an httpbin.org endpoint. The
// httpbin.org endpoint will randomly return 200, 400, or 429 status. We want to
// keep trying the GET until we get a 200 status code.
//
// Example output:
//
// 2019/07/19 15:31:57 got: 429
// 2019/07/19 15:31:58 got: 400
// 2019/07/19 15:31:59 got: 400
// 2019/07/19 15:32:01 got: 400
// 2019/07/19 15:32:05 got: 200
// 2019/07/19 15:32:05 succeeded: 200
//
// For demonstration purposes only--you should probably do better code
// isolatation in practice.

func main() {
	var (
		resp *http.Response
		req  *http.Request
		// keep the last request error for inspection if all tries fail
		lastErr error

		timeout      = 10 * time.Second
		tries   int8 = 5

		url = "https://httpbin.org/status/200%2C400%2C429"
	)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		panic(err)
	}

	bo := backoff.NewBackoff(backoff.DefaultBinaryExponential())
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	err = bo.Try(ctx, tries, func(ctx context.Context) bool {
		resp, lastErr = http.DefaultClient.Do(req.WithContext(ctx))
		if lastErr == nil {
			log.Println("got:", resp.StatusCode)
		} else {
			log.Println("error:", lastErr)
		}
		return lastErr == nil && resp.StatusCode >= 200 && resp.StatusCode < 400
	})

	if err != nil {
		log.Println("failed:", err)
		if lastErr != nil {
			log.Println("last request error:", lastErr)
		}
		os.Exit(1)
	}

	log.Println("succeeded:", resp.StatusCode)
}
