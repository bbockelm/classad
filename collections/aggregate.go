package collections

import (
	"strings"

	"github.com/PelicanPlatform/classad/classad"
	"github.com/PelicanPlatform/classad/collections/vm"
)

// Aggregate reduces a compiled expression over the ads of c that match filter,
// the collection analogue of the sum()/avg()/min()/max() ClassAd functions:
// SELECT <op>(expr) WHERE <filter>. A nil filter aggregates over every ad
// (Scan); a nil expr is allowed only for op "count".
//
// expr is evaluated against each matching ad (MY.* / bare references resolve in
// the ad; there is no TARGET). The per-ad values are reduced with
// classad.Aggregate, so the int/real/undefined/error semantics are identical to
// the list aggregates:
//
//	c.Aggregate(nil, cpusExpr, "sum")                  // total Cpus over all ads
//	c.Aggregate(prodQuery, slotWeightExpr, "sum")      // weighted size of a group
//	c.Aggregate(gpuQuery, nil, "count")                // how many GPU slots
//
// The scan is the same single-threaded, scan-exactly-once iteration as Query, so
// an unknown op returns error and a filter/expr that references a runtime name
// falls back to full decode transparently.
func (c *Collection) Aggregate(filter *vm.Query, expr *vm.Query, op string) classad.Value {
	seq := c.Scan()
	if filter != nil {
		seq = c.Query(filter)
	}

	if strings.EqualFold(op, "count") {
		var n int64
		for range seq {
			n++
		}
		return classad.NewIntValue(n)
	}

	if expr == nil {
		return classad.NewErrorValue()
	}
	m := expr.Matcher() // reused across ads; the scan is single-threaded
	var vals []classad.Value
	for ad := range seq {
		vals = append(vals, m.Eval(ad))
	}
	return classad.Aggregate(op, vals)
}
