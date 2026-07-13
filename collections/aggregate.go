package collections

import (
	"strings"

	"github.com/PelicanPlatform/classad/classad"
	"github.com/PelicanPlatform/classad/collections/vm"
)

// AggSpec is one aggregation to compute over a collection: a reduction Op
// ("sum"/"avg"/"min"/"max"/"count", case-insensitive) applied to Expr evaluated
// per matching ad. Expr may be nil only for "count".
type AggSpec struct {
	Op   string
	Expr *vm.Query
}

// Aggregate computes every spec over the ads matching filter in a SINGLE scan of
// the collection -- the collection analogue of the sum()/avg()/min()/max()
// ClassAd functions: SELECT <op1>(expr1), <op2>(expr2), ... WHERE <filter>. It
// returns one result value per spec, in order. A nil filter aggregates over
// every ad (Scan).
//
// Each spec's Expr is evaluated against each matching ad (MY.* / bare references
// resolve in the ad; there is no TARGET), and the per-ad values are reduced with
// classad.Aggregate, so the int/real/undefined/error semantics are identical to
// the list aggregates. A spec with an unknown Op, or a nil Expr for a non-count
// op, yields an error value in its slot.
//
//	c.Aggregate(prodQuery, []AggSpec{         // one pass, three results
//	    {Op: "sum",   Expr: slotWeightExpr},  // weighted size of a group
//	    {Op: "count"},                        // number of slots
//	    {Op: "max",   Expr: cpusExpr},        // largest slot
//	})
//
// The scan is the same single-threaded, scan-exactly-once iteration as Query, so
// a filter/expr that references a runtime name falls back to full decode
// transparently.
func (c *Collection) Aggregate(filter *vm.Query, specs []AggSpec) []classad.Value {
	seq := c.Scan()
	if filter != nil {
		seq = c.Query(filter)
	}

	// One Matcher per numeric spec (reused across ads; the scan is
	// single-threaded); "count" needs only the ad count, not its expr.
	matchers := make([]*vm.Matcher, len(specs))
	vals := make([][]classad.Value, len(specs))
	for i, s := range specs {
		if !strings.EqualFold(s.Op, "count") && s.Expr != nil {
			matchers[i] = s.Expr.Matcher()
		}
	}

	var count int64
	for ad := range seq {
		count++
		for i := range specs {
			if matchers[i] != nil {
				vals[i] = append(vals[i], matchers[i].Eval(ad))
			}
		}
	}

	out := make([]classad.Value, len(specs))
	for i, s := range specs {
		switch {
		case strings.EqualFold(s.Op, "count"):
			out[i] = classad.NewIntValue(count)
		case s.Expr == nil:
			out[i] = classad.NewErrorValue()
		default:
			out[i] = classad.Aggregate(s.Op, vals[i])
		}
	}
	return out
}
