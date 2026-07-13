package classad

import "testing"

// TestAggregate covers the value-slice reduction that backs collection
// aggregates, including the int/real/undefined/error semantics inherited from
// the list sum()/avg()/min()/max() functions.
func TestAggregate(t *testing.T) {
	ints := []Value{NewIntValue(2), NewIntValue(3), NewIntValue(5)}
	reals := []Value{NewIntValue(2), NewRealValue(2.5)}
	withUndef := []Value{NewIntValue(4), NewUndefinedValue(), NewIntValue(6)}
	withErr := []Value{NewIntValue(1), NewErrorValue()}

	// count ignores value types and counts everything.
	if v := Aggregate("count", withErr); !isInt(t, v, 2) {
		t.Errorf("count = %v, want 2", v)
	}
	if v := Aggregate("count", nil); !isInt(t, v, 0) {
		t.Errorf("count of empty = %v, want 0", v)
	}

	// sum stays integer when all contributors are ints; undefined is skipped.
	if v := Aggregate("sum", ints); !isInt(t, v, 10) {
		t.Errorf("sum ints = %v, want int 10", v)
	}
	if v := Aggregate("sum", withUndef); !isInt(t, v, 10) {
		t.Errorf("sum with undefined = %v, want int 10 (undefined skipped)", v)
	}
	if v := Aggregate("sum", nil); !isInt(t, v, 0) {
		t.Errorf("sum of empty = %v, want int 0", v)
	}
	// A real contributor makes the sum real.
	if v := Aggregate("sum", reals); !v.IsReal() {
		t.Errorf("sum with a real = %v, want a real", v)
	}
	// An error contributor makes the whole sum an error.
	if v := Aggregate("sum", withErr); !v.IsError() {
		t.Errorf("sum with an error = %v, want error", v)
	}

	// avg over {2,3,5} = 10/3.
	if v := Aggregate("avg", ints); !v.IsReal() {
		t.Errorf("avg = %v, want a real", v)
	}
	// min / max.
	if v := Aggregate("min", ints); !isInt(t, v, 2) {
		t.Errorf("min = %v, want 2", v)
	}
	if v := Aggregate("max", ints); !isInt(t, v, 5) {
		t.Errorf("max = %v, want 5", v)
	}
	// min/max of empty is undefined.
	if v := Aggregate("min", nil); !v.IsUndefined() {
		t.Errorf("min of empty = %v, want undefined", v)
	}
	// case-insensitive op; unknown op is error.
	if v := Aggregate("SUM", ints); !isInt(t, v, 10) {
		t.Errorf("SUM (upper) = %v, want int 10", v)
	}
	if v := Aggregate("median", ints); !v.IsError() {
		t.Errorf("unknown op = %v, want error", v)
	}
}

func isInt(t *testing.T, v Value, want int64) bool {
	t.Helper()
	got, err := v.IntValue()
	return err == nil && got == want
}
