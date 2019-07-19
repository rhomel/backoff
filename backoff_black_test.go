package backoff_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/rhomel/backoff"
	"github.com/rhomel/backoff/test/try"
)

// blackbox tests

func Test_Try(t *testing.T) {
	shortDelay := 10 * time.Millisecond

	cases := map[string]struct {
		trueAfterN    int
		tries         int8
		timeout       time.Duration
		delay         time.Duration
		interval      backoff.Intervals
		wantErr       error
		wantDurations []time.Duration
		wantEvents    []string
	}{
		"Succeed Immediately": {
			trueAfterN: 0,
			tries:      1,
			timeout:    time.Second,
			delay:      shortDelay,
			interval:   backoff.DefaultBinaryExponential(),
			wantErr:    nil,
			wantEvents: []string{
				try.CaseAfter,
				try.CaseReturnTrue,
			},
		},
		"Succeed After 3 tries": {
			trueAfterN: 2,
			tries:      3,
			timeout:    5 * time.Second,
			delay:      shortDelay,
			interval:   backoff.DefaultBinaryExponential(),
			wantErr:    nil,
			wantEvents: []string{
				try.CaseAfter,
				try.CaseReturnFalse,
				try.CaseAfter,
				try.CaseReturnFalse,
				try.CaseAfter,
				try.CaseReturnTrue,
			},
		},
		"Fail After 3 tries": {
			trueAfterN: 3,
			tries:      3,
			timeout:    5 * time.Second,
			delay:      shortDelay,
			interval:   backoff.DefaultBinaryExponential(),
			wantErr:    backoff.AllTriesFailed,
			wantEvents: []string{
				try.CaseAfter,
				try.CaseReturnFalse,
				try.CaseAfter,
				try.CaseReturnFalse,
				try.CaseAfter,
				try.CaseReturnFalse,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc := tc
			events, tryFn := try.FnLogger(tc.delay, tc.trueAfterN)

			ctx, cancel := context.WithTimeout(context.Background(), tc.timeout)
			defer cancel()
			bo := backoff.NewBackoff(tc.interval)
			err := bo.Try(ctx, tc.tries, tryFn)

			assert.Equal(t, tc.wantErr, err)
			assert.Equal(t, tc.wantEvents, events.Events)
		})
	}
}
