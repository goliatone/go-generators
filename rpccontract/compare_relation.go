package rpccontract

// schemaRelation proves equality or directional inclusion as a graph of positive
// boolean obligations. Cycles use the greatest fixed point: start true, then
// propagate concrete failures. Caching a DFS's provisional true would be unsound
// when another member of that cycle fails later or a union branch is rejected.
// Each pair is built once; each node can become false once. Memo tables belong to
// one comparison and fixed manifest ordering/direction, never to global state.
type schemaRelation struct {
	source, target *ContractManifest
	mode           string // equal, request (old -> new), response (new -> old)
	nodes          map[string]*relationNode
	pending        []*relationNode
}

type relationNode struct {
	valid        bool
	any          bool // otherwise every dependency must hold
	value        bool
	dependencies []*relationNode
	parents      []*relationNode
}

func newSchemaRelation(source, target *ContractManifest, mode string) *schemaRelation {
	return &schemaRelation{source: source, target: target, mode: mode, nodes: map[string]*relationNode{}}
}

func (r *schemaRelation) check(source, target WireSchema) bool {
	root := r.build(source, target)
	queue := []*relationNode{}
	for _, n := range r.pending {
		if n.value && !n.holds() {
			n.value = false
			queue = append(queue, n)
		}
	}
	r.pending = nil
	for i := 0; i < len(queue); i++ {
		for _, parent := range queue[i].parents {
			if parent.value && !parent.holds() {
				parent.value = false
				queue = append(queue, parent)
			}
		}
	}
	return root.value
}

func (n *relationNode) holds() bool {
	if !n.valid {
		return false
	}
	if n.any {
		for _, child := range n.dependencies {
			if child.value {
				return true
			}
		}
		return false
	}
	for _, child := range n.dependencies {
		if !child.value {
			return false
		}
	}
	return true
}

func (r *schemaRelation) build(s, t WireSchema) *relationNode {
	key := pairKey(s, t)
	if n, ok := r.nodes[key]; ok {
		return n
	}
	n := &relationNode{valid: true, value: true}
	r.nodes[key] = n
	r.pending = append(r.pending, n)
	dep := func(a, b WireSchema) {
		child := r.build(a, b)
		n.dependencies = append(n.dependencies, child)
		child.parents = append(child.parents, n)
	}
	s, _ = resolve(r.source, s)
	t, _ = resolve(r.target, t)
	if r.mode == "equal" {
		if s.Kind != t.Kind || len(s.Properties) != len(t.Properties) || len(s.Variants) != len(t.Variants) || (s.Element == nil) != (t.Element == nil) {
			n.valid = false
			return n
		}
		for _, name := range keys(s.Properties) {
			a := s.Properties[name]
			b, ok := t.Properties[name]
			if !ok || a.Required != b.Required {
				n.valid = false
				return n
			}
			dep(a.Schema, b.Schema)
		}
		if s.Element != nil {
			dep(*s.Element, *t.Element)
		}
		for i, a := range s.Variants {
			dep(a, t.Variants[i])
		}
		s.Properties = nil
		t.Properties = nil
		s.Element = nil
		t.Element = nil
		s.Variants = nil
		t.Variants = nil
		n.valid = sameNode(s, t)
		return n
	}
	if t.Kind == "json" {
		return n
	}
	if s.Kind == "json" {
		n.valid = false
		return n
	}
	if s.Nullable && s.Kind != "null" {
		s.Nullable = false
		dep(s, t)
		dep(WireSchema{Kind: "null"}, t)
		return n
	}
	if s.Kind == "union" {
		for _, v := range s.Variants {
			dep(v, t)
		}
		return n
	}
	if t.Kind == "union" {
		if s.Kind == "null" && t.Nullable {
			return n
		}
		n.any = true
		for _, v := range t.Variants {
			dep(s, v)
		}
		return n
	}
	if s.Kind == "null" {
		n.valid = t.Kind == "null" || t.Nullable
		return n
	}
	if s.Kind == "integer" && t.Kind == "number" {
		return n
	}
	if s.Kind == "enum" && t.Kind == "string" && t.Encoding == "" {
		return n
	}
	if s.Kind != t.Kind {
		n.valid = false
		return n
	}
	switch s.Kind {
	case "string":
		n.valid = s.Encoding == t.Encoding
	case "enum":
		set := map[string]bool{}
		for _, v := range t.Values {
			set[v] = true
		}
		for _, v := range s.Values {
			if !set[v] {
				n.valid = false
				break
			}
		}
	case "integer":
		n.valid = *s.Minimum >= *t.Minimum && *s.Maximum <= *t.Maximum
	case "array", "map":
		if t.Length != nil && (s.Length == nil || *s.Length != *t.Length) {
			n.valid = false
			return n
		}
		dep(*s.Element, *t.Element)
	case "object":
		// Extra response fields are allowed except on the generated closed empty
		// object. Removing any declared field remains conservative in both directions.
		if len(t.Properties) == 0 && len(s.Properties) != 0 {
			n.valid = false
			return n
		}
		if r.mode == "request" {
			for name := range s.Properties {
				if _, ok := t.Properties[name]; !ok {
					n.valid = false
					return n
				}
			}
		}
		for _, name := range keys(t.Properties) {
			tp := t.Properties[name]
			sp, ok := s.Properties[name]
			if !ok {
				if tp.Required || r.mode == "response" {
					n.valid = false
					return n
				}
				continue
			}
			if tp.Required && !sp.Required {
				n.valid = false
				return n
			}
			dep(sp.Schema, tp.Schema)
		}
	}
	return n
}
