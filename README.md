
# Exponential Backoff in Go

Implements an exponential backoff algorithm. Useful for situations where you
want to poll a resource that can intermittently fail (REST API, gRPC, etc)
but you do not want to flood the polled service on successive requests.

# Synopsis

```
import (
	"context"
	"time"

	"github.com/rhomel/backoff"
)

func main() {
	bo := backoff.NewBackoff(backoff.DefaultBinaryExponential())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = bo.Try(ctx, 5, func(ctx context.Context) bool {
		APIErr := api.CallThatCanIntermittentlyFail(ctx)
		return APIErr == nil // return true to report success
	})

	if err != nil {
		// all requests failed or context timed out
	}

	// success!
}
```

In this case, we assume that `api.CallThatCanIntermittentlyFail` will
correctly abort when `ctx.Done()` channel is closed.

See `example/example.go` for a full working example.

# Default Binary Exponential

The default retry interval is a binary exponential algorithm with the
following progression:

```
0.5s, 1s, 2s, 4s, 8s, 16s, 20s, 20s, ...
```

# Default Binary Exponential with Jitter

You can add a random "jitter" using `DefaultBinaryExponentialJitter`. The
default jitter will add/subtract a random value in the range of -0.5s to
0.5s. This will help distribute simulataneous failed polls by multiple
clients over a short period.

This will see the pseudo random generator with a cryptographically random
number so you will get random (non-deterministic) pauses on successive
backoff intervals.

```
interval, err := DefaultBinaryExponentialJitter()
if err != nil {
	// error likely due to crypto/rand io error
}
bo := backoff.NewBackoff(interval)
```

# Custom Exponential Series

You can configure the Exponential parameters to something that suits your
application:

```
e := Exponential{
	Base:    3 * time.Second,
	Unit:    time.Second,
	Initial: 1 * time.Second,
	Max:     30 * time.Second,
}
bo := backoff.NewBackoff(e)
```

produces a backoff series:

```
1s, 3s, 9s, 27s, 30s, 30s, ...
```

# Even if you fail, don't give up... (infinite tries)

You can configure `Try` to try forever:

```
err := bo.Try(ctx, backoff.InfiniteTries, func(ctx context.Context) bool {
	// your code
})
```

Obviously if your `Completable` func never returns `true` then this will try
forever.

# Caution

## Don't provide a non-cancellable Context

While you can provide a Context without a timeout or deadline with something
like `context.Background` it will create a possibility that `Try` will block
forever even if `tries` is finite. For this reason the **Synopsis** purposely
includes a context with a timeout. An example where this can happen is using
the `http.DefaultClient` without a timeout.

Similarly, in your provided `Completable` func, you should take care to
listen to the `ctx.Done()` channel when implementing your own routine. If the
called routine does properly support `Context` then you do not need to take
action.
