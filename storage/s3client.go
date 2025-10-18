package storage

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// S3Client is a lightweight S3-compatible client using only standard library
type S3Client struct {
	AccessKeyID     string
	SecretAccessKey string
	Region          string
	EndpointURL     string
	client          *http.Client
}

// NewS3Client creates a new S3 client
func NewS3Client(accessKeyID, secretAccessKey, region, endpointURL string) *S3Client {
	return &S3Client{
		AccessKeyID:     accessKeyID,
		SecretAccessKey: secretAccessKey,
		Region:          region,
		EndpointURL:     endpointURL,
		client:          &http.Client{Timeout: 5 * time.Minute},
	}
}

// GetObject downloads an object from S3 and returns the response body
func (c *S3Client) GetObject(bucket, key string) (io.ReadCloser, error) {
	url := fmt.Sprintf("%s/%s/%s", c.EndpointURL, bucket, key)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Sign the request
	if err := c.signRequest(req, bucket, key, ""); err != nil {
		return nil, fmt.Errorf("failed to sign request: %w", err)
	}

	// Execute request
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		resp.Body.Close()
		return nil, fmt.Errorf("object not found: %s/%s", bucket, key)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("S3 error %d: %s", resp.StatusCode, string(body))
	}

	return resp.Body, nil
}

// PutObject uploads an object to S3 from a file
func (c *S3Client) PutObject(bucket, key string, reader io.ReadSeeker, size int64) error {
	url := fmt.Sprintf("%s/%s/%s", c.EndpointURL, bucket, key)

	req, err := http.NewRequest("PUT", url, reader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.ContentLength = size
	req.Header.Set("Content-Type", "application/octet-stream")

	// Calculate payload hash
	reader.Seek(0, 0)
	payloadHash := sha256.New()
	io.Copy(payloadHash, reader)
	payloadHashStr := hex.EncodeToString(payloadHash.Sum(nil))

	// Reset reader
	reader.Seek(0, 0)

	// Sign the request
	if err := c.signRequest(req, bucket, key, payloadHashStr); err != nil {
		return fmt.Errorf("failed to sign request: %w", err)
	}

	// Execute request
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("S3 error %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// signRequest signs an HTTP request using AWS Signature Version 4
func (c *S3Client) signRequest(req *http.Request, bucket, key, payloadHash string) error {
	now := time.Now().UTC()
	dateStamp := now.Format("20060102")
	timeStamp := now.Format("20060102T150405Z")

	// If payloadHash is empty (for GET), use empty hash
	if payloadHash == "" {
		payloadHash = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" // empty hash
	}

	// Set required headers
	req.Header.Set("Host", req.URL.Host)
	req.Header.Set("X-Amz-Date", timeStamp)
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)

	// Step 1: Create canonical request
	canonicalHeaders := c.getCanonicalHeaders(req)
	signedHeaders := c.getSignedHeaders(req)

	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		req.Method,
		"/"+bucket+"/"+key,
		req.URL.RawQuery,
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	)

	// Step 2: Create string to sign
	credentialScope := fmt.Sprintf("%s/%s/s3/aws4_request", dateStamp, c.Region)
	hashedCanonicalRequest := sha256.Sum256([]byte(canonicalRequest))
	stringToSign := fmt.Sprintf("AWS4-HMAC-SHA256\n%s\n%s\n%s",
		timeStamp,
		credentialScope,
		hex.EncodeToString(hashedCanonicalRequest[:]),
	)

	// Step 3: Calculate signature
	signature := c.calculateSignature(dateStamp, stringToSign)

	// Step 4: Add authorization header
	authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		c.AccessKeyID,
		credentialScope,
		signedHeaders,
		signature,
	)
	req.Header.Set("Authorization", authHeader)

	return nil
}

// getCanonicalHeaders returns canonical headers string
func (c *S3Client) getCanonicalHeaders(req *http.Request) string {
	var headers []string
	for key := range req.Header {
		lowerKey := strings.ToLower(key)
		headers = append(headers, lowerKey)
	}
	sort.Strings(headers)

	var canonical []string
	for _, key := range headers {
		value := strings.TrimSpace(req.Header.Get(key))
		canonical = append(canonical, fmt.Sprintf("%s:%s", key, value))
	}
	return strings.Join(canonical, "\n") + "\n"
}

// getSignedHeaders returns signed headers string
func (c *S3Client) getSignedHeaders(req *http.Request) string {
	var headers []string
	for key := range req.Header {
		headers = append(headers, strings.ToLower(key))
	}
	sort.Strings(headers)
	return strings.Join(headers, ";")
}

// calculateSignature calculates the AWS SigV4 signature
func (c *S3Client) calculateSignature(dateStamp, stringToSign string) string {
	kDate := hmacSHA256([]byte("AWS4"+c.SecretAccessKey), []byte(dateStamp))
	kRegion := hmacSHA256(kDate, []byte(c.Region))
	kService := hmacSHA256(kRegion, []byte("s3"))
	kSigning := hmacSHA256(kService, []byte("aws4_request"))
	signature := hmacSHA256(kSigning, []byte(stringToSign))
	return hex.EncodeToString(signature)
}

// hmacSHA256 creates an HMAC-SHA256
func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}
