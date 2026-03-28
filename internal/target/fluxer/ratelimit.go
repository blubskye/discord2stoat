// Copyright (C) 2026 blubskye <https://github.com/blubskye/discord2stoat>
// SPDX-License-Identifier: AGPL-3.0-or-later

package fluxer

import (
	"encoding/json"
	"log"
	"regexp"
	"time"
)

// maxRetries is the number of times to retry a Fluxer API call on 429.
const maxRetries = 8

// bodyRe extracts the JSON body from fluxergo error strings of the form:
//
//	Status: 429 Too Many Requests, Body: {...}
var bodyRe = regexp.MustCompile(`Body:\s*(\{.*\})`)

// fluxerRateLimitBody is the JSON payload Fluxer returns on 429.
type fluxerRateLimitBody struct {
	Code       string  `json:"code"`
	RetryAfter float64 `json:"retry_after"`
}

// retryAfterFromErr parses a retry_after duration from a fluxergo 429 error.
// Returns 0 if the error is not a 429 or parsing fails.
func retryAfterFromErr(err error) time.Duration {
	if err == nil {
		return 0
	}
	m := bodyRe.FindStringSubmatch(err.Error())
	if len(m) < 2 {
		return 0
	}
	var body fluxerRateLimitBody
	if jsonErr := json.Unmarshal([]byte(m[1]), &body); jsonErr != nil {
		return 0
	}
	if body.Code != "RATE_LIMITED" || body.RetryAfter <= 0 {
		return 0
	}
	return time.Duration(body.RetryAfter*1000) * time.Millisecond
}

// withRetry calls fn, retrying on 429 up to maxRetries times.
// On each 429 it sleeps the retry_after duration from the response body.
func withRetry(label string, fn func() error) error {
	for attempt := 0; attempt <= maxRetries; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}
		wait := retryAfterFromErr(err)
		if wait == 0 {
			// Not a rate-limit error; don't retry.
			return err
		}
		if attempt == maxRetries {
			return err
		}
		log.Printf("fluxer: %s: 429 rate limited, retrying in %s (attempt %d/%d)",
			label, wait.Round(time.Millisecond), attempt+1, maxRetries)
		time.Sleep(wait)
	}
	return nil // unreachable
}

// withRetryVal is withRetry for functions that return (T, error).
func withRetryVal[T any](label string, fn func() (T, error)) (T, error) {
	var zero T
	for attempt := 0; attempt <= maxRetries; attempt++ {
		v, err := fn()
		if err == nil {
			return v, nil
		}
		wait := retryAfterFromErr(err)
		if wait == 0 {
			return zero, err
		}
		if attempt == maxRetries {
			return zero, err
		}
		log.Printf("fluxer: %s: 429 rate limited, retrying in %s (attempt %d/%d)",
			label, wait.Round(time.Millisecond), attempt+1, maxRetries)
		time.Sleep(wait)
	}
	return zero, nil // unreachable
}
