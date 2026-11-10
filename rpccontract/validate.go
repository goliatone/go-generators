package rpccontract

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"sort"
	"strings"
)

func keys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Parse validates the complete manifest and producer fingerprint. Duplicate keys
// and unknown fields fail instead of silently discarding conflicting declarations.
func Parse(data []byte) (*ContractManifest, error) {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	if err := uniqueJSON(d, "$"); err != nil {
		return nil, err
	}
	if _, err := d.Token(); err != io.EOF {
		return nil, fmt.Errorf("manifest: trailing JSON")
	}
	d = json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	var m ContractManifest
	if err := d.Decode(&m); err != nil {
		return nil, fmt.Errorf("manifest: %w", err)
	}
	if err := validate(&m); err != nil {
		return nil, err
	}
	d = json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	var raw map[string]any
	if err := d.Decode(&raw); err != nil {
		return nil, err
	}
	delete(raw, "fingerprint")
	b, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(b)
	if m.Fingerprint != "sha256:"+hex.EncodeToString(sum[:]) {
		return nil, fmt.Errorf("fingerprint: does not match canonical manifest")
	}
	return &m, nil
}
func uniqueJSON(d *json.Decoder, path string) error {
	t, err := d.Token()
	if err != nil {
		return err
	}
	delim, ok := t.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for d.More() {
			k, err := d.Token()
			if err != nil {
				return err
			}
			key, ok := k.(string)
			if !ok {
				return fmt.Errorf("%s: invalid key", path)
			}
			if seen[key] {
				return fmt.Errorf("%s.%s: duplicate key", path, key)
			}
			seen[key] = true
			if err := uniqueJSON(d, path+"."+key); err != nil {
				return err
			}
		}
	case '[':
		i := 0
		for d.More() {
			if err := uniqueJSON(d, fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
			i++
		}
	default:
		return fmt.Errorf("%s: unexpected delimiter", path)
	}
	_, err = d.Token()
	return err
}
func validate(m *ContractManifest) error {
	if m.SchemaVersion != SchemaVersion {
		return fmt.Errorf("schemaVersion: unsupported %q", m.SchemaVersion)
	}
	if m.Definitions == nil || m.Endpoints == nil || m.Commands == nil || m.Canonical == nil {
		return fmt.Errorf("manifest: definitions, endpoints, commands and canonical must be objects")
	}
	aliases := map[string]string{}
	for _, id := range keys(m.Definitions) {
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("definitions: empty ID")
		}
		a := typeName(id)
		if old, ok := aliases[a]; ok {
			return fmt.Errorf("definitions: alias collision %q and %q", old, id)
		}
		aliases[a] = id
		if err := node(m, m.Definitions[id], "definitions."+id, 0); err != nil {
			return err
		}
		if err := aliasCycle(m, m.Definitions[id], map[string]bool{}); err != nil {
			return fmt.Errorf("definitions.%s: %w", id, err)
		}
		if _, err := resolve(m, m.Definitions[id]); err != nil {
			return fmt.Errorf("definitions.%s: %w", id, err)
		}
	}
	for _, name := range []string{"RequestMeta", "Error", "DispatchOptions", "DispatchReceipt", "CommandDispatchRequest", "CommandDispatchResponse"} {
		s, ok := m.Canonical[name]
		if !ok {
			return fmt.Errorf("canonical.%s: missing", name)
		}
		if err := node(m, s, "canonical."+name, 0); err != nil {
			return err
		}
		n, err := resolve(m, s)
		if err != nil {
			return err
		}
		if n.Kind != "object" || n.Nullable {
			return fmt.Errorf("canonical.%s: expected nonnullable object", name)
		}
	}
	for _, id := range keys(m.Endpoints) {
		e := m.Endpoints[id]
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("endpoints: empty method")
		}
		for _, item := range []struct {
			name string
			s    WireSchema
		}{{"request", e.Request}, {"response", e.Response}} {
			if err := node(m, item.s, "endpoints."+id+"."+item.name, 0); err != nil {
				return err
			}
			n, err := resolve(m, item.s)
			if err != nil {
				return err
			}
			if n.Kind != "object" || n.Nullable {
				return fmt.Errorf("endpoints.%s.%s: expected envelope object", id, item.name)
			}
			if _, ok := n.Properties["data"]; !ok {
				return fmt.Errorf("endpoints.%s.%s.data: missing schema", id, item.name)
			}
		}
		req, _ := resolve(m, e.Request)
		res, _ := resolve(m, e.Response)
		if !req.Properties["data"].Required || res.Properties["data"].Required {
			return fmt.Errorf("endpoints.%s: request data must be required and response data optional", id)
		}
		if p, ok := req.Properties["meta"]; !ok || !sameNode(p.Schema, m.Canonical["RequestMeta"]) {
			return fmt.Errorf("endpoints.%s.request.meta: must reference canonical metadata", id)
		}
		for name, p := range req.Properties {
			if name != "data" && name != "meta" && p.Required {
				return fmt.Errorf("endpoints.%s.request.%s: unsupported required envelope field", id, name)
			}
		}
	}
	for _, id := range keys(m.Commands) {
		c := m.Commands[id]
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("commands: empty ID")
		}
		if strings.TrimSpace(id) != id {
			return fmt.Errorf("commands.%q: noncanonical ID; surrounding whitespace is not allowed", id)
		}
		if err := node(m, c.Input, "commands."+id+".input", 0); err != nil {
			return err
		}
		if c.ResultMode != "receipt" && c.ResultMode != "optional" {
			return fmt.Errorf("commands.%s.resultMode: unsupported", id)
		}
		if (c.ResultMode == "receipt") != (c.Result == nil) {
			return fmt.Errorf("commands.%s.result: inconsistent result mode", id)
		}
		if c.Result != nil {
			if err := node(m, *c.Result, "commands."+id+".result", 0); err != nil {
				return err
			}
		}
	}
	req, _ := resolve(m, m.Canonical["CommandDispatchRequest"])
	res, _ := resolve(m, m.Canonical["CommandDispatchResponse"])
	for _, name := range []string{"command_id", "name", "payload", "ids", "options"} {
		if _, ok := req.Properties[name]; !ok {
			return fmt.Errorf("canonical.CommandDispatchRequest.%s: missing", name)
		}
	}
	if p, ok := res.Properties["receipt"]; !ok || !p.Required || !sameNode(p.Schema, m.Canonical["DispatchReceipt"]) {
		return fmt.Errorf("canonical.CommandDispatchResponse.receipt: required canonical receipt expected")
	}
	if p, ok := res.Properties["result"]; !ok || p.Required {
		return fmt.Errorf("canonical.CommandDispatchResponse.result: optional result expected")
	}
	if !sameNode(req.Properties["options"].Schema, m.Canonical["DispatchOptions"]) {
		return fmt.Errorf("canonical.CommandDispatchRequest.options: canonical options expected")
	}
	return nil
}
func sameNode(a, b WireSchema) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return bytes.Equal(x, y)
}
func resolve(m *ContractManifest, s WireSchema) (WireSchema, error) {
	seen := map[string]bool{}
	nullable := s.Nullable
	for s.Kind == "ref" {
		if seen[s.Ref] {
			return s, fmt.Errorf("cyclic reference alias %q", s.Ref)
		}
		seen[s.Ref] = true
		var ok bool
		s, ok = m.Definitions[s.Ref]
		if !ok {
			return s, fmt.Errorf("unresolved reference")
		}
		nullable = nullable || s.Nullable
	}
	s.Nullable = nullable
	return s, nil
}
func node(m *ContractManifest, s WireSchema, path string, depth int) error {
	bad := func() error { return fmt.Errorf("%s: malformed or unsupported %q schema", path, s.Kind) }
	if depth > 128 {
		return fmt.Errorf("%s: schema nesting limit", path)
	}
	allowed := map[string]bool{"kind": true, "nullable": true}
	switch s.Kind {
	case "ref":
		allowed["ref"] = true
		if s.Ref == "" {
			return bad()
		}
		if _, ok := m.Definitions[s.Ref]; !ok {
			return fmt.Errorf("%s.ref: unresolved %q", path, s.Ref)
		}
	case "object":
		allowed["properties"] = true
		for _, k := range keys(s.Properties) {
			if err := node(m, s.Properties[k].Schema, path+"."+k, depth+1); err != nil {
				return err
			}
		}
	case "array", "map":
		allowed["element"] = true
		if s.Element == nil {
			return bad()
		}
		if s.Kind == "array" {
			allowed["length"] = true
		}
		if s.Length != nil && (*s.Length < 0 || *s.Length > 10000) {
			return bad()
		}
		if err := node(m, *s.Element, path+"[]", depth+1); err != nil {
			return err
		}
	case "union":
		allowed["variants"] = true
		if len(s.Variants) < 2 {
			return bad()
		}
		for i, n := range s.Variants {
			if err := node(m, n, fmt.Sprintf("%s.variant[%d]", path, i), depth+1); err != nil {
				return err
			}
		}
	case "enum":
		allowed["values"] = true
		if len(s.Values) == 0 {
			return bad()
		}
		seen := map[string]bool{}
		for _, v := range s.Values {
			if seen[v] {
				return bad()
			}
			seen[v] = true
		}
	case "integer":
		allowed["minimum"] = true
		allowed["maximum"] = true
		if s.Minimum == nil || s.Maximum == nil || *s.Minimum < -9007199254740991 || *s.Maximum > 9007199254740991 || *s.Minimum > *s.Maximum {
			return bad()
		}
	case "string":
		allowed["encoding"] = true
		switch s.Encoding {
		case "", "base64", "date-time", "json-string":
		default:
			return bad()
		}
	case "boolean", "number", "null", "json":
	default:
		return bad()
	}
	b, _ := json.Marshal(s)
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(b, &fields)
	for _, k := range keys(fields) {
		if !allowed[k] {
			return fmt.Errorf("%s.%s: invalid for %s", path, k, s.Kind)
		}
	}
	return nil
}

// Recursive object/array/map types are productive; bare aliases or union-only
// cycles cannot be represented as TypeScript aliases.
func aliasCycle(m *ContractManifest, s WireSchema, seen map[string]bool) error {
	switch s.Kind {
	case "ref":
		if seen[s.Ref] {
			return fmt.Errorf("cyclic reference alias %q", s.Ref)
		}
		next := map[string]bool{}
		maps.Copy(next, seen)
		next[s.Ref] = true
		return aliasCycle(m, m.Definitions[s.Ref], next)
	case "union":
		for _, n := range s.Variants {
			if err := aliasCycle(m, n, seen); err != nil {
				return err
			}
		}
	}
	return nil
}
