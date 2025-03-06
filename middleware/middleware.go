package middleware

import (
	"log"
	"net/http"
	"runtime/debug"
	"time"
)

// Logger logs information about each HTTP request
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create a custom response writer to capture the status code
		crw := &customResponseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK, // Default status code
		}

		next.ServeHTTP(crw, r)

		log.Printf(
			"%s %s %s %d %s",
			r.Method,
			r.RequestURI,
			r.RemoteAddr,
			crw.statusCode,
			time.Since(start),
		)
	})
}

// Recover middleware recovers from panics and logs the error
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic: %v\n%s", err, debug.Stack())
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(w, r)
	})
}

// customResponseWriter is a custom response writer that captures the status code
type customResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader captures the status code and calls the underlying WriteHeader
func (crw *customResponseWriter) WriteHeader(code int) {
	crw.statusCode = code
	crw.ResponseWriter.WriteHeader(code)
}
