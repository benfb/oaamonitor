package middleware

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLogger(t *testing.T) {
	tests := []struct {
		name           string
		statusCode     int
		expectedMethod string
		expectedURI    string
	}{
		{
			name:           "200 OK response",
			statusCode:     http.StatusOK,
			expectedMethod: "GET",
			expectedURI:    "/test",
		},
		{
			name:           "404 Not Found response",
			statusCode:     http.StatusNotFound,
			expectedMethod: "GET",
			expectedURI:    "/missing",
		},
		{
			name:           "500 Internal Server Error",
			statusCode:     http.StatusInternalServerError,
			expectedMethod: "POST",
			expectedURI:    "/error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture log output
			var logBuf bytes.Buffer
			log.SetOutput(&logBuf)
			defer log.SetOutput(nil)

			// Create a test handler that returns the specified status code
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			})

			// Wrap with Logger middleware
			handler := Logger(testHandler)

			// Create test request
			req := httptest.NewRequest(tt.expectedMethod, tt.expectedURI, nil)
			req.RemoteAddr = "127.0.0.1:12345"
			rec := httptest.NewRecorder()

			// Execute request
			handler.ServeHTTP(rec, req)

			// Verify status code
			if rec.Code != tt.statusCode {
				t.Errorf("Expected status code %d, got %d", tt.statusCode, rec.Code)
			}

			// Verify log contains expected information
			logOutput := logBuf.String()

			if !strings.Contains(logOutput, tt.expectedMethod) {
				t.Errorf("Log missing method. Got: %s", logOutput)
			}

			if !strings.Contains(logOutput, tt.expectedURI) {
				t.Errorf("Log missing URI. Got: %s", logOutput)
			}

			if !strings.Contains(logOutput, "127.0.0.1:12345") {
				t.Errorf("Log missing remote address. Got: %s", logOutput)
			}

			// Verify log contains status code
			statusStr := string(rune('0' + (tt.statusCode / 100)))
			if !strings.Contains(logOutput, statusStr) {
				t.Errorf("Log missing status code indicator. Got: %s", logOutput)
			}
		})
	}
}

func TestLoggerCapturesStatusCode(t *testing.T) {
	var logBuf bytes.Buffer
	log.SetOutput(&logBuf)
	defer log.SetOutput(nil)

	// Test handler that explicitly writes a status code
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated) // 201
		w.Write([]byte("created"))
	})

	handler := Logger(testHandler)
	req := httptest.NewRequest("POST", "/create", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "201") {
		t.Errorf("Expected log to contain status code 201, got: %s", logOutput)
	}
}

func TestRecover(t *testing.T) {
	// Capture log output
	var logBuf bytes.Buffer
	log.SetOutput(&logBuf)
	defer log.SetOutput(nil)

	// Create a handler that panics
	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	// Wrap with Recover middleware
	handler := Recover(panicHandler)

	// Create test request
	req := httptest.NewRequest("GET", "/panic", nil)
	rec := httptest.NewRecorder()

	// Execute request - should not panic
	handler.ServeHTTP(rec, req)

	// Verify response is 500 Internal Server Error
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Expected status code %d, got %d", http.StatusInternalServerError, rec.Code)
	}

	// Verify error message in response
	body := rec.Body.String()
	if !strings.Contains(body, "Internal Server Error") {
		t.Errorf("Expected error message in body, got: %s", body)
	}

	// Verify log contains panic information
	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "panic: test panic") {
		t.Errorf("Log missing panic message. Got: %s", logOutput)
	}

	// Verify log contains stack trace
	if !strings.Contains(logOutput, "middleware") {
		t.Errorf("Log missing stack trace. Got: %s", logOutput)
	}
}

func TestRecoverNoPanic(t *testing.T) {
	// Create a handler that does not panic
	normalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	// Wrap with Recover middleware
	handler := Recover(normalHandler)

	// Create test request
	req := httptest.NewRequest("GET", "/normal", nil)
	rec := httptest.NewRecorder()

	// Execute request
	handler.ServeHTTP(rec, req)

	// Verify normal operation
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, rec.Code)
	}

	body := rec.Body.String()
	if body != "success" {
		t.Errorf("Expected body 'success', got: %s", body)
	}
}

func TestCustomResponseWriter(t *testing.T) {
	rec := httptest.NewRecorder()
	crw := &customResponseWriter{
		ResponseWriter: rec,
		statusCode:     http.StatusOK,
	}

	// Test default status code
	if crw.statusCode != http.StatusOK {
		t.Errorf("Expected default status code %d, got %d", http.StatusOK, crw.statusCode)
	}

	// Test WriteHeader captures status code
	crw.WriteHeader(http.StatusNotFound)
	if crw.statusCode != http.StatusNotFound {
		t.Errorf("Expected status code %d, got %d", http.StatusNotFound, crw.statusCode)
	}

	// Verify underlying ResponseWriter received the status
	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected underlying recorder status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestMiddlewareChaining(t *testing.T) {
	var logBuf bytes.Buffer
	log.SetOutput(&logBuf)
	defer log.SetOutput(nil)

	// Test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// Chain both middleware
	handler := Logger(Recover(testHandler))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Verify request succeeded
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, rec.Code)
	}

	// Verify logging happened
	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "GET") {
		t.Errorf("Expected log output to contain method. Got: %s", logOutput)
	}
}
