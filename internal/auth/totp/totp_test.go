package totp

import (
	"encoding/base64"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
)

func TestGenerateQRCode(t *testing.T) {
	tests := []struct {
		name     string
		username string
		siteUrl  string
		secret   []byte
		wantErr  bool
	}{
		{
			name:     "basic generation with nil secret",
			username: "testuser",
			siteUrl:  "opengist.io",
			secret:   nil,
			wantErr:  false,
		},
		{
			name:     "basic generation with provided secret",
			username: "testuser",
			siteUrl:  "opengist.io",
			secret:   []byte("1234567890123456"),
			wantErr:  false,
		},
		{
			name:     "username with special characters",
			username: "test.user",
			siteUrl:  "opengist.io",
			secret:   nil,
			wantErr:  false,
		},
		{
			name:     "site URL with protocol and port",
			username: "testuser",
			siteUrl:  "https://opengist.io:6157",
			secret:   nil,
			wantErr:  false,
		},
		{
			name:     "empty username",
			username: "",
			siteUrl:  "opengist.io",
			secret:   nil,
			wantErr:  true,
		},
		{
			name:     "empty site URL",
			username: "testuser",
			siteUrl:  "",
			secret:   nil,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			secretStr, qrcode, secretBytes, err := GenerateQRCode(tt.username, tt.siteUrl, tt.secret)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateQRCode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify secret string is not empty
				if secretStr == "" {
					t.Error("GenerateQRCode() returned empty secret string")
				}

				// Verify QR code image is generated
				if qrcode == "" {
					t.Error("GenerateQRCode() returned empty QR code")
				}

				// Verify QR code has correct data URI prefix
				if !strings.HasPrefix(string(qrcode), "data:image/png;base64,") {
					t.Errorf("QR code does not have correct data URI prefix: %s", qrcode[:50])
				}

				// Verify QR code is valid base64 after prefix
				base64Data := strings.TrimPrefix(string(qrcode), "data:image/png;base64,")
				_, err := base64.StdEncoding.DecodeString(base64Data)
				if err != nil {
					t.Errorf("QR code base64 data is invalid: %v", err)
				}

				// Verify secret bytes are returned
				if secretBytes == nil {
					t.Error("GenerateQRCode() returned nil secret bytes")
				}

				// Verify secret bytes have correct length
				if len(secretBytes) != secretSize {
					t.Errorf("Secret bytes length = %d, want %d", len(secretBytes), secretSize)
				}

				// If a secret was provided, verify it matches what was returned
				if tt.secret != nil && string(secretBytes) != string(tt.secret) {
					t.Error("Returned secret bytes do not match provided secret")
				}
			}
		})
	}
}

func TestGenerateQRCode_SecretUniqueness(t *testing.T) {
	username := "testuser"
	siteUrl := "opengist.io"
	iterations := 10

	secrets := make(map[string]bool)
	secretBytes := make(map[string]bool)

	for i := 0; i < iterations; i++ {
		secretStr, _, secret, err := GenerateQRCode(username, siteUrl, nil)
		if err != nil {
			t.Fatalf("Iteration %d failed: %v", i, err)
		}

		// Check secret string uniqueness
		if secrets[secretStr] {
			t.Errorf("Duplicate secret string generated at iteration %d", i)
		}
		secrets[secretStr] = true

		// Check secret bytes uniqueness
		secretKey := string(secret)
		if secretBytes[secretKey] {
			t.Errorf("Duplicate secret bytes generated at iteration %d", i)
		}
		secretBytes[secretKey] = true
	}
}

func TestGenerateQRCode_WithProvidedSecret(t *testing.T) {
	username := "testuser"
	siteUrl := "opengist.io"
	providedSecret := []byte("mysecret12345678")

	// Generate QR code multiple times with the same secret
	secretStr1, _, secret1, err := GenerateQRCode(username, siteUrl, providedSecret)
	if err != nil {
		t.Fatalf("First generation failed: %v", err)
	}

	secretStr2, _, secret2, err := GenerateQRCode(username, siteUrl, providedSecret)
	if err != nil {
		t.Fatalf("Second generation failed: %v", err)
	}

	// Secret strings should be the same when using the same input secret
	if secretStr1 != secretStr2 {
		t.Error("Secret strings differ when using the same provided secret")
	}

	// Secret bytes should match the provided secret
	if string(secret1) != string(providedSecret) {
		t.Error("Returned secret bytes do not match provided secret (first call)")
	}
	if string(secret2) != string(providedSecret) {
		t.Error("Returned secret bytes do not match provided secret (second call)")
	}
}

func TestGenerateQRCode_ConcurrentGeneration(t *testing.T) {
	username := "testuser"
	siteUrl := "opengist.io"
	concurrency := 10

	type result struct {
		secretStr   string
		secretBytes []byte
		err         error
	}

	results := make(chan result, concurrency)
	var wg sync.WaitGroup

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			secretStr, _, secretBytes, err := GenerateQRCode(username, siteUrl, nil)
			results <- result{secretStr: secretStr, secretBytes: secretBytes, err: err}
		}()
	}

	wg.Wait()
	close(results)

	secrets := make(map[string]bool)
	for res := range results {
		if res.err != nil {
			t.Errorf("Concurrent generation failed: %v", res.err)
			continue
		}

		// Check for duplicates
		if secrets[res.secretStr] {
			t.Error("Duplicate secret generated in concurrent test")
		}
		secrets[res.secretStr] = true
	}
}

func TestValidate(t *testing.T) {
	// Generate a valid secret for testing
	_, _, secret, err := GenerateQRCode("testuser", "opengist.io", nil)
	if err != nil {
		t.Fatalf("Failed to generate secret: %v", err)
	}

	// Convert secret bytes to base32 string for TOTP
	secretStr, _, _, err := GenerateQRCode("testuser", "opengist.io", secret)
	if err != nil {
		t.Fatalf("Failed to generate secret string: %v", err)
	}

	// Generate a valid passcode for the current time
	validPasscode, err := totp.GenerateCode(secretStr, time.Now())
	if err != nil {
		t.Fatalf("Failed to generate valid passcode: %v", err)
	}

	tests := []struct {
		name      string
		passcode  string
		secret    string
		wantValid bool
	}{
		{
			name:      "valid passcode",
			passcode:  validPasscode,
			secret:    secretStr,
			wantValid: true,
		},
		{
			name:      "invalid passcode - wrong digits",
			passcode:  "000000",
			secret:    secretStr,
			wantValid: false,
		},
		{
			name:      "invalid passcode - wrong length",
			passcode:  "123",
			secret:    secretStr,
			wantValid: false,
		},
		{
			name:      "empty passcode",
			passcode:  "",
			secret:    secretStr,
			wantValid: false,
		},
		{
			name:      "empty secret",
			passcode:  validPasscode,
			secret:    "",
			wantValid: false,
		},
		{
			name:      "invalid secret format",
			passcode:  validPasscode,
			secret:    "not-a-valid-base32-secret!@#",
			wantValid: false,
		},
		{
			name:      "passcode with letters",
			passcode:  "12345A",
			secret:    secretStr,
			wantValid: false,
		},
		{
			name:      "passcode with spaces",
			passcode:  "123 456",
			secret:    secretStr,
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := Validate(tt.passcode, tt.secret)
			if valid != tt.wantValid {
				t.Errorf("Validate() = %v, want %v", valid, tt.wantValid)
			}
		})
	}
}

func TestValidate_TimeDrift(t *testing.T) {
	// Generate a valid secret
	secretStr, _, _, err := GenerateQRCode("testuser", "opengist.io", nil)
	if err != nil {
		t.Fatalf("Failed to generate secret: %v", err)
	}

	// Test that passcodes from previous and next time windows are accepted
	// (TOTP typically accepts codes from ±1 time window for clock drift)
	pastTime := time.Now().Add(-30 * time.Second)
	futureTime := time.Now().Add(30 * time.Second)

	pastPasscode, err := totp.GenerateCode(secretStr, pastTime)
	if err != nil {
		t.Fatalf("Failed to generate past passcode: %v", err)
	}

	futurePasscode, err := totp.GenerateCode(secretStr, futureTime)
	if err != nil {
		t.Fatalf("Failed to generate future passcode: %v", err)
	}

	// These should be valid due to time drift tolerance
	if !Validate(pastPasscode, secretStr) {
		t.Error("Validate() rejected passcode from previous time window")
	}

	if !Validate(futurePasscode, secretStr) {
		t.Error("Validate() rejected passcode from next time window")
	}
}

func TestValidate_ExpiredPasscode(t *testing.T) {
	// Generate a valid secret
	secretStr, _, _, err := GenerateQRCode("testuser", "opengist.io", nil)
	if err != nil {
		t.Fatalf("Failed to generate secret: %v", err)
	}

	// Generate a passcode from 2 minutes ago (should be expired)
	oldTime := time.Now().Add(-2 * time.Minute)
	oldPasscode, err := totp.GenerateCode(secretStr, oldTime)
	if err != nil {
		t.Fatalf("Failed to generate old passcode: %v", err)
	}

	// This should be invalid
	if Validate(oldPasscode, secretStr) {
		t.Error("Validate() accepted expired passcode from 2 minutes ago")
	}
}

func TestValidate_RoundTrip(t *testing.T) {
	// Test full round trip: generate secret, generate code, validate code
	tests := []struct {
		name     string
		username string
		siteUrl  string
	}{
		{
			name:     "basic round trip",
			username: "testuser",
			siteUrl:  "opengist.io",
		},
		{
			name:     "round trip with dot in username",
			username: "test.user",
			siteUrl:  "opengist.io",
		},
		{
			name:     "round trip with hyphen in username",
			username: "test-user",
			siteUrl:  "opengist.io",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Generate QR code and secret
			secretStr, _, _, err := GenerateQRCode(tt.username, tt.siteUrl, nil)
			if err != nil {
				t.Fatalf("GenerateQRCode() failed: %v", err)
			}

			// Generate a valid passcode
			passcode, err := totp.GenerateCode(secretStr, time.Now())
			if err != nil {
				t.Fatalf("GenerateCode() failed: %v", err)
			}

			// Validate the passcode
			if !Validate(passcode, secretStr) {
				t.Error("Validate() rejected valid passcode")
			}

			// Validate wrong passcode fails
			wrongPasscode := "000000"
			if passcode == wrongPasscode {
				wrongPasscode = "111111"
			}
			if Validate(wrongPasscode, secretStr) {
				t.Error("Validate() accepted invalid passcode")
			}
		})
	}
}

func TestGenerateSecret(t *testing.T) {
	// Test the internal generateSecret function behavior through GenerateQRCode
	for i := 0; i < 10; i++ {
		_, _, secret, err := GenerateQRCode("testuser", "opengist.io", nil)
		if err != nil {
			t.Fatalf("Iteration %d: generateSecret() failed: %v", i, err)
		}

		if len(secret) != secretSize {
			t.Errorf("Iteration %d: secret length = %d, want %d", i, len(secret), secretSize)
		}

		// Verify secret is not all zeros (extremely unlikely with crypto/rand)
		allZeros := true
		for _, b := range secret {
			if b != 0 {
				allZeros = false
				break
			}
		}
		if allZeros {
			t.Errorf("Iteration %d: secret is all zeros", i)
		}
	}
}
