//go:generate go run ../../cmd/options-setters/main.go -input ./options.go -output ./options_setters.go -package cron
package cron

import (
	"io"
	"time"
)

type LogLevel int

type Logger any

// Parser represents a cron expression parser type
type Parser int

type Scheduler struct {
	location     *time.Location
	logger       Logger
	logWriter    io.Writer
	logLevel     LogLevel
	errorHandler func(error)
	parser       Parser
}

// Option defines the functional option type for CronScheduler
type Option func(*Scheduler)

// WithLocation sets the timezone location for the scheduler
func WithLocation(loc *time.Location) Option {
	return func(cs *Scheduler) {
		cs.location = loc
	}
}

// WithLogger sets a custom logger for the scheduler
func WithLogger(logger Logger) Option {
	return func(cs *Scheduler) {
		cs.logger = logger
	}
}

// WithLogWriter sets a custom writer for logging
func WithLogWriter(writer io.Writer) Option {
	return func(cs *Scheduler) {
		cs.logWriter = writer
	}
}

// WithLogLevel sets the logging level
func WithLogLevel(level LogLevel) Option {
	return func(cs *Scheduler) {
		cs.logLevel = level
	}
}

// WithErrorHandler sets a custom error handler for the scheduler
func WithErrorHandler(handler func(error)) Option {
	return func(cs *Scheduler) {
		cs.errorHandler = handler
	}
}

// WithParser sets the type of cron expression parser to use
func WithParser(p Parser) Option {
	return func(cs *Scheduler) {
		cs.parser = p
	}
}
