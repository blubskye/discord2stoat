// Copyright (C) 2026 blubskye <https://github.com/blubskye/discord2stoat>
// SPDX-License-Identifier: AGPL-3.0-or-later

package fluxer

import (
	"encoding/json"
	"log"
	"regexp"
	"strings"
	"time"
)

// maxRetries is the number of times to retry a Fluxer API call on retriable errors.
const maxRetries = 8

// serverErrBackoff is the fixed wait before retrying on a 5xx response.
const serverErrBackoff = 5 * time.Second

// bodyRe extracts the JSON body from fluxergo error strings of the form:
//
//	Status: 429 Too Many Requests, Body: {...}
var bodyRe = regexp.MustCompile(`Body:\s*(\{.*\})`)

// fluxerRateLimitBody is the JSON payload Fluxer returns on 429.
type fluxerRateLimitBody struct {
	Code       string  `json:"code"`
	RetryAfter float64 `json:"retry_after"`
}

// retryWaitFromErr returns the duration to wait before retrying, or 0 if the
// error is not retriable.
//
//   - 429 RATE_LIMITED → wait retry_after seconds from the response body
//   - 5xx server errors (502, 503, 504, …) → wait serverErrBackoff
//   - anything else → 0 (not retriable)
func retryWaitFromErr(err error) time.Duration {
	if err == nil {
		return 0
	}
	s := err.Error()

	// 5xx transient server errors.
	for _, code := range []string{"500", "502", "503", "504"} {
		if strings.Contains(s, "Status: "+code) {
			return serverErrBackoff
		}
	}

	// 429 rate limit — parse retry_after from JSON body.
	m := bodyRe.FindStringSubmatch(s)
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

// withRetry calls fn, retrying on retriable errors up to maxRetries times.
func withRetry(label string, fn func() error) error {
	for attempt := 0; attempt <= maxRetries; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}
		wait := retryWaitFromErr(err)
		if wait == 0 {
			return err
		}
		if attempt == maxRetries {
			return err
		}
		log.Printf("fluxer: %s: retrying in %s (attempt %d/%d): %v",
			label, wait.Round(time.Millisecond), attempt+1, maxRetries, err)
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
		wait := retryWaitFromErr(err)
		if wait == 0 {
			return zero, err
		}
		if attempt == maxRetries {
			return zero, err
		}
		log.Printf("fluxer: %s: retrying in %s (attempt %d/%d): %v",
			label, wait.Round(time.Millisecond), attempt+1, maxRetries, err)
		time.Sleep(wait)
	}
	return zero, nil // unreachable
}
