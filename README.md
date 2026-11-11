# Go Generators

A collection of code generators for Go projects.

## Installation

Install all generators:

```bash
go install github.com/goliatone/go-generators/cmd/...@latest
```

Or install specific generators:

```bash
go install github.com/goliatone/go-generators/cmd/options-setters@latest
go install github.com/goliatone/go-generators/cmd/config-getters@latest
go install github.com/goliatone/go-generators/cmd/app-config@latest
```

## Usage

1. Install the generators you need
2. Add appropriate `//go:generate` comments to your code
3. Run `go generate ./...` in your project

## Available Generators


### RPC contracts to TypeScript

`rpccontract.Generate(manifestJSON)` provides a reusable, side-effect-free emitter. The `rpc-contract-ts` CLI reads the same versioned Go-owned JSON manifest. It emits concrete JSON types, method and command maps, and transport-agnostic `createEndpoints` / `createDispatcher` helpers. Callbacks preserve `{ data?: T }`, reject RPC errors through the existing host client, and leave operational failure data intact. Dispatch preserves Go field casing and forwards IDs separately from payloads. No server, route discovery or Go application dependency is required by the emitter.

From this source checkout, with Go dependencies already cached:

```sh
GOWORK=off GOPROXY=off GOSUMDB=off go run ./cmd/rpc-contract-ts \
  -manifest <manifest.json> -out <contracts.ts>
GOWORK=off GOPROXY=off GOSUMDB=off go run ./cmd/rpc-contract-ts \
  -manifest <manifest.json> -out <contracts.ts> --check
```

`--check` never writes inputs or outputs, including when output is missing. Drift returns a nonzero exit status; it does not classify compatibility. Generation rejects unsupported schema versions, unresolved references, ambiguous JSON declarations and fingerprint mismatches. Explicit opaque JSON is emitted as `JsonValue`; missing type information is an error. TypeScript's `number` cannot enforce numeric ranges: typed Go adapters use the producer's `DecodeContractJSON[T]` on original input and `ValidateContractValue` before output serialization. Canonical serialized-JSON checking alone does not reproduce Go's typed decoding semantics.

### Structural RPC comparison

`rpccontract.Compare(baselineJSON, candidateJSON)` validates both complete
manifests and returns a deterministic `rpc-contract-comparison/v1` report. Compare
an old client contract with a new server contract:

```sh
go run ./cmd/rpc-contract-ts -baseline before.json -manifest after.json -fail-on-breaking
```

Comparison prints JSON to stdout and never writes files. `-out` and `--check`
are rejected in comparison mode, even when explicitly set to empty/false. The
existing generation `--check` remains a byte-drift check. Exit 0 means comparison
completed; with `-fail-on-breaking`, a potentially breaking report is still printed
and the process exits 1. Invalid manifests/flags and output errors also exit 1,
with diagnostics on stderr. Do not redirect stdout over either input file: shell
redirection occurs before the CLI can read or validate it.

Reports contain both fingerprints, a classification and sorted changes with
JSON-pointer paths, direction, code and explanation. Paths through references
identify the resolved schema at the usage site. Classifications are `unchanged`,
`compatible`, `additive` and `potentially_breaking`; the last dominates, then
additive, then compatible. Fingerprint-only/representation changes remain visible.

| Change | Request/input | Response/result |
| --- | --- | --- |
| Widen enum, numeric range or allowed nulls | Compatible | Potentially breaking |
| Narrow those values | Potentially breaking | Compatible |
| Optional property becomes required | Potentially breaking | Compatible |
| Add optional property | Additive | Additive, except an empty closed object |
| Remove declared property, even optional | Potentially breaking | Potentially breaking |
| Change string encoding or receipt/result mode | Review required | Review required |

New operations/definitions are additive; removal is potentially breaking.
Shared/recursive types are checked at each known usage direction. Union inclusion
is conservative: each source branch must fit a target branch. Unproven inclusion,
changed definitions without known wire usage and unknown canonical directions
require review. Removed/renamed exported definitions are flagged; this is not a
source-compatibility guarantee for arbitrary direct generated-alias consumers.
Presence reflects serialization, not validation. Opaque JSON and a compatible
structural result establish no authorization, business semantics or retry safety.

Go requirements come from [go.mod](go.mod). The coordinated consumer support
boundary and release matrix are maintained in the
[admin RPC guide](https://github.com/goliatone/go-admin/blob/main/docs/GUIDE_RPC.md#compatibility-and-support-boundary).
The comparison API is a source candidate until its owning release is published.

After R2-G is published, replace the release placeholder with its verified tag and commit that exact tool pin in the consuming application's generation recipe. Never use a moving tag for reproducible generation:

```go
//go:generate go run ./internal/rpcexport -out contracts
//go:generate go run github.com/goliatone/go-generators/cmd/rpc-contract-ts@<published-R2-G-tag> -manifest contracts/manifest.json -out contracts/contracts.ts
```

The first command is the application's explicit offline Go export program. Pre-create its output directory. Pre-cache the pinned tool and Go dependencies, then run `GOWORK=off GOPROXY=off GOSUMDB=off go generate ./...`. Normal builds and tests do not generate files. Run the same pinned CLI with `--check` in CI. Publication and later go-command CLI adoption remain separate release steps.

The pre-release integration harness verifies the recorded Go source archive, runs its exporter twice in a disposable directory, emits TypeScript, executes the producer review regressions, compiles fresh Go JSON literals with strict positive/negative consumers, and tests real-wire callbacks:

```sh
python3 rpccontract/interop/verify.py --tsc <installed-typescript-bin/tsc>
```

This needs cached Go dependencies, Python, Node and an installed TypeScript compiler; it does not download tools. The checked fixture and golden files live under `rpccontract/testdata`; the frozen producer snapshot and its digest are under `.ctx/specs/rpc-contract-typescript/support/evidence`. This source proof is distinct from future published-consumer and live HTTP checks.


### App Config

The `app-config` generator reads a given JSON file and automatically generates Go struct definitions that mirror the JSON configuration. This is especially useful when you want to create configuration types directly from a JSON file, reducing boilerplate and ensuring that your configuration structs stay in sync with your JSON.

By integrating `app-config` via a `//go:generate` directive, you can automatically generate configuration structs with appropriate `koanf` tags to support libraries like [koanf](https://github.com/knadh/koanf).

1. **Add a generate comment to your configuration file (or any Go file):**

```go
//go:generate app-config -input ./app.json -output ./config_structs.go -pkg config
package config
```

2. Run go generate:

```bash
go generate ./...
```

Or run the generator directly:

```bash
app-config -input ./config/app.json -output ./config/config_structs.go -package config
```

#### Options

- `-input`: Input JSON file that defines your configuration (default: config.json)
- `-output`: Output file for generated Go structs (default: config_structs.go)
- `-pkg`: Package name for the generated code (default: main)
- `-extension`:

#### Example

Given a JSON file config.json with the following content:

```json
{
    "database": {
        "dsn": "file:test.db?cache=shared",
        "debug": true,
        "driver": "sqlite",
        "server": "file"
    },
    "auth": {
        "enabled": true,
        "users": [
            {
                "username": "admin",
                "password": "pwd"
            }
        ]
    },
    "views": {
        "dir": "./views",
        "extension": ".html"
    }
}
```

The generated code (config_structs.go) will be:

```go
// Code generated by app-config; DO NOT EDIT.
package config

type Config struct {
	Database Database `koanf:"database"`
	Auth     Auth     `koanf:"auth"`
	Views    Views    `koanf:"views"`
}

type Database struct {
	Dsn    string `koanf:"dsn"`
	Debug  bool   `koanf:"debug"`
	Driver string `koanf:"driver"`
	Server string `koanf:"server"`
}

type Auth struct {
	Enabled bool   `koanf:"enabled"`
	Users   []User `koanf:"users"`
}

type User struct {
	Username string `koanf:"username"`
	Password string `koanf:"password"`
}

type Views struct {
	Dir       string `koanf:"dir"`
	Extension string `koanf:"extension"`
}
```

#### Example Overrides

Given this app.json:

```json
{
  "database": {
    "dsn": "file:test.db?cache=shared"
  }
}
```
And this overrides.yml:

```yaml
database:
  - name: Dsn
    override: DSN
  - name: assets_fs
    override: AssetsFS
    type: fs.FS
```

Add the `-extension` flag with the path to the `overrides.yml` file:
```
//go:generate app-config -input ./app.json -output ./config_structs.go -pkg config --struct BaseConfig -extension overrides.yml
```

The generated file is:
```go
// Code generated by app-config; DO NOT EDIT.

package appconfig

type Config struct {
	Database Database `koanf:"database"`
}

type Database struct {
	AssetsFS fs.FS  `koanf:"assets_fs"`
	DSN      string `koanf:"dsn"`
}
```
- `AssetFS` was created but not present in app.json
- `DSN`  instead of `Dsn` that is what it would generate otherwise

### Config Getters

The `config-getters` generator scans a given Go file for struct definitions and automatically generates getter methods for each field. This is particularly useful for configuration structs where you want to provide a clean, consistent interface for accessing configuration values.

By integrating config-getters via a `//go:generate` directive, you can automatically maintain getter methods for your configuration structs without writing boilerplate code.

1. Add a generate comment to your config file:

```go
//go:generate config-getters -input ./config.go
```

2. Run go generate:

```bash
go generate ./...
```

Or run the generator directly:

```bash
config-getters -input ./config.go
```

#### Options

- `-input`: Input file containing the struct definitions (default: config.go)
- `-output`: Output file for generated code (default: {input}_getters.go)

#### Example

Given the following `config.go` file:

```go
package config

type Config struct {
    Logger    Logger
    Database  Database
}

type Logger struct {
    Level    string
    Filename string
}

type Database struct {
    DNS   string
    Debug bool
}
```

Generated code:

```go
// Code generated by config-getters; DO NOT EDIT.

package config

// Config Getters
func (c Config) GetLogger() Logger {
    return c.Logger
}

func (c Config) GetDatabase() Database {
    return c.Database
}

// Logger Getters
func (l Logger) GetLevel() string {
    return l.Level
}

func (l Logger) GetFilename() string {
    return l.Filename
}

// Database Getters
func (d Database) GetDNS() string {
    return d.DNS
}

func (d Database) GetDebug() bool {
    return d.Debug
}
```

### Option Setters

The `optoins-setters` is a generator that will scan a given Go file for existing functional options (like `func WithTimeout(...) Option`) and automatically creates setter interfaces and methods in a new file.

This lets you pass configuration data from a struct that implements the generated getter interfaces into your code’s functional options.

By integrating options-setters via a `//go:generate` directive, you can keep your code base clean, consistent, and easily configurable across different packages or modules.

1. Add a generate comment to your options file:

```go
//go:generate options-setters -input ./options.go
```

2. Run go generate:

```bash
go generate ./...
```

Or run the generator directly:

```bash
options-setters -input ./options.go
```

#### Options

- `-input`: Input file containing the options (default: options.go)
- `-output`: Output file for generated code (default: {input}_setters.go)

#### Example

Given the following `options.go` file:

```go
package myapp

import (
	"context"
	"time"
)

type Option func(*Config)

func WithTimeout(d time.Duration) Option {
	return func(c *Config) {
		c.timeout = d
	}
}

func WithContext(ctx context.Context) Option {
	return func(c *Config) {
		c.ctx = ctx
	}
}

func WithHandler(h func(error) bool) Option {
	return func(c *Config) {
		c.handler = h
	}
}
```

Generated code:

```go
// Code generated by options-setters; DO NOT EDIT.

package myapp

type TimeoutGetter interface {
	GetTimeout() time.Duration
}

func WithTimeoutSetter(s TimeoutGetter) Option {
	return func(cs *Config) {
		if s != nil {
			cs.timeout = s.GetTimeout()
		}
	}
}

type ContextGetter interface {
	GetContext() context.Context
}

func WithContextSetter(s ContextGetter) Option {
	return func(cs *Config) {
		if s != nil {
			cs.ctx = s.GetContext()
		}
	}
}

type HandlerGetter interface {
	GetHandler() func(error) bool
}

func WithHandlerSetter(s HandlerGetter) Option {
	return func(cs *Config) {
		if s != nil {
			cs.handler = s.GetHandler()
		}
	}
}

// WithConfigurator sets multiple options from
// a single configuration struct that implements
// one or more Getter interfaces
func WithConfigurator(i interface{}) Option {
	return func(cs *Config) {

		if s, ok := i.(TimeoutGetter); ok {
			cs.timeout = s.GetTimeout()
		}

		if s, ok := i.(ContextGetter); ok {
			cs.ctx = s.GetContext()
		}

		if s, ok := i.(HandlerGetter); ok {
			cs.handler = s.GetHandler()
		}

	}
}
```
