// Copyright (C) 2026 blubskye <https://github.com/blubskye/discord2stoat>
// SPDX-License-Identifier: AGPL-3.0-or-later

package stoat

import (
	"encoding/json"
	"log"
	"math/rand/v2"
	"strings"
	"time"
)

// jitterMs adds 10–15% random jitter to a wait duration to avoid thundering-herd
// re-collisions where the server's retry_after window keeps getting hit on the boundary.
func jitterMs(ms int64) int64 {
	// Add between 10% and 15% extra.
	extra := ms/10 + int64(rand.N(ms/20+1))
	return ms + extra
}

// withRetry executes fn, retrying on Revolt 429 responses by honouring retry_after (milliseconds).
func withRetry(fn func() error) error {
	for {
		err := fn()
		if err == nil {
			return nil
		}
		ms := parseRetryAfterMs(err)
		if ms <= 0 {
			return err
		}
		wait := jitterMs(ms)
		log.Printf("stoat: rate limited, waiting %dms before retry", wait)
		time.Sleep(time.Duration(wait) * time.Millisecond)
	}
}

// withRetryVal is the generic form of withRetry for calls that return a value.
func withRetryVal[T any](fn func() (T, error)) (T, error) {
	for {
		v, err := fn()
		if err == nil {
			return v, nil
		}
		ms := parseRetryAfterMs(err)
		if ms <= 0 {
			return v, err
		}
		wait := jitterMs(ms)
		log.Printf("stoat: rate limited, waiting %dms before retry", wait)
		time.Sleep(time.Duration(wait) * time.Millisecond)
	}
}

// parseRetryAfterMs extracts the retry_after value (milliseconds) from a Revolt 429 error string.
// Error format: "bad status code 429: {"retry_after":8740}"
// Returns 0 if the error is not a 429.
func parseRetryAfterMs(err error) int64 {
	s := err.Error()
	if !strings.Contains(s, "429") {
		return 0
	}
	idx := strings.Index(s, "{")
	if idx == -1 {
		return 1000 // default 1 s fallback
	}
	var p struct {
		RetryAfter int64 `json:"retry_after"`
	}
	if json.Unmarshal([]byte(s[idx:]), &p) != nil || p.RetryAfter <= 0 {
		return 1000
	}
	return p.RetryAfter
}
