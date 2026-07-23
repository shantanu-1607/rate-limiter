package limiter

import "context"

type Decision struct {
	Allowed    bool
	Remaining  float64
	ResetAfter float64
	Degraded   bool
}

type Limiter interface {
	Check(ctx context.Context, tenant string, cost float64) (Decision, error)
}
