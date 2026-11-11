package rpccontract

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func comparisonManifest(t *testing.T) *ContractManifest {
	t.Helper()
	m, err := Parse(fixture(t))
	if err != nil {
		t.Fatal(err)
	}
	// Keep real canonical schemas but isolate the compared application surface.
	m.Endpoints = map[string]EndpointContract{}
	m.Commands = map[string]NamedCommandContract{}
	return m
}
func comparisonBytes(t *testing.T, m *ContractManifest) []byte {
	t.Helper()
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	var raw map[string]any
	if err := d.Decode(&raw); err != nil {
		t.Fatal(err)
	}
	return sign(t, raw)
}
func obj(required bool, s WireSchema) WireSchema {
	return WireSchema{Kind: "object", Properties: map[string]WireProperty{"value": {Schema: s, Required: required}}}
}
func integer(lo, hi int64) WireSchema     { return WireSchema{Kind: "integer", Minimum: &lo, Maximum: &hi} }
func arr(s WireSchema, n *int) WireSchema { return WireSchema{Kind: "array", Element: &s, Length: n} }
func ptr[T any](v T) *T                   { return &v }
func TestCompareDirectionalSchemas(t *testing.T) {
	str := WireSchema{Kind: "string"}
	nullable := str
	nullable.Nullable = true
	enum := func(v ...string) WireSchema { return WireSchema{Kind: "enum", Values: v} }
	union := func(v ...WireSchema) WireSchema { return WireSchema{Kind: "union", Variants: v} }
	cases := []struct {
		name              string
		a, b              WireSchema
		request, response Compatibility
	}{
		{"nullable", str, nullable, Compatible, PotentiallyBreaking},
		{"required", obj(false, str), obj(true, str), PotentiallyBreaking, Compatible},
		{"enum widening", enum("a"), enum("a", "b"), Compatible, PotentiallyBreaking},
		{"enum narrowing", enum("a", "b"), enum("a"), PotentiallyBreaking, Compatible},
		{"integer widening", integer(0, 10), integer(-1, 11), Compatible, PotentiallyBreaking},
		{"integer narrowing", integer(0, 10), integer(1, 9), PotentiallyBreaking, Compatible},
		{"number widening", integer(0, 10), WireSchema{Kind: "number"}, Compatible, PotentiallyBreaking},
		{"fixed array widening", arr(str, ptr(2)), arr(str, nil), Compatible, PotentiallyBreaking},
		{"array length", arr(str, ptr(2)), arr(str, ptr(3)), PotentiallyBreaking, PotentiallyBreaking},
		{"map value", WireSchema{Kind: "map", Element: &str}, WireSchema{Kind: "map", Element: &nullable}, Compatible, PotentiallyBreaking},
		{"union widening", union(str, WireSchema{Kind: "boolean"}), union(str, WireSchema{Kind: "boolean"}, WireSchema{Kind: "number"}), Compatible, PotentiallyBreaking},
		{"encoded string", str, WireSchema{Kind: "string", Encoding: "date-time"}, PotentiallyBreaking, PotentiallyBreaking},
		{"opaque widening", str, WireSchema{Kind: "json"}, Compatible, PotentiallyBreaking},
		{"optional removed", obj(false, str), WireSchema{Kind: "object"}, PotentiallyBreaking, PotentiallyBreaking},
	}
	for _, tc := range cases {
		for _, dir := range []string{"request", "response"} {
			t.Run(tc.name+"/"+dir, func(t *testing.T) {
				a, b := comparisonManifest(t), comparisonManifest(t)
				x, y := NamedCommandContract{Input: tc.a, ResultMode: "receipt"}, NamedCommandContract{Input: tc.b, ResultMode: "receipt"}
				want := tc.request
				if dir == "response" {
					x.Input = str
					y.Input = str
					x.Result = &tc.a
					y.Result = &tc.b
					x.ResultMode = "optional"
					y.ResultMode = "optional"
					want = tc.response
				}
				a.Commands["demo"] = x
				b.Commands["demo"] = y
				got, err := Compare(comparisonBytes(t, a), comparisonBytes(t, b))
				if err != nil {
					t.Fatal(err)
				}
				if got.Classification != want {
					t.Fatalf("got %s want %s: %+v", got.Classification, want, got.Changes)
				}
			})
		}
	}
}
func TestCompareAdditionsMetadataAndOutcomes(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*ContractManifest)
		want   Compatibility
		code   string
	}{
		{"new command", func(m *ContractManifest) { m.Commands["new"] = m.Commands["demo"] }, Additive, "command_added"},
		{"removed command", func(m *ContractManifest) { delete(m.Commands, "demo") }, PotentiallyBreaking, "command_removed"},
		{"metadata", func(m *ContractManifest) { c := m.Commands["demo"]; c.Description = "new docs"; m.Commands["demo"] = c }, Compatible, "metadata_changed"},
		{"result mode", func(m *ContractManifest) {
			c := m.Commands["demo"]
			c.ResultMode = "optional"
			c.Result = &WireSchema{Kind: "string"}
			m.Commands["demo"] = c
		}, PotentiallyBreaking, "result_mode_changed"},
		{"optional input", func(m *ContractManifest) {
			c := m.Commands["demo"]
			c.Input.Properties["new"] = WireProperty{Schema: WireSchema{Kind: "string"}}
			m.Commands["demo"] = c
		}, Additive, "property_added"},
		{"required input", func(m *ContractManifest) {
			c := m.Commands["demo"]
			c.Input.Properties["new"] = WireProperty{Schema: WireSchema{Kind: "string"}, Required: true}
			m.Commands["demo"] = c
		}, PotentiallyBreaking, "property_added"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, b := comparisonManifest(t), comparisonManifest(t)
			for _, m := range []*ContractManifest{a, b} {
				m.Commands["demo"] = NamedCommandContract{Input: obj(true, WireSchema{Kind: "string"}), ResultMode: "receipt"}
			}
			tc.mutate(b)
			got, err := Compare(comparisonBytes(t, a), comparisonBytes(t, b))
			if err != nil {
				t.Fatal(err)
			}
			if got.Classification != tc.want || len(got.Changes) != 1 || got.Changes[0].Code != tc.code {
				t.Fatalf("%+v", got)
			}
		})
	}
}
func TestCompareSharedRecursiveReferencesAndPaths(t *testing.T) {
	a, b := comparisonManifest(t), comparisonManifest(t)
	for _, m := range []*ContractManifest{a, b} {
		m.Definitions["node"] = WireSchema{Kind: "object", Properties: map[string]WireProperty{"next": {Schema: WireSchema{Kind: "ref", Ref: "node", Nullable: true}}, "count": {Schema: integer(0, 10), Required: true}}}
		ref := WireSchema{Kind: "ref", Ref: "node"}
		m.Commands["a/b~c"] = NamedCommandContract{Input: ref, Result: &ref, ResultMode: "optional"}
	}
	n := b.Definitions["node"]
	n.Properties["count"] = WireProperty{Schema: integer(0, 20), Required: true}
	b.Definitions["node"] = n
	before, after := comparisonBytes(t, a), comparisonBytes(t, b)
	savedA, savedB := bytes.Clone(before), bytes.Clone(after)
	var first []byte
	for i := 0; i < 20; i++ {
		got, err := Compare(before, after)
		if err != nil {
			t.Fatal(err)
		}
		if got.Classification != PotentiallyBreaking {
			t.Fatalf("%+v", got)
		}
		request, response := false, false
		for _, ch := range got.Changes {
			if !strings.HasPrefix(ch.Path, "/commands/a~1b~0c/") {
				t.Fatal(ch.Path)
			}
			request = request || ch.Direction == "request" && ch.Classification == Compatible
			response = response || ch.Direction == "response" && ch.Classification == PotentiallyBreaking
		}
		if !request || !response {
			t.Fatalf("missing directional shared changes: %+v", got)
		}
		raw, _ := json.Marshal(got)
		if i == 0 {
			first = raw
		} else if !bytes.Equal(first, raw) {
			t.Fatal("nondeterministic")
		}
	}
	if !bytes.Equal(savedA, before) || !bytes.Equal(savedB, after) {
		t.Fatal("mutated input")
	}
}
func TestCompareUnionReferencesAndFailedBranches(t *testing.T) {
	a, b := comparisonManifest(t), comparisonManifest(t)
	for _, m := range []*ContractManifest{a, b} {
		m.Definitions["choice"] = integer(0, 10)
		m.Commands["demo"] = NamedCommandContract{Input: WireSchema{Kind: "union", Variants: []WireSchema{{Kind: "ref", Ref: "choice"}, {Kind: "string"}}}, ResultMode: "receipt"}
	}
	b.Definitions["choice"] = integer(1, 9)
	got, err := Compare(comparisonBytes(t, a), comparisonBytes(t, b))
	if err != nil || got.Classification != PotentiallyBreaking {
		t.Fatalf("%+v %v", got, err)
	}
	// Failed branch proofs must not be reused as successful recursive proofs.
	source := obj(true, integer(0, 10))
	target := WireSchema{Kind: "union", Variants: []WireSchema{obj(true, integer(1, 9)), obj(true, integer(1, 9))}}
	if newSchemaRelation(a, b, "request").check(source, target) {
		t.Fatal("failed branch accepted")
	}
}
func TestCompareValidatesBothInputsAndCanonicalExtras(t *testing.T) {
	good := fixture(t)
	for _, bad := range [][]byte{[]byte(`{}`), append(bytes.Clone(good), []byte(` {}`)...), bytes.Replace(good, []byte(`"schemaVersion": "rpc-contract/v1"`), []byte(`"schemaVersion": "unsupported"`), 1)} {
		if bytes.Equal(good, bad) {
			t.Fatal("bad fixture unchanged")
		}
		for _, pair := range [][2][]byte{{good, bad}, {bad, good}} {
			if _, err := Compare(pair[0], pair[1]); err == nil {
				t.Fatal("invalid manifest accepted")
			}
		}
	}
	m := comparisonManifest(t)
	m.Canonical["extra"] = WireSchema{Kind: "ref", Ref: "missing"}
	if _, err := Compare(good, comparisonBytes(t, m)); err == nil {
		t.Fatal("invalid extra canonical accepted")
	}
	got, err := Compare(good, good)
	if err != nil || got.Classification != Unchanged || len(got.Changes) != 0 {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestCompareRemovalInsideUnion(t *testing.T) {
	for _, direction := range []string{"request", "response"} {
		a, b := comparisonManifest(t), comparisonManifest(t)
		old := obj(true, WireSchema{Kind: "string"})
		old.Properties["optional"] = WireProperty{Schema: WireSchema{Kind: "string"}}
		next := obj(true, WireSchema{Kind: "string"})
		oldUnion := WireSchema{Kind: "union", Variants: []WireSchema{old, {Kind: "boolean"}}}
		newUnion := WireSchema{Kind: "union", Variants: []WireSchema{next, {Kind: "boolean"}}}
		x, y := NamedCommandContract{Input: oldUnion, ResultMode: "receipt"}, NamedCommandContract{Input: newUnion, ResultMode: "receipt"}
		if direction == "response" {
			x.Input = WireSchema{Kind: "string"}
			y.Input = x.Input
			x.Result = &oldUnion
			y.Result = &newUnion
			x.ResultMode = "optional"
			y.ResultMode = "optional"
		}
		a.Commands["demo"] = x
		b.Commands["demo"] = y
		got, err := Compare(comparisonBytes(t, a), comparisonBytes(t, b))
		if err != nil || got.Classification != PotentiallyBreaking {
			t.Fatalf("%s: %+v %v", direction, got, err)
		}
	}
}

func TestCompareEndpointAndCanonicalSurfaces(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*ContractManifest)
		want   Compatibility
		code   string
	}{
		{"endpoint added", func(m *ContractManifest) {
			for _, e := range m.Endpoints {
				m.Endpoints["extra"] = e
				break
			}
		}, Additive, "endpoint_added"},
		{"endpoint removed", func(m *ContractManifest) { delete(m.Endpoints, "example.echo") }, PotentiallyBreaking, "endpoint_removed"},
		{"canonical response", func(m *ContractManifest) {
			n, _ := resolve(m, m.Canonical["Error"])
			p := n.Properties["code"]
			p.Required = false
			n.Properties["code"] = p
			m.Canonical["Error"] = n
		}, PotentiallyBreaking, "presence_changed"},
		{"exported alias removed", func(m *ContractManifest) { delete(m.Definitions, "unused") }, PotentiallyBreaking, "definition_removed"},
		{"unused changed", func(m *ContractManifest) { m.Definitions["unused"] = WireSchema{Kind: "boolean"} }, PotentiallyBreaking, "unused_definition_changed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, err := Parse(fixture(t))
			if err != nil {
				t.Fatal(err)
			}
			b, err := Parse(fixture(t))
			if err != nil {
				t.Fatal(err)
			}
			a.Definitions["unused"] = WireSchema{Kind: "string"}
			b.Definitions["unused"] = WireSchema{Kind: "string"}
			tc.change(b)
			got, err := Compare(comparisonBytes(t, a), comparisonBytes(t, b))
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, ch := range got.Changes {
				found = found || ch.Code == tc.code
			}
			if got.Classification != tc.want || !found {
				t.Fatalf("%+v", got)
			}
		})
	}
}
