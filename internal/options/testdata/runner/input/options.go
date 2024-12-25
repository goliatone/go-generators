package runner

import (
	"io"
	"time"
)

type Option func(*Handler)

func WithIO(t io.Writer) Option {
	return func(r *Handler) {
		r.write = t
	}
}

func WithTimeout(t time.Duration) Option {
	return func(r *Handler) {
		r.timeout = t
	}
}
