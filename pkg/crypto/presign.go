// Package crypto provides cryptographic helpers used by the platform.
package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// SignURL generates a signed URL for a specific method, resource, and expiration.
func SignURL(secretKey, baseURL, method, bucket, key string, expires time.Time) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid base URL: %w", err)
	}

	// Clean path - strip all leading slashes to match VerifyURL behavior
	cleanKey := strings.TrimLeft(key, "/")
	path := fmt.Sprintf("/storage/presigned/%s/%s", bucket, cleanKey)
	u.Path = path

	// Add expiration param
	q := u.Query()
	expStr := strconv.FormatInt(expires.Unix(), 10)
	q.Set("expires", expStr)
	q.Set("method", method)

	// Calculate signature
	// Data to sign: METHOD\nPATH\nEXPIRES
	dataToSign := fmt.Sprintf("%s\n%s\n%s", method, path, expStr)
	sig := calculateHMAC(secretKey, dataToSign)
	q.Set("signature", sig)

	u.RawQuery = q.Encode()
	return u.String(), nil
}

// VerifyURL checks if a signed URL is valid and not expired.
func VerifyURL(secretKey, method, path, expiresStr, signature string) error {
	// 1. Check expiration
	exp, err := strconv.ParseInt(expiresStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid expiration format")
	}

	if time.Now().Unix() > exp {
		return fmt.Errorf("URL expired")
	}

	// 2. Re-calculate signature
	dataToSign := fmt.Sprintf("%s\n%s\n%s", method, path, expiresStr)
	expectedSig := calculateHMAC(secretKey, dataToSign)

	// 3. Compare safely
	if !hmac.Equal([]byte(signature), []byte(expectedSig)) {
		return fmt.Errorf("invalid signature")
	}

	return nil
}

func calculateHMAC(secret, data string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}
