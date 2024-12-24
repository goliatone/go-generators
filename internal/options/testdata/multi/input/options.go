package multi

import (
	"io"
	"net/http"
	"time"
)

type Option func(*Config)

func WithWriter(w io.Writer) Option {
	return func(c *Config) {
		c.writer = w
	}
}

func WithClient(client *http.Client) Option {
	return func(c *Config) {
		c.client = client
	}
}

func WithDeadline(t time.Time) Option {
	return func(c *Config) {
		c.deadline = t
	}
}
