// Package logger provides structured logging utilities for the application.
package logger

import (
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
)

var log zerolog.Logger

// Init initializes the global logger with the specified configuration.
func Init(debug bool, logFile string) error {
	var writers []io.Writer

	// Console writer with pretty formatting
	consoleWriter := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339,
	}
	writers = append(writers, consoleWriter)

	// File writer if specified
	if logFile != "" {
		file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return err
		}
		writers = append(writers, file)
	}

	// Create multi-writer
	multi := zerolog.MultiLevelWriter(writers...)

	// Set log level
	level := zerolog.InfoLevel
	if debug {
		level = zerolog.DebugLevel
	}

	log = zerolog.New(multi).
		Level(level).
		With().
		Timestamp().
		Caller().
		Logger()

	return nil
}

// Debug logs a debug message.
func Debug() *zerolog.Event {
	return log.Debug()
}

// Info logs an info message.
func Info() *zerolog.Event {
	return log.Info()
}

// Warn logs a warning message.
func Warn() *zerolog.Event {
	return log.Warn()
}

// Error logs an error message.
func Error() *zerolog.Event {
	return log.Error()
}

// Fatal logs a fatal message and exits.
func Fatal() *zerolog.Event {
	return log.Fatal()
}

// WithField returns a logger with the specified field.
func WithField(key string, value interface{}) zerolog.Logger {
	return log.With().Interface(key, value).Logger()
}
