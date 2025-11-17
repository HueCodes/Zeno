package middleware

import (
	"log"
	"net/http"
	"time"
)

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Logging is HTTP middleware that logs requests
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		wrapped := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		next.ServeHTTP(wrapped, r)

		log.Printf("%s %s %d %v", r.Method, r.URL.Path, wrapped.statusCode, time.Since(start))
	})
}

// Logger is a simple logging middleware
type Logger struct {
	prefix string
}

// NewLogger creates a new logger instance
func NewLogger(prefix string) *Logger {
	return &Logger{prefix: prefix}
}

// Info logs an info message
func (l *Logger) Info(msg string, args ...interface{}) {
	log.Printf("[%s] INFO: "+msg, append([]interface{}{l.prefix}, args...)...)
}

// Error logs an error message
func (l *Logger) Error(msg string, args ...interface{}) {
	log.Printf("[%s] ERROR: "+msg, append([]interface{}{l.prefix}, args...)...)
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, args ...interface{}) {
	log.Printf("[%s] DEBUG: "+msg, append([]interface{}{l.prefix}, args...)...)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, args ...interface{}) {
	log.Printf("[%s] WARN: "+msg, append([]interface{}{l.prefix}, args...)...)
}

// LogDuration logs the duration of an operation
func LogDuration(operation string, start time.Time) {
	duration := time.Since(start)
	log.Printf("[PERF] %s took %v", operation, duration)
}
