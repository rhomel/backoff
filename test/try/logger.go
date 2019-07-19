package try

import (
	"context"
	"time"
)

type Events struct {
	Events []string
}

const (
	CaseDone        = "case:ctx.Done"
	CaseAfter       = "case:time.After"
	CaseReturnFalse = "return:false"
	CaseReturnTrue  = "return:true"
)

// log the events as Completable gets called in `try`
func FnLogger(delay time.Duration, trueAfterN int) (*Events, func(ctx context.Context) bool) {
	i := 0
	e := &Events{}
	return e, func(ctx context.Context) bool {
		select {
		case <-ctx.Done():
			e.Events = append(e.Events, CaseDone)
			e.Events = append(e.Events, CaseReturnFalse)
			return false
		case <-time.After(delay):
			e.Events = append(e.Events, CaseAfter)
			if i >= trueAfterN {
				e.Events = append(e.Events, CaseReturnTrue)
				return true
			}
			i++
			e.Events = append(e.Events, CaseReturnFalse)
			return false
		}
	}
}
