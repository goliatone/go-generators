package complex

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
