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

### App Config

The `app-config` generator reads a given JSON file and automatically generates Go struct definitions that mirror the JSON configuration. This is especially useful when you want to create configuration types directly from a JSON file, reducing boilerplate and ensuring that your configuration structs stay in sync with your JSON.

By integrating `app-config` via a `//go:generate` directive, you can automatically generate configuration structs with appropriate `koanf` tags to support libraries like [koanf](https://github.com/knadh/koanf).

1. **Add a generate comment to your configuration file (or any Go file):**

```go
//go:generate app-config -input ./config.json -output ./config_structs.go -package config
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

Example `config.go` file:

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
