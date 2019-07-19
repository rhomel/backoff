package backoff

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rhomel/backoff/test/try"
)

// whitebox tests

type durations struct {
	durations []time.Duration
}

// log the received pause durations as `try` backs-off
func afterFnLogger() (*durations, func(time.Duration) <-chan time.Time) {
	ds := &durations{}
	return ds, func(d time.Duration) <-chan time.Time {
		ds.durations = append(ds.durations, d)
		return defaultAfterFunc(d)
	}
}

func Test_try(t *testing.T) {
	var (
		shortDelay    = 10 * time.Millisecond
		shortInterval = Exponential{
			Base:    2 * time.Millisecond,
			Unit:    time.Millisecond,
			Initial: 1 * time.Millisecond,
			Max:     20 * time.Millisecond,
		}
	)

	cases := map[string]struct {
		trueAfterN    int
		tries         int8
		initI         int8
		initWait      time.Duration
		timeout       time.Duration
		delay         time.Duration
		interval      Intervals
		wantErr       error
		wantDurations []time.Duration
		wantEvents    []string
	}{
		"Succeed After 3 Tries": {
			trueAfterN: 3,
			tries:      10,
			initI:      0,
			initWait:   0,
			timeout:    time.Second,
			delay:      shortDelay,
			interval:   shortInterval,
			wantErr:    nil,
			wantDurations: []time.Duration{
				1 * time.Millisecond,
				2 * time.Millisecond,
				4 * time.Millisecond,
			},
			wantEvents: []string{
				try.CaseAfter,
				try.CaseReturnFalse,
				try.CaseAfter,
				try.CaseReturnFalse,
				try.CaseAfter,
				try.CaseReturnFalse,
				try.CaseAfter,
				try.CaseReturnTrue,
			},
		},
		"Should Not Overflow": {
			trueAfterN: 2,
			tries:      InfiniteTries,
			initI:      math.MaxInt8 - 1,
			initWait:   0,
			timeout:    time.Second,
			delay:      shortDelay,
			interval:   shortInterval,
			wantErr:    nil,
			wantDurations: []time.Duration{
				shortInterval.Max,
				shortInterval.Max,
			},
			wantEvents: []string{
				try.CaseAfter,
				try.CaseReturnFalse,
				try.CaseAfter,
				try.CaseReturnFalse,
				try.CaseAfter,
				try.CaseReturnTrue,
			},
		},
		"Fail after 4 tries": {
			trueAfterN: 4,
			tries:      4,
			initI:      0,
			initWait:   0,
			timeout:    time.Second,
			delay:      shortDelay,
			interval:   shortInterval,
			wantErr:    AllTriesFailed,
			wantDurations: []time.Duration{
				1 * time.Millisecond,
				2 * time.Millisecond,
				4 * time.Millisecond,
			},
			wantEvents: []string{
				try.CaseAfter,
				try.CaseReturnFalse,
				try.CaseAfter,
				try.CaseReturnFalse,
				try.CaseAfter,
				try.CaseReturnFalse,
				try.CaseAfter,
				try.CaseReturnFalse,
			},
		},
		"Fail after context timeout during fn call": {
			trueAfterN: 6,
			tries:      10,
			initI:      0,
			initWait:   0,
			timeout:    70 * time.Millisecond,
			delay:      50 * time.Millisecond,
			interval: Exponential{
				Base:    0 * time.Millisecond,
				Unit:    time.Millisecond,
				Initial: 0 * time.Millisecond,
				Max:     0 * time.Millisecond,
			},
			wantErr: BackoffContextTimeoutExceeded,
			wantDurations: []time.Duration{
				0 * time.Millisecond,
				0 * time.Millisecond,
			},
			wantEvents: []string{
				try.CaseAfter,
				try.CaseReturnFalse,
				try.CaseDone,
				try.CaseReturnFalse,
			},
		},
		"Fail after context timeout during backoff pause": {
			trueAfterN: 6,
			tries:      10,
			initI:      0,
			initWait:   0,
			timeout:    70 * time.Millisecond,
			delay:      0 * time.Millisecond,
			interval: Exponential{
				Base:    200 * time.Millisecond,
				Unit:    time.Millisecond,
				Initial: 200 * time.Millisecond,
				Max:     200 * time.Millisecond,
			},
			wantErr: BackoffContextTimeoutExceeded,
			wantDurations: []time.Duration{
				200 * time.Millisecond,
			},
			wantEvents: []string{
				try.CaseAfter,
				try.CaseReturnFalse,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc := tc
			ds, afterFn := afterFnLogger()
			events, tryFn := try.FnLogger(tc.delay, tc.trueAfterN)

			ctx, cancel := context.WithTimeout(context.Background(), tc.timeout)
			defer cancel()
			bo := NewBackoff(tc.interval, withAfterFunc(afterFn))
			err := bo.try(ctx, tc.tries, tryFn, tc.initI, tc.initWait)

			assert.Equal(t, tc.wantErr, err)
			assert.Equal(t, tc.wantDurations, ds.durations)
			assert.Equal(t, tc.wantEvents, events.Events)
		})
	}
}

var defaultExampleCases = map[string]struct {
	i    int8
	last time.Duration
	want time.Duration
}{
	"initial": {
		i:    0,
		last: 0 * time.Millisecond,
		want: 500 * time.Millisecond,
	},
	"1": {
		i:    1,
		last: 500 * time.Millisecond,
		want: 1000 * time.Millisecond,
	},
	"2": {
		i:    2,
		last: 1000 * time.Millisecond,
		want: 2000 * time.Millisecond,
	},
	"3": {
		i:    3,
		last: 2000 * time.Millisecond,
		want: 4000 * time.Millisecond,
	},
	"4": {
		i:    4,
		last: 4000 * time.Millisecond,
		want: 8000 * time.Millisecond,
	},
	"5": {
		i:    5,
		last: 8000 * time.Millisecond,
		want: 16000 * time.Millisecond,
	},
	"6": {
		i:    6,
		last: 16000 * time.Millisecond,
		want: 20000 * time.Millisecond,
	},
	"7": {
		i:    7,
		last: 20000 * time.Millisecond,
		want: 20000 * time.Millisecond,
	},
	"i=0 is always initial value": {
		i:    0,
		last: 500 * time.Millisecond,
		want: 500 * time.Millisecond,
	},
	"i=MaxInt8 is always max": {
		i:    math.MaxInt8,
		last: 0 * time.Millisecond,
		want: DefaultBinaryExponential().Max,
	},
}

func Test_DefaultBinaryExponential_NextShouldFollowHappyPath(t *testing.T) {
	dbe := DefaultBinaryExponential()

	for name, tc := range defaultExampleCases {
		t.Run(name, func(t *testing.T) {
			tc := tc
			got := dbe.Next(tc.i, tc.last)
			assert.Equal(t, tc.want, got)
		})
	}
}

func Test_DefaultBinaryExponentialJitter_NextShouldFollowHappyPath(t *testing.T) {
	dbej, err := DefaultBinaryExponentialJitter()
	require.NoError(t, err)

	for name, tc := range defaultExampleCases {
		t.Run(name, func(t *testing.T) {
			tc := tc
			got := dbej.Next(tc.i, tc.last)
			wantMin := tc.want - dbej.JitterMax
			wantMax := tc.want + dbej.JitterMax
			assert.True(t, wantMin <= got && got <= wantMax)
		})
	}
}

func Test_Exponential_Base3(t *testing.T) {
	t.Parallel()

	e := Exponential{
		Base:    3 * time.Second,
		Unit:    time.Second,
		Initial: 1 * time.Second,
		Max:     30 * time.Second,
	}

	var cases = map[string]struct {
		i    int8
		last time.Duration
		want time.Duration
	}{
		"initial": {
			i:    0,
			last: 0 * time.Millisecond,
			want: 1000 * time.Millisecond,
		},
		"1": {
			i:    1,
			last: 1000 * time.Millisecond,
			want: 3000 * time.Millisecond,
		},
		"2": {
			i:    2,
			last: 3000 * time.Millisecond,
			want: 9000 * time.Millisecond,
		},
		"3": {
			i:    3,
			last: 9000 * time.Millisecond,
			want: 27000 * time.Millisecond,
		},
		"4": {
			i:    4,
			last: 27000 * time.Millisecond,
			want: 30000 * time.Millisecond,
		},
		"5": {
			i:    5,
			last: 30000 * time.Millisecond,
			want: 30000 * time.Millisecond,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc := tc
			got := e.Next(tc.i, tc.last)
			assert.Equal(t, tc.want, got)
		})
	}
}

func Test_Exponential_Base3Initial0IsAlwaysZero(t *testing.T) {
	t.Parallel()

	e := Exponential{
		Base:    3 * time.Second,
		Unit:    time.Second,
		Initial: 0 * time.Second,
		Max:     30 * time.Second,
	}

	for i := 0; i < 7; i++ {
		t.Run(fmt.Sprintf("Iteration %d", i), func(t *testing.T) {
			i := i
			got := e.Next(int8(i), 0)
			assert.Equal(t, time.Duration(0), got)
		})
	}
}

func Test_DefaultBinaryExponentialJitter_RandomInputNextShouldBeWithinRange(t *testing.T) {
	dbej, err := DefaultBinaryExponentialJitter()
	require.NoError(t, err)

	var maxI int8 = 20
	minWant := time.Duration(0)
	maxWant := dbej.Max + dbej.JitterMax

	for iteration := 0; iteration < 1000; iteration++ {
		i := int8(rand.Intn(int(maxI)))
		last := time.Duration(rand.Int63n(int64(dbej.JitterMax)))
		got := dbej.Next(i, last)

		assert.True(t, minWant <= got && got <= maxWant,
			"Next(%d, %s) got %s is not in range %s and %s",
			i, last, got, minWant, maxWant)
	}
}
