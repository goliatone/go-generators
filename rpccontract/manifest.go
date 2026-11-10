// Package rpccontract generates TypeScript from producer-owned offline JSON contracts.
package rpccontract

const SchemaVersion = "rpc-contract/v1"

type WireSchema struct {
	Kind       string                  `json:"kind"`
	Ref        string                  `json:"ref,omitempty"`
	Nullable   bool                    `json:"nullable,omitempty"`
	Properties map[string]WireProperty `json:"properties,omitempty"`
	Element    *WireSchema             `json:"element,omitempty"`
	Values     []string                `json:"values,omitempty"`
	Variants   []WireSchema            `json:"variants,omitempty"`
	Encoding   string                  `json:"encoding,omitempty"`
	Minimum    *int64                  `json:"minimum,omitempty"`
	Maximum    *int64                  `json:"maximum,omitempty"`
	Length     *int                    `json:"length,omitempty"`
}
type WireProperty struct {
	Schema   WireSchema `json:"schema"`
	Required bool       `json:"required"`
}
type EndpointContract struct {
	Request  WireSchema `json:"request"`
	Response WireSchema `json:"response"`
}
type NamedCommandContract struct {
	Input       WireSchema  `json:"input"`
	Result      *WireSchema `json:"result,omitempty"`
	ResultMode  string      `json:"resultMode"`
	Summary     string      `json:"summary,omitempty"`
	Description string      `json:"description,omitempty"`
}
type ContractManifest struct {
	SchemaVersion string                          `json:"schemaVersion"`
	Definitions   map[string]WireSchema           `json:"definitions"`
	Endpoints     map[string]EndpointContract     `json:"endpoints"`
	Commands      map[string]NamedCommandContract `json:"commands"`
	Canonical     map[string]WireSchema           `json:"canonical"`
	Fingerprint   string                          `json:"fingerprint,omitempty"`
}
