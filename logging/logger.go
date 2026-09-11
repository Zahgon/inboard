// Package logging provides structured logging with logrus.
package logging

import (
	"fmt"
	"log"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// ContextKeyLogEntry is the echo context key holding the request scoped log entry.
const ContextKeyLogEntry = "log_entry"

// Logger is a configured logrus.Logger.
var Logger *logrus.Logger

// StructuredLogger is a structured logrus Logger.
type StructuredLogger struct {
	Logger *logrus.Logger
}

// NewLogger creates and configures a new logrus Logger.
func NewLogger() *logrus.Logger {
	Logger = logrus.New()
	if viper.GetBool("log_textlogging") {
		Logger.Formatter = &logrus.TextFormatter{
			DisableTimestamp: true,
		}
	} else {
		Logger.Formatter = &logrus.JSONFormatter{
			DisableTimestamp: true,
		}
	}

	level := viper.GetString("log_level")
	if level == "" {
		level = "error"
	}
	l, err := logrus.ParseLevel(level)
	if err != nil {
		log.Fatal(err)
	}
	Logger.Level = l
	return Logger
}

// NewStructuredLogger implements a custom structured logrus Logger as echo middleware.
func NewStructuredLogger(logger *logrus.Logger) echo.MiddlewareFunc {
	l := &StructuredLogger{logger}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			entry := l.NewLogEntry(c)
			c.Set(ContextKeyLogEntry, entry)

			start := time.Now()
			err := next(c)
			if err != nil {
				// let the central error handler write the response before logging it
				c.Error(err)
			}

			res := c.Response()
			entry.Write(res.Status, int(res.Size), time.Since(start))
			return nil
		}
	}
}

// NewLogEntry sets default request log fields.
func (l *StructuredLogger) NewLogEntry(c echo.Context) *StructuredLoggerEntry {
	r := c.Request()
	entry := &StructuredLoggerEntry{Logger: logrus.NewEntry(l.Logger)}
	logFields := logrus.Fields{}

	logFields["ts"] = time.Now().UTC().Format(time.RFC1123)

	if reqID := c.Response().Header().Get(echo.HeaderXRequestID); reqID != "" {
		logFields["req_id"] = reqID
	}

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}

	logFields["http_scheme"] = scheme
	logFields["http_proto"] = r.Proto
	logFields["http_method"] = r.Method

	logFields["remote_addr"] = r.RemoteAddr
	logFields["user_agent"] = r.UserAgent()

	logFields["uri"] = r.RequestURI

	entry.Logger = entry.Logger.WithFields(logFields)

	entry.Logger.Infoln("request started")

	return entry
}

// StructuredLoggerEntry is a logrus.FieldLogger.
type StructuredLoggerEntry struct {
	Logger logrus.FieldLogger
}

// Write logs the completed request.
func (l *StructuredLoggerEntry) Write(status, bytes int, elapsed time.Duration) {
	l.Logger = l.Logger.WithFields(logrus.Fields{
		"resp_status":       status,
		"resp_bytes_length": bytes,
		"resp_elapsed_ms":   float64(elapsed.Nanoseconds()) / 1000000.0,
	})

	l.Logger.Infoln("request complete")
}

// Panic prints stack trace
func (l *StructuredLoggerEntry) Panic(v any, stack []byte) {
	l.Logger = l.Logger.WithFields(logrus.Fields{
		"stack": string(stack),
		"panic": fmt.Sprintf("%+v", v),
	})
}

// Helper methods used by the application to get the request-scoped
// logger entry and set additional fields between handlers.

// GetLogEntry returns the request scoped logrus.FieldLogger.
func GetLogEntry(c echo.Context) logrus.FieldLogger {
	entry, ok := c.Get(ContextKeyLogEntry).(*StructuredLoggerEntry)
	if !ok {
		return logrus.NewEntry(Logger)
	}
	return entry.Logger
}

// LogEntrySetField adds a field to the request scoped logrus.FieldLogger.
func LogEntrySetField(c echo.Context, key string, value any) {
	if entry, ok := c.Get(ContextKeyLogEntry).(*StructuredLoggerEntry); ok {
		entry.Logger = entry.Logger.WithField(key, value)
	}
}

// LogEntrySetFields adds multiple fields to the request scoped logrus.FieldLogger.
func LogEntrySetFields(c echo.Context, fields map[string]any) {
	if entry, ok := c.Get(ContextKeyLogEntry).(*StructuredLoggerEntry); ok {
		entry.Logger = entry.Logger.WithFields(fields)
	}
}
