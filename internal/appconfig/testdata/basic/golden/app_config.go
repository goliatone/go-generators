// Code generated by app-config; DO NOT EDIT.

package appconfig

type Config struct {
	Database Database `json:"database" koanf:"database"`
}

type Database struct {
	Debug      bool   `json:"debug" koanf:"debug"`
	DefaultTTL string `json:"default_ttl" koanf:"default_ttl"`
	Driver     string `json:"driver" koanf:"driver"`
	Dsn        string `json:"dsn" koanf:"dsn"`
	Server     string `json:"server" koanf:"server"`
}
