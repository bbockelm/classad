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

// TestCollectionAggregate covers a batch of sum/avg/min/max/count over a
// filtered collection computed in a single scan (SELECT ... WHERE).
func TestCollectionAggregate(t *testing.T) {
	c := New(Options{Shards: 4})
	// Two "sites", each with slots of varying Cpus/Memory. SlotWeight is an
	// EXPRESSION (Cpus + Memory/1024) to prove the aggregate evaluates it.
	mustAdd(t, c, "a1", `[ Site="A"; Cpus=4; Memory=2048; SlotWeight=Cpus+Memory/1024 ]`)  // weight 6
	mustAdd(t, c, "a2", `[ Site="A"; Cpus=8; Memory=1024; SlotWeight=Cpus+Memory/1024 ]`)  // weight 9
	mustAdd(t, c, "b1", `[ Site="B"; Cpus=2; Memory=1024; SlotWeight=Cpus+Memory/1024 ]`)  // weight 3
	mustAdd(t, c, "b2", `[ Site="B"; Cpus=16; Memory=4096; SlotWeight=Cpus+Memory/1024 ]`) // weight 20

	siteA := mustQuery(t, `Site == "A"`)
	weight := mustQuery(t, `SlotWeight`)
	cpus := mustQuery(t, `Cpus`)

	// One pass over site A computing five aggregates at once.
	got := c.Aggregate(siteA, []AggSpec{
		{Op: "sum", Expr: weight}, // 6 + 9 = 15 (the accountant weighted-pool-size case)
		{Op: "avg", Expr: cpus},   // (4+8)/2 = 6
		{Op: "min", Expr: cpus},   // 4
		{Op: "max", Expr: cpus},   // 8
		{Op: "count"},             // 2 (expr nil is fine for count)
	})
	want := []float64{15, 6, 4, 8, 2}
	if len(got) != len(want) {
		t.Fatalf("got %d results, want %d", len(got), len(want))
	}
	for i, w := range want {
		if g := asFloat(t, got[i]); g != w {
			t.Errorf("spec[%d] = %v, want %v", i, g, w)
		}
	}

	// nil filter aggregates over the whole pool: sum(Cpus) = 4+8+2+16 = 30.
	all := c.Aggregate(nil, []AggSpec{{Op: "sum", Expr: cpus}, {Op: "count"}})
	if g := asFloat(t, all[0]); g != 30 {
		t.Errorf("sum(Cpus) over all = %v, want 30", g)
	}
	if g := asFloat(t, all[1]); g != 4 {
		t.Errorf("count over all = %v, want 4", g)
	}
}

// TestCollectionAggregateEmptyAndErrors covers the edge semantics inherited from
// the list aggregates, plus per-spec error isolation.
func TestCollectionAggregateEmptyAndErrors(t *testing.T) {
	c := New(Options{Shards: 2})
	mustAdd(t, c, "x", `[ Site="A"; Cpus=4 ]`)

	none := mustQuery(t, `Site == "Z"`) // matches nothing
	cpus := mustQuery(t, `Cpus`)

	// sum over no ads is int 0; count is 0; min over no ads is undefined.
	empty := c.Aggregate(none, []AggSpec{{Op: "sum", Expr: cpus}, {Op: "count"}, {Op: "min", Expr: cpus}})
	if g := asFloat(t, empty[0]); g != 0 {
		t.Errorf("sum over empty = %v, want 0", g)
	}
	if g := asFloat(t, empty[1]); g != 0 {
		t.Errorf("count over empty = %v, want 0", g)
	}
	if !empty[2].IsUndefined() {
		t.Errorf("min over empty = %v, want undefined", empty[2])
	}

	// Per-spec errors are isolated: a nil expr for a numeric op and an unknown op
	// error only in their own slot; a valid spec beside them still computes.
	mixed := c.Aggregate(nil, []AggSpec{
		{Op: "sum"},                // nil expr -> error
		{Op: "median", Expr: cpus}, // unknown op -> error
		{Op: "sum", Expr: cpus},    // valid -> 4
	})
	if !mixed[0].IsError() {
		t.Errorf("sum with nil expr = %v, want error", mixed[0])
	}
	if !mixed[1].IsError() {
		t.Errorf("unknown op = %v, want error", mixed[1])
	}
	if g := asFloat(t, mixed[2]); g != 4 {
		t.Errorf("valid sum beside errors = %v, want 4", g)
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
	got := c.Aggregate(mustQuery(t, `Owner == "u0"`), []AggSpec{{Op: "sum", Expr: mustQuery(t, `Cpus`)}})
	if g := asFloat(t, got[0]); g != want {
		t.Errorf("sum(Cpus) WHERE Owner==u0 = %v, want %v", g, want)
	}
}
