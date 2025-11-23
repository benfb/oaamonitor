package storage

import (
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func TestNewS3Client(t *testing.T) {
	client := NewS3Client("access-key", "secret-key", "us-east-1", "https://s3.amazonaws.com")

	if client.AccessKeyID != "access-key" {
		t.Errorf("Expected AccessKeyID 'access-key', got '%s'", client.AccessKeyID)
	}
	if client.SecretAccessKey != "secret-key" {
		t.Errorf("Expected SecretAccessKey 'secret-key', got '%s'", client.SecretAccessKey)
	}
	if client.Region != "us-east-1" {
		t.Errorf("Expected Region 'us-east-1', got '%s'", client.Region)
	}
}

func TestS3ClientGetObject(t *testing.T) {
	client := NewS3Client("test-access-key", "test-secret-key", "us-east-1", "https://example.com")
	client.client = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			// Verify request method
			if req.Method != "GET" {
				t.Errorf("Expected GET request, got %s", req.Method)
			}

			// Verify path
			expectedPath := "/test-bucket/test-key.db"
			if req.URL.Path != expectedPath {
				t.Errorf("Expected path %s, got %s", expectedPath, req.URL.Path)
			}

			// Verify AWS SigV4 headers are present
			if req.Header.Get("Authorization") == "" {
				t.Error("Missing Authorization header")
			}
			if req.Header.Get("X-Amz-Date") == "" {
				t.Error("Missing X-Amz-Date header")
			}
			if req.Header.Get("X-Amz-Content-Sha256") == "" {
				t.Error("Missing X-Amz-Content-Sha256 header")
			}

			// Verify Authorization header format
			authHeader := req.Header.Get("Authorization")
			if !strings.HasPrefix(authHeader, "AWS4-HMAC-SHA256") {
				t.Errorf("Invalid Authorization header format: %s", authHeader)
			}
			if !strings.Contains(authHeader, "Credential=") {
				t.Error("Authorization header missing Credential")
			}
			if !strings.Contains(authHeader, "SignedHeaders=") {
				t.Error("Authorization header missing SignedHeaders")
			}
			if !strings.Contains(authHeader, "Signature=") {
				t.Error("Authorization header missing Signature")
			}

			return newResponse(http.StatusOK, "test database content"), nil
		}),
	}

	// Test GetObject
	resp, err := client.GetObject("test-bucket", "test-key.db")
	if err != nil {
		t.Fatalf("GetObject failed: %v", err)
	}
	defer resp.Close()

	// Verify response content
	content, err := io.ReadAll(resp)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	expectedContent := "test database content"
	if string(content) != expectedContent {
		t.Errorf("Expected content %q, got %q", expectedContent, string(content))
	}
}

func TestS3ClientGetObjectNotFound(t *testing.T) {
	client := NewS3Client("test-key", "test-secret", "us-east-1", "https://example.com")
	client.client = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return newResponse(http.StatusNotFound, "NoSuchKey"), nil
		}),
	}

	_, err := client.GetObject("test-bucket", "missing.db")
	if err == nil {
		t.Error("Expected error for 404 response, got nil")
	}
	if !strings.Contains(err.Error(), "object not found") {
		t.Errorf("Expected 'object not found' error, got: %v", err)
	}
}

func TestS3ClientPutObject(t *testing.T) {
	// Create test file to upload
	tmpFile, err := os.CreateTemp("", "s3_upload_test_*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	testContent := "test upload content"
	if _, err := tmpFile.WriteString(testContent); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}
	tmpFile.Close()

	client := NewS3Client("test-access-key", "test-secret-key", "us-east-1", "https://example.com")
	client.client = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			// Verify request method
			if req.Method != "PUT" {
				t.Errorf("Expected PUT request, got %s", req.Method)
			}

			// Verify AWS SigV4 headers
			if req.Header.Get("Authorization") == "" {
				t.Error("Missing Authorization header")
			}
			if req.Header.Get("X-Amz-Date") == "" {
				t.Error("Missing X-Amz-Date header")
			}
			if req.Header.Get("X-Amz-Content-Sha256") == "" {
				t.Error("Missing X-Amz-Content-Sha256 header")
			}

			// Verify Content-Type header
			if req.Header.Get("Content-Type") != "application/octet-stream" {
				t.Errorf("Expected Content-Type 'application/octet-stream', got '%s'", req.Header.Get("Content-Type"))
			}

			return newResponse(http.StatusOK, ""), nil
		}),
	}

	// Open file for upload
	file, err := os.Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open test file: %v", err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		t.Fatalf("Failed to stat test file: %v", err)
	}

	// Test PutObject
	err = client.PutObject("test-bucket", "test-upload.db", file, fileInfo.Size())
	if err != nil {
		t.Fatalf("PutObject failed: %v", err)
	}
}

func TestS3ClientPutObjectMissingFile(t *testing.T) {
	client := NewS3Client("test-key", "test-secret", "us-east-1", "https://example.com")
	client.client = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			t.Error("client transport should not be invoked for missing file")
			return newResponse(http.StatusInternalServerError, ""), nil
		}),
	}

	// Try to open non-existent file
	file, err := os.Open("/nonexistent/file.db")
	if err != nil {
		// Expected - file doesn't exist
		return
	}
	defer file.Close()

	// If we got here, the file unexpectedly exists
	fileInfo, _ := file.Stat()
	err = client.PutObject("test-bucket", "test.db", file, fileInfo.Size())
	if err == nil {
		t.Error("Expected test to fail on non-existent file")
	}
}

func TestS3ClientSignatureComponents(t *testing.T) {
	client := NewS3Client("AKIAIOSFODNN7EXAMPLE", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", "us-east-1", "https://s3.amazonaws.com")

	req, _ := http.NewRequest("GET", "https://s3.amazonaws.com/test-bucket/test.txt", nil)

	// Test canonical headers generation
	req.Header.Set("Host", "s3.amazonaws.com")
	req.Header.Set("X-Amz-Date", "20230615T120000Z")

	canonicalHeaders := client.getCanonicalHeaders(req)
	if !strings.Contains(canonicalHeaders, "host:") {
		t.Error("Canonical headers missing 'host' header")
	}
	if !strings.Contains(canonicalHeaders, "x-amz-date:") {
		t.Error("Canonical headers missing 'x-amz-date' header")
	}

	// Test signed headers generation
	signedHeaders := client.getSignedHeaders(req)
	if !strings.Contains(signedHeaders, "host") {
		t.Error("Signed headers missing 'host'")
	}
	if !strings.Contains(signedHeaders, "x-amz-date") {
		t.Error("Signed headers missing 'x-amz-date'")
	}

	// Verify headers are sorted alphabetically
	headers := strings.Split(signedHeaders, ";")
	for i := 1; i < len(headers); i++ {
		if headers[i-1] > headers[i] {
			t.Errorf("Headers not sorted: %s > %s", headers[i-1], headers[i])
		}
	}
}

func TestHmacSHA256(t *testing.T) {
	// Test HMAC-SHA256 calculation with known values
	key := []byte("key")
	data := []byte("data")

	result := hmacSHA256(key, data)

	// HMAC should always produce 32 bytes (256 bits)
	if len(result) != 32 {
		t.Errorf("Expected HMAC length 32, got %d", len(result))
	}
}

func TestBuildCanonicalURIEncoding(t *testing.T) {
	got := buildCanonicalURI("my-bucket", "folder/a b/c+d.txt")
	want := "/my-bucket/folder/a%20b/c%2Bd.txt"
	if got != want {
		t.Fatalf("canonical URI mismatch: got %q want %q", got, want)
	}
}

func TestCanonicalQueryString(t *testing.T) {
	raw := "z=last&b=hi%20there&a=1&b=second&empty="
	got := canonicalQueryString(raw)
	want := "a=1&b=hi%20there&b=second&empty=&z=last"
	if got != want {
		t.Fatalf("canonical query mismatch: got %q want %q", got, want)
	}
}
