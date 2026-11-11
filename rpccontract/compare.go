package rpccontract

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Compatibility describes structural old-client/new-server compatibility only.
// Neither a compatible report nor a retry hint establishes semantic safety.
type Compatibility string

const (
	Unchanged           Compatibility = "unchanged"
	Compatible          Compatibility = "compatible"
	Additive            Compatibility = "additive"
	PotentiallyBreaking Compatibility = "potentially_breaking"
)

// Change identifies a difference using a JSON pointer into the manifest. Paths
// beneath references describe the resolved schema at its first sorted depth-first
// usage path per root/direction. Shared paths are not exhaustively enumerated.
type Change struct {
	Path           string        `json:"path"`
	Direction      string        `json:"direction"`
	Code           string        `json:"code"`
	Classification Compatibility `json:"classification"`
	Message        string        `json:"message"`
}

// Comparison is a versioned structural report, not a semantic compatibility verdict.
type Comparison struct {
	SchemaVersion        string        `json:"schemaVersion"`
	BaselineFingerprint  string        `json:"baselineFingerprint"`
	CandidateFingerprint string        `json:"candidateFingerprint"`
	Classification       Compatibility `json:"classification"`
	Changes              []Change      `json:"changes"`
}

// Compare validates both manifests before comparing their wire surfaces. It is
// side-effect free. Unproven inclusion is reported as potentially breaking;
// structural comparison cannot establish authorization or semantic compatibility.
func Compare(baseline, candidate []byte) (*Comparison, error) {
	a, err := Parse(baseline)
	if err != nil {
		return nil, fmt.Errorf("baseline: %w", err)
	}
	b, err := Parse(candidate)
	if err != nil {
		return nil, fmt.Errorf("candidate: %w", err)
	}
	// Parse's required canonical entries are validated by generation. Extra entries
	// are also part of comparison and must not bypass schema validation.
	for _, m := range []*ContractManifest{a, b} {
		for _, k := range keys(m.Canonical) {
			if err := node(m, m.Canonical[k], "canonical."+k, 0); err != nil {
				return nil, err
			}
			if err := aliasCycle(m, m.Canonical[k], map[string]bool{}); err != nil {
				return nil, err
			}
		}
	}
	c := comparer{a: a, b: b, equal: newSchemaRelation(a, b, "equal"), request: newSchemaRelation(a, b, "request"), response: newSchemaRelation(b, a, "response"), report: Comparison{SchemaVersion: "rpc-contract-comparison/v1", BaselineFingerprint: a.Fingerprint, CandidateFingerprint: b.Fingerprint, Classification: Unchanged, Changes: []Change{}}}
	for _, id := range mergedKeys(a.Endpoints, b.Endpoints) {
		x, ax := a.Endpoints[id]
		y, by := b.Endpoints[id]
		path := pointer("/endpoints", id)
		if !ax {
			c.add(path, "surface", "endpoint_added", Additive, "Endpoint added.")
			continue
		}
		if !by {
			c.add(path, "surface", "endpoint_removed", PotentiallyBreaking, "Endpoint removed.")
			continue
		}
		c.walk(x.Request, y.Request, path+"/request", "request", map[string]bool{})
		c.walk(x.Response, y.Response, path+"/response", "response", map[string]bool{})
	}
	for _, id := range mergedKeys(a.Commands, b.Commands) {
		x, ax := a.Commands[id]
		y, by := b.Commands[id]
		path := pointer("/commands", id)
		if !ax {
			c.add(path, "surface", "command_added", Additive, "Named command added.")
			continue
		}
		if !by {
			c.add(path, "surface", "command_removed", PotentiallyBreaking, "Named command removed.")
			continue
		}
		c.walk(x.Input, y.Input, path+"/input", "request", map[string]bool{})
		if x.ResultMode != y.ResultMode {
			c.add(path+"/resultMode", "response", "result_mode_changed", PotentiallyBreaking, "Receipt/result contract changed; review outcome handling.")
		}
		if x.Result != nil && y.Result != nil {
			c.walk(*x.Result, *y.Result, path+"/result", "response", map[string]bool{})
		}
		if x.Summary != y.Summary || x.Description != y.Description {
			c.add(path, "metadata", "metadata_changed", Compatible, "Command documentation changed.")
		}
	}
	directions := map[string]string{"RequestMeta": "request", "DispatchOptions": "request", "CommandDispatchRequest": "request", "Error": "response", "DispatchReceipt": "response", "CommandDispatchResponse": "response"}
	for _, id := range mergedKeys(a.Canonical, b.Canonical) {
		x, ax := a.Canonical[id]
		y, by := b.Canonical[id]
		path := pointer("/canonical", id)
		if !ax {
			c.add(path, "surface", "canonical_added", Additive, "Canonical declaration added.")
			continue
		}
		if !by {
			c.add(path, "surface", "canonical_removed", PotentiallyBreaking, "Canonical declaration removed.")
			continue
		}
		d, known := directions[id]
		if known {
			c.walk(x, y, path, d, map[string]bool{})
		} else if !c.equal.check(x, y) {
			c.add(path, "unknown", "canonical_changed", PotentiallyBreaking, "Unknown canonical usage direction requires review.")
		}
	}
	usedA, usedB := reachable(a), reachable(b)
	for _, id := range mergedKeys(a.Definitions, b.Definitions) {
		x, ax := a.Definitions[id]
		y, by := b.Definitions[id]
		path := pointer("/definitions", id)
		if !ax {
			c.add(path, "surface", "definition_added", Additive, "Exported definition added.")
		} else if !by {
			c.add(path, "surface", "definition_removed", PotentiallyBreaking, "Exported definition removed or renamed.")
		} else if !usedA[id] && !usedB[id] && !sameNode(x, y) {
			c.add(path, "unknown", "unused_definition_changed", PotentiallyBreaking, "Definition has no known wire usage direction; review direct consumers.")
		}
	}
	if len(c.report.Changes) == 0 && a.Fingerprint != b.Fingerprint {
		c.add("", "metadata", "representation_changed", Compatible, "Manifest representation changed without a detected wire-surface change.")
	}
	sort.Slice(c.report.Changes, func(i, j int) bool {
		x, y := c.report.Changes[i], c.report.Changes[j]
		if x.Path != y.Path {
			return x.Path < y.Path
		}
		if x.Direction != y.Direction {
			return x.Direction < y.Direction
		}
		return x.Code < y.Code
	})
	return &c.report, nil
}

type comparer struct {
	a, b                     *ContractManifest
	report                   Comparison
	equal, request, response *schemaRelation
}

func rank(c Compatibility) int {
	switch c {
	case Compatible:
		return 1
	case Additive:
		return 2
	case PotentiallyBreaking:
		return 3
	}
	return 0
}
func (c *comparer) add(path, d, code string, classification Compatibility, message string) {
	c.report.Changes = append(c.report.Changes, Change{path, d, code, classification, message})
	if rank(classification) > rank(c.report.Classification) {
		c.report.Classification = classification
	}
}
func pointer(path, key string) string {
	return path + "/" + strings.ReplaceAll(strings.ReplaceAll(key, "~", "~0"), "/", "~1")
}
func mergedKeys[V any](a, b map[string]V) []string {
	m := map[string]bool{}
	for k := range a {
		m[k] = true
	}
	for k := range b {
		m[k] = true
	}
	return keys(m)
}
func pairKey(a, b WireSchema) string {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) + "\n" + string(y)
}

func (c *comparer) walk(a, b WireSchema, path, d string, seen map[string]bool) {
	// Expand each reference pair once per root/direction. Inline property paths
	// remain distinct; shared graphs use the first sorted depth-first usage path.
	if a.Kind == "ref" || b.Kind == "ref" {
		key := pairKey(a, b)
		if seen[key] {
			return
		}
		seen[key] = true
	}
	a, _ = resolve(c.a, a)
	b, _ = resolve(c.b, b)
	classification := Compatible
	safe := func() bool {
		if d == "request" {
			return c.request.check(a, b)
		}
		return c.response.check(b, a)
	}
	if a.Kind != b.Kind || a.Kind == "union" || a.Kind == "enum" {
		if !sameNode(a, b) || a.Kind == "union" { // Equal refs in unions may resolve differently.
			if !c.equal.check(a, b) {
				if !safe() {
					classification = PotentiallyBreaking
				}
				c.add(path, d, "schema_changed", classification, "Value alternatives changed; classification follows request/response direction.")
			}
		}
		return
	}
	if a.Nullable != b.Nullable {
		if (d == "request" && a.Nullable) || (d == "response" && b.Nullable) {
			classification = PotentiallyBreaking
		}
		c.add(path+"/nullable", d, "nullability_changed", classification, "Allowed null values changed.")
	}
	if a.Encoding != b.Encoding {
		c.add(path+"/encoding", d, "encoding_changed", PotentiallyBreaking, "String encoding changed; review wire interpretation.")
	}
	switch a.Kind {
	case "object":
		for _, name := range mergedKeys(a.Properties, b.Properties) {
			x, ax := a.Properties[name]
			y, by := b.Properties[name]
			p := pointer(path+"/properties", name)
			if !ax {
				cl := Additive
				if (d == "request" && y.Required) || (d == "response" && len(a.Properties) == 0) {
					cl = PotentiallyBreaking
				}
				c.add(p, d, "property_added", cl, "Declared property added.")
				continue
			}
			if !by {
				c.add(p, d, "property_removed", PotentiallyBreaking, "Declared property removed; existing consumers may depend on it.")
				continue
			}
			if x.Required != y.Required {
				cl := Compatible
				if (d == "request" && y.Required) || (d == "response" && x.Required) {
					cl = PotentiallyBreaking
				}
				c.add(p+"/required", d, "presence_changed", cl, "Serialized property presence changed; this is not runtime validation.")
			}
			c.walk(x.Schema, y.Schema, p+"/schema", d, seen)
		}
	case "array", "map":
		if !equalInt(a.Length, b.Length) {
			cl := Compatible
			if !safe() {
				cl = PotentiallyBreaking
			}
			c.add(path+"/length", d, "length_changed", cl, "Array length constraint changed.")
		}
		c.walk(*a.Element, *b.Element, path+"/element", d, seen)
	case "integer":
		if *a.Minimum != *b.Minimum || *a.Maximum != *b.Maximum {
			cl := Compatible
			if !safe() {
				cl = PotentiallyBreaking
			}
			c.add(path, d, "bounds_changed", cl, "Integer range changed.")
		}
	}
}
func equalInt(a, b *int) bool { return a == nil && b == nil || a != nil && b != nil && *a == *b }

func reachable(m *ContractManifest) map[string]bool {
	seen := map[string]bool{}
	var visit func(WireSchema)
	visit = func(s WireSchema) {
		if s.Kind == "ref" {
			if seen[s.Ref] {
				return
			}
			seen[s.Ref] = true
			visit(m.Definitions[s.Ref])
		}
		for _, p := range s.Properties {
			visit(p.Schema)
		}
		if s.Element != nil {
			visit(*s.Element)
		}
		for _, v := range s.Variants {
			visit(v)
		}
	}
	for _, e := range m.Endpoints {
		visit(e.Request)
		visit(e.Response)
	}
	for _, c := range m.Commands {
		visit(c.Input)
		if c.Result != nil {
			visit(*c.Result)
		}
	}
	for _, s := range m.Canonical {
		visit(s)
	}
	return seen
}
