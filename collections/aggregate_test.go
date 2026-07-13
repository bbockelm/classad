package collections

import (
	"fmt"
	"testing"

	"github.com/PelicanPlatform/classad/classad"
)

func mustAdd(t *testing.T, c *Collection, key, ad string) {
	t.Helper()
	a, err := classad.Parse(ad)
	if err != nil {
		t.Fatalf("classad.Parse(%q): %v", ad, err)
	}
	if err := c.Put([]byte(key), a); err != nil {
		t.Fatalf("Put: %v", err)
	}
}

func asFloat(t *testing.T, v classad.Value) float64 {
	t.Helper()
	f, err := v.NumberValue()
	if err != nil {
		t.Fatalf("value %v is not numeric: %v", v, err)
	}
	return f
}

// TestCollectionAggregate covers sum/avg/min/max/count over a filtered
// collection, the SELECT <op>(expr) WHERE <filter> primitive.
func TestCollectionAggregate(t *testing.T) {
	c := New(Options{Shards: 4})
	// Two "sites", each with slots of varying Cpus/Memory. SlotWeight is an
	// EXPRESSION (Cpus + Memory/1024) to prove the aggregate evaluates it.
	mustAdd(t, c, "a1", `[ Site="A"; Cpus=4; Memory=2048; SlotWeight=Cpus+Memory/1024 ]`) // weight 6
	mustAdd(t, c, "a2", `[ Site="A"; Cpus=8; Memory=1024; SlotWeight=Cpus+Memory/1024 ]`) // weight 9
	mustAdd(t, c, "b1", `[ Site="B"; Cpus=2; Memory=1024; SlotWeight=Cpus+Memory/1024 ]`) // weight 3
	mustAdd(t, c, "b2", `[ Site="B"; Cpus=16; Memory=4096; SlotWeight=Cpus+Memory/1024 ]`) // weight 20

	siteA := mustQuery(t, `Site == "A"`)
	weight := mustQuery(t, `SlotWeight`)
	cpus := mustQuery(t, `Cpus`)

	// Weighted pool size of site A = 6 + 9 = 15 (the exact accountant use case).
	if got := asFloat(t, c.Aggregate(siteA, weight, "sum")); got != 15 {
		t.Errorf("sum(SlotWeight) WHERE Site==A = %v, want 15", got)
	}
	// Total Cpus across the whole pool (nil filter) = 4+8+2+16 = 30.
	if got := asFloat(t, c.Aggregate(nil, cpus, "sum")); got != 30 {
		t.Errorf("sum(Cpus) over all = %v, want 30", got)
	}
	// avg / min / max of Cpus in site A over {4,8}.
	if got := asFloat(t, c.Aggregate(siteA, cpus, "avg")); got != 6 {
		t.Errorf("avg(Cpus) WHERE Site==A = %v, want 6", got)
	}
	if got := asFloat(t, c.Aggregate(siteA, cpus, "min")); got != 4 {
		t.Errorf("min(Cpus) WHERE Site==A = %v, want 4", got)
	}
	if got := asFloat(t, c.Aggregate(siteA, cpus, "max")); got != 8 {
		t.Errorf("max(Cpus) WHERE Site==A = %v, want 8", got)
	}
	// count of site-A ads = 2; count over all = 4 (expr may be nil for count).
	if got := asFloat(t, c.Aggregate(siteA, nil, "count")); got != 2 {
		t.Errorf("count WHERE Site==A = %v, want 2", got)
	}
	if got := asFloat(t, c.Aggregate(nil, nil, "count")); got != 4 {
		t.Errorf("count over all = %v, want 4", got)
	}
}

// TestCollectionAggregateEmptyAndErrors covers the edge semantics inherited from
// the list aggregates.
func TestCollectionAggregateEmptyAndErrors(t *testing.T) {
	c := New(Options{Shards: 2})
	mustAdd(t, c, "x", `[ Site="A"; Cpus=4 ]`)

	none := mustQuery(t, `Site == "Z"`) // matches nothing
	cpus := mustQuery(t, `Cpus`)

	// sum over no ads is int 0; count is 0; min over no ads is undefined.
	if got := asFloat(t, c.Aggregate(none, cpus, "sum")); got != 0 {
		t.Errorf("sum over empty = %v, want 0", got)
	}
	if got := asFloat(t, c.Aggregate(none, nil, "count")); got != 0 {
		t.Errorf("count over empty = %v, want 0", got)
	}
	if v := c.Aggregate(none, cpus, "min"); !v.IsUndefined() {
		t.Errorf("min over empty = %v, want undefined", v)
	}
	// A nil expr for a numeric op is an error (only count allows nil expr).
	if v := c.Aggregate(nil, nil, "sum"); !v.IsError() {
		t.Errorf("sum with nil expr = %v, want error", v)
	}
	// An unknown op is an error.
	if v := c.Aggregate(nil, cpus, "median"); !v.IsError() {
		t.Errorf("unknown op = %v, want error", v)
	}
}

// TestCollectionAggregateScale sanity-checks a larger collection.
func TestCollectionAggregateScale(t *testing.T) {
	c := New(Options{Shards: 8})
	var want float64
	for i := 0; i < 1000; i++ {
		mustAdd(t, c, fmt.Sprintf("k%d", i), fmt.Sprintf(`[ Owner="u%d"; Cpus=%d ]`, i%5, i%8))
		if i%5 == 0 { // Owner u0
			want += float64(i % 8)
		}
	}
	if got := asFloat(t, c.Aggregate(mustQuery(t, `Owner == "u0"`), mustQuery(t, `Cpus`), "sum")); got != want {
		t.Errorf("sum(Cpus) WHERE Owner==u0 = %v, want %v", got, want)
	}
}
