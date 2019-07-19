package backoff

import (
	"context"
	crypto "crypto/rand"
	"math"
	"math/big"
	"math/rand"
	"time"
)

// Error represents a constant typed error
type Error string

func (e Error) Error() string {
	return string(e)
}

const (
	// InfiniteTries represents infinite `tries`. Use this in the `Try` method to
	// keep trying until Completable returns true
	InfiniteTries = math.MaxInt8

	// AllTriesFailed indicates that all requested tries failed
	AllTriesFailed = Error("all tries failed")
	// BackoffContextTimeoutExceeded indicates that the backoff context Done
	// channel was closed
	BackoffContextTimeoutExceeded = Error("backoff context timeout exceeded")
)

// Completable is a function that should complete and terminate early if the
// context.Done() channel is closed.
type Completable func(ctx context.Context) bool

// after represents time.After method signature
// this should only be used for testing
type after func(time.Duration) <-chan time.Time

func defaultAfterFunc(d time.Duration) <-chan time.Time {
	return time.After(d)
}

// Options are additional options to be used in NewBackoff. Currently
// there are no exported options, only options that are used internally for
// testing.
type Options func(bo *Backoff)

// only for testing
func withAfterFunc(fn after) Options {
	return func(bo *Backoff) {
		bo.afterFunc = fn
	}
}

// Backoff is a simple backoff implementation. You will want to use NewBackoff
// or NewBackoffWithTimeout to create an instance.
type Backoff struct {
	intervals Intervals
	afterFunc after
	result    chan bool
}

// NewBackoff creates a new Backoff struct. Intervals represents the interval
// calculation algorithm (ex: Exponential). Tries represents the number of times
// to call the function. Context may be a cancellable context.
//
// If you want a timeout Context, consider using NewBackoffWithTimeout instead.
func NewBackoff(intervals Intervals, options ...Options) *Backoff {
	backoff := &Backoff{
		intervals: intervals,
		afterFunc: defaultAfterFunc,
		result:    make(chan bool, 1),
	}
	for _, option := range options {
		option(backoff)
	}
	return backoff
}

// Try will try to call the provided Completable the number of times specifed in
// NewBackoff until an execution of Completable returns true.
//
// If the Completable returns false more times than specified in tries in
// NewBackoff, then Try will return a AllTriesFailed error.
//
// If the provided context cancel function is called before a Completable call
// returns true, then Try will return a BackoffContextTimeoutExceeded error.
func (b *Backoff) Try(ctx context.Context, tries int8, fn Completable) error {
	return b.try(ctx, tries, fn, 0, 0)
}

// Specify initI and initWait to start the loop at a pre-determined point in the
// series. The assumed starting point is initI = 0, initWait = 0.
func (b *Backoff) try(ctx context.Context, tries int8, fn Completable, initI int8, initWait time.Duration) error {
	wait := initWait
	i := initI
	for {
		if fn(ctx) {
			return nil
		}
		if i+1 >= tries && InfiniteTries != tries {
			return AllTriesFailed
		}
		wait = b.intervals.Next(i, wait)
		chWait := b.afterFunc(wait)
		select {
		case <-ctx.Done():
			return BackoffContextTimeoutExceeded
		case <-chWait:
			// repeat the loop
			if i < InfiniteTries {
				i++
			}
		}
	}
}

// Intervals represents the interface backoff interval function should
// implement. `i` represents the current iteration. `last` represents the last
// backoff duration for the previous iteration, zero if this is the first
// iteration. The number of iterations is expected to be fairly small, but if
// the number of iterations is InfiniteTries (math.MaxInt8), `i` will always be
// InfiniteTries.
type Intervals interface {
	Next(i int8, last time.Duration) time.Duration
}

// Exponential implements an exponential interval function.
type Exponential struct {
	Base    time.Duration
	Unit    time.Duration
	Initial time.Duration
	Max     time.Duration
}

var _ Intervals = (*Exponential)(nil)

// DefaultBinaryExponential creates a binary exponential interval function with
// the following series: 0.5s, 1s, 2s, 4s, 8s, 16s, 20s, 20s, ...
func DefaultBinaryExponential() Exponential {
	return Exponential{
		Base:    2 * time.Second,
		Unit:    time.Second,
		Initial: 500 * time.Millisecond,
		Max:     20 * time.Second,
	}
}

// Next provides the interval in the series based in iteration.
//
// Note that we intentially do not use `last` in this function so it is easy to
// add a consistent Jitter implementation on top of this. The trade-off is we
// have to do a floating point Pow calculation.
func (e Exponential) Next(i int8, last time.Duration) time.Duration {
	base := e.Base / e.Unit // base without unit scalar
	pow := math.Pow(float64(base), float64(i))
	if math.IsInf(pow, 1) {
		return e.Max
	}
	next := float64(e.Initial) * pow
	if next > float64(e.Max) {
		return e.Max
	}
	return time.Duration(next)
}

// ExponentialJitter implements an exponential interval function with a
// random jitter factor added to each fixed interval.
type ExponentialJitter struct {
	Exponential
	JitterMax time.Duration
	Rand      *rand.Rand
}

// generates a new *rand.Rand with a cryptographically random seed
func newRand() (*rand.Rand, error) {
	seedMax, err := crypto.Int(crypto.Reader, big.NewInt(math.MaxInt64))
	if err != nil {
		return nil, err
	}
	return rand.New(rand.NewSource(seedMax.Int64())), nil
}

// DefaultBinaryExponentialJitter creates a DefaultBinaryExponential interval
// function with each interval adjusted by a random value between +/- 500ms. The
// current underlying implemntation uses crypto/rand to seed the psuedo-random
// generator.
//
// Since the crypt/rand generator can fail due to io errors, the method returns
// an error if any.
func DefaultBinaryExponentialJitter() (ExponentialJitter, error) {
	random, err := newRand()
	if err != nil {
		return ExponentialJitter{}, err
	}
	return ExponentialJitter{
		Exponential: DefaultBinaryExponential(),

		JitterMax: 500 * time.Millisecond,
		Rand:      random,
	}, nil
}

// Next provides the interval in the series based in iteration. Since this
// method contains jitter and it is seeded by crypto/rand it will return
// seemingly non-deterministic random values.
func (ej ExponentialJitter) Next(i int8, last time.Duration) time.Duration {
	randRange := ej.JitterMax * 2
	// center at 0
	jitter := ej.Rand.Int63n(int64(randRange)) - int64(ej.JitterMax)
	return ej.Exponential.Next(i, last) + time.Duration(jitter)
}
