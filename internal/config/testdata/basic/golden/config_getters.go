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
