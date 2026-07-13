package classad

import "strings"

// Aggregate reduces a slice of already-evaluated values with a named aggregate,
// using the SAME semantics as the sum()/avg()/min()/max() ClassAd functions
// applied to a list literal:
//
//   - "sum"   integer/real accumulation (int stays exact until a real appears);
//     undefined elements are skipped, an error element yields error, sum of no
//     contributing values is int 0.
//   - "avg"   sum divided by the count of contributing values; empty or
//     all-undefined is int 0 (mirroring the list avg()).
//   - "min" / "max"  the least / greatest value; empty is undefined.
//   - "count" the number of values (an aggregate the list functions do not have,
//     but the natural collection reduction; error/undefined values still count).
//
// It is the reduction half of a collection aggregate: a caller evaluates an
// expression over each matching ad, collects the values, and applies this. An
// unknown op yields error.
func Aggregate(op string, values []Value) Value {
	switch strings.ToLower(op) {
	case "count":
		return NewIntValue(int64(len(values)))
	case "sum":
		return builtinSum([]Value{NewListValue(values)})
	case "avg":
		return builtinAvg([]Value{NewListValue(values)})
	case "min":
		return builtinMin([]Value{NewListValue(values)})
	case "max":
		return builtinMax([]Value{NewListValue(values)})
	default:
		return NewErrorValue()
	}
}
