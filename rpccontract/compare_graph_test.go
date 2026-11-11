package rpccontract

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestUnknownCanonicalGraphChanges(t *testing.T) {
	for _, shape := range []string{"direct", "reference", "transitive", "recursive", "known-and-unknown"} {
		t.Run(shape, func(t *testing.T) {
			a, b := comparisonManifest(t), comparisonManifest(t)
			for _, m := range []*ContractManifest{a, b} {
				m.Definitions["leaf"] = integer(0, 10)
				entry := WireSchema{Kind: "ref", Ref: "leaf"}
				switch shape {
				case "direct":
					entry = integer(0, 10)
				case "transitive":
					m.Definitions["alias"] = entry
					entry = WireSchema{Kind: "ref", Ref: "alias"}
				case "recursive":
					m.Definitions["node"] = WireSchema{Kind: "object", Properties: map[string]WireProperty{
						"next":  {Schema: WireSchema{Kind: "ref", Ref: "node", Nullable: true}},
						"value": {Schema: entry, Required: true},
					}}
					entry = WireSchema{Kind: "ref", Ref: "node"}
				case "known-and-unknown":
					m.Commands["read"] = NamedCommandContract{Input: entry, ResultMode: "receipt"}
				}
				m.Canonical["Custom/Contract"] = entry
			}
			before := comparisonBytes(t, a)
			equal, err := Compare(before, comparisonBytes(t, b))
			if err != nil || equal.Classification != Unchanged {
				t.Fatalf("unchanged: %+v %v", equal, err)
			}
			if shape == "direct" {
				b.Canonical["Custom/Contract"] = integer(0, 20)
			} else {
				b.Definitions["leaf"] = integer(0, 20)
			}
			got, err := Compare(before, comparisonBytes(t, b))
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, ch := range got.Changes {
				if ch.Path == "/canonical/Custom~1Contract" && ch.Code == "canonical_changed" && ch.Direction == "unknown" && ch.Classification == PotentiallyBreaking {
					found = true
				}
			}
			if !found || got.Classification != PotentiallyBreaking {
				t.Fatalf("missed unknown-direction change: %+v", got)
			}
		})
	}
}

func sharedGraph(m *ContractManifest, depth int) WireSchema {
	m.Definitions["leaf"] = integer(0, 10)
	ref := WireSchema{Kind: "ref", Ref: "leaf"}
	for i := 0; i < depth; i++ {
		name := fmt.Sprintf("node%02d", i)
		m.Definitions[name] = WireSchema{Kind: "object", Properties: map[string]WireProperty{
			"left": {Schema: ref, Required: true}, "right": {Schema: ref, Required: true},
		}}
		ref = WireSchema{Kind: "ref", Ref: name}
	}
	return ref
}

// Depth 28 has over 250 million expanded paths but only 29 definitions. Assert
// operation counts/report size instead of relying on machine-specific timings.
func TestSharedGraphComparisonIsBounded(t *testing.T) {
	const depth = 28
	a, b := comparisonManifest(t), comparisonManifest(t)
	var ref WireSchema
	for _, m := range []*ContractManifest{a, b} {
		ref = sharedGraph(m, depth)
		m.Commands["dag"] = NamedCommandContract{Input: ref, Result: &ref, ResultMode: "optional"}
	}
	before := comparisonBytes(t, a)
	got, err := Compare(before, comparisonBytes(t, b))
	if err != nil || got.Classification != Unchanged {
		t.Fatalf("%+v %v", got, err)
	}
	c := b.Commands["dag"]
	c.Summary = "docs only"
	b.Commands["dag"] = c
	got, err = Compare(before, comparisonBytes(t, b))
	if err != nil || got.Classification != Compatible || len(got.Changes) != 1 {
		t.Fatalf("metadata: %+v %v", got, err)
	}
	b.Definitions["leaf"] = integer(0, 20)
	// Known request widening and response widening must retain opposite judgments.
	got, err = Compare(before, comparisonBytes(t, b))
	if err != nil {
		t.Fatal(err)
	}
	if got.Classification != PotentiallyBreaking || len(got.Changes) != 3 {
		t.Fatalf("unbounded or missing changes: %+v", got)
	}
	path := strings.Repeat("/properties/left/schema", depth)
	found := map[string]bool{}
	for _, ch := range got.Changes {
		if ch.Code == "bounds_changed" {
			suffix := "/commands/dag/input" + path
			want := Compatible
			if ch.Direction == "response" {
				suffix = "/commands/dag/result" + path
				want = PotentiallyBreaking
			}
			if ch.Path != suffix || ch.Classification != want {
				t.Fatalf("unexpected representative path/class: %+v", ch)
			}
			found[ch.Direction] = true
		}
	}
	if !found["request"] || !found["response"] {
		t.Fatal("direction conflated", got)
	}
	raw, _ := json.Marshal(got)
	repeat, err := Compare(before, comparisonBytes(t, b))
	if err != nil {
		t.Fatal(err)
	}
	again, _ := json.Marshal(repeat)
	if string(raw) != string(again) {
		t.Fatal("unstable paths/order")
	}
	// Bound graph analysis directly, including memo reuse across completed queries.
	for _, mode := range []string{"equal", "request", "response"} {
		source, target := a, b
		if mode == "response" {
			source, target = b, a
		}
		r := newSchemaRelation(source, target, mode)
		want := mode == "request"
		if r.check(ref, ref) != want {
			t.Fatal("wrong relation", mode)
		}
		count := len(r.nodes)
		if count > depth+2 {
			t.Fatalf("%s expanded %d pairs", mode, count)
		}
		if r.check(ref, ref) != want || len(r.nodes) != count {
			t.Fatal("completed-pair cache not reused", mode)
		}
	}
	// Force equality/inclusion through union nodes, not just the report walker.
	union := WireSchema{Kind: "union", Variants: []WireSchema{ref, {Kind: "string"}}}
	for _, mode := range []string{"equal", "request", "response"} {
		source, target := a, b
		if mode == "response" {
			source, target = b, a
		}
		r := newSchemaRelation(source, target, mode)
		if r.check(union, union) != (mode == "request") {
			t.Fatal("wrong union relation", mode)
		}
		if len(r.nodes) > 8*(depth+1) {
			t.Fatalf("%s expanded union graph: %d", mode, len(r.nodes))
		}
	}
}

func TestRecursiveRelationFailurePropagationAndMemoReuse(t *testing.T) {
	a, b := comparisonManifest(t), comparisonManifest(t)
	for _, m := range []*ContractManifest{a, b} {
		m.Definitions["A"] = WireSchema{Kind: "object", Properties: map[string]WireProperty{
			"cycle": {Schema: WireSchema{Kind: "ref", Ref: "B"}}, "later": {Schema: integer(0, 10)},
		}}
		m.Definitions["B"] = obj(false, WireSchema{Kind: "ref", Ref: "A"})
	}
	changed := b.Definitions["A"]
	changed.Properties["later"] = WireProperty{Schema: integer(1, 9)}
	b.Definitions["A"] = changed
	ar, br := WireSchema{Kind: "ref", Ref: "A"}, WireSchema{Kind: "ref", Ref: "B"}
	for _, mode := range []string{"equal", "request"} {
		r := newSchemaRelation(a, b, mode)
		// B appears provisionally equal/included while A is on the DFS stack. A's
		// later numeric mismatch must invalidate B too, including future queries.
		if r.check(ar, ar) || r.check(br, br) {
			t.Fatal("cached provisional cycle truth", mode)
		}
		bad := WireSchema{Kind: "union", Variants: []WireSchema{ar, br}}
		if r.check(ar, bad) {
			t.Fatal("failed recursive union branch accepted", mode)
		}
		good := WireSchema{Kind: "union", Variants: []WireSchema{ar, {Kind: "json"}}}
		if mode == "request" && !r.check(ar, good) {
			t.Fatal("valid alternate union branch rejected")
		}
	}
	equal := newSchemaRelation(a, a, "equal")
	if !equal.check(ar, ar) || !equal.check(br, br) {
		t.Fatal("productive cycle rejected")
	}
}

func TestSharedUnionAliasValidationIsBoundedAndRejectsCycles(t *testing.T) {
	m := comparisonManifest(t)
	m.Definitions["aliasLeaf"] = WireSchema{Kind: "string"}
	ref := WireSchema{Kind: "ref", Ref: "aliasLeaf"}
	for i := 0; i < 28; i++ {
		name := fmt.Sprintf("union%02d", i)
		m.Definitions[name] = WireSchema{Kind: "union", Variants: []WireSchema{ref, ref}}
		ref = WireSchema{Kind: "ref", Ref: name}
	}
	m.Commands["aliases"] = NamedCommandContract{Input: ref, ResultMode: "receipt"}
	raw := comparisonBytes(t, m)
	got, err := Compare(raw, raw)
	if err != nil || got.Classification != Unchanged {
		t.Fatalf("%+v %v", got, err)
	}
	if _, err := Generate(raw); err != nil {
		t.Fatal(err)
	}
	// Replacing the leaf with an ancestor introduces an unproductive alias cycle,
	// even though every other path and duplicate branch has already been checked.
	m.Definitions["aliasLeaf"] = ref
	if _, err := Compare(raw, comparisonBytes(t, m)); err == nil || !strings.Contains(err.Error(), "cyclic reference") {
		t.Fatalf("cycle accepted: %v", err)
	}
}
