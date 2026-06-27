package password

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestArgon2ID_Hash(t *testing.T) {
	tests := []struct {
		name    string
		plain   string
		wantErr bool
	}{
		{
			name:    "basic password",
			plain:   "password123",
			wantErr: false,
		},
		{
			name:    "empty string",
			plain:   "",
			wantErr: false,
		},
		{
			name:    "long password",
			plain:   strings.Repeat("a", 10000),
			wantErr: false,
		},
		{
			name:    "unicode password",
			plain:   "パスワード🔒",
			wantErr: false,
		},
		{
			name:    "special characters",
			plain:   "!@#$%^&*()_+-=[]{}|;:',.<>?/`~",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := Argon2id.Hash(tt.plain)
			if (err != nil) != tt.wantErr {
				t.Errorf("Argon2id.Hash() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify the hash format
				if !strings.HasPrefix(hash, "$argon2id$") {
					t.Errorf("Hash does not start with $argon2id$: %v", hash)
				}

				// Verify all parts are present
				parts := strings.Split(hash, "$")
				if len(parts) != 6 {
					t.Errorf("Hash has %d parts, expected 6: %v", len(parts), hash)
				}

				// Verify salt is properly encoded
				if len(parts) >= 5 {
					_, err := base64.RawStdEncoding.DecodeString(parts[4])
					if err != nil {
						t.Errorf("Salt is not properly base64 encoded: %v", err)
					}
				}

				// Verify hash is properly encoded
				if len(parts) >= 6 {
					_, err := base64.RawStdEncoding.DecodeString(parts[5])
					if err != nil {
						t.Errorf("Hash is not properly base64 encoded: %v", err)
					}
				}
			}
		})
	}
}

func TestArgon2ID_Verify(t *testing.T) {
	// Generate a valid hash for testing
	testPassword := "correctpassword"
	validHash, err := Argon2id.Hash(testPassword)
	if err != nil {
		t.Fatalf("Failed to generate test hash: %v", err)
	}

	tests := []struct {
		name      string
		plain     string
		hash      string
		wantMatch bool
		wantErr   bool
	}{
		{
			name:      "correct password",
			plain:     testPassword,
			hash:      validHash,
			wantMatch: true,
			wantErr:   false,
		},
		{
			name:      "incorrect password",
			plain:     "wrongpassword",
			hash:      validHash,
			wantMatch: false,
			wantErr:   false,
		},
		{
			name:      "empty password",
			plain:     "",
			hash:      validHash,
			wantMatch: false,
			wantErr:   false,
		},
		{
			name:      "empty hash",
			plain:     testPassword,
			hash:      "",
			wantMatch: false,
			wantErr:   false,
		},
		{
			name:      "invalid hash - too few parts",
			plain:     testPassword,
			hash:      "$argon2id$v=19$m=65536",
			wantMatch: false,
			wantErr:   true,
		},
		{
			name:      "invalid hash - too many parts",
			plain:     testPassword,
			hash:      "$argon2id$v=19$m=65536,t=1,p=4$salt$hash$extra",
			wantMatch: false,
			wantErr:   true,
		},
		{
			name:      "invalid hash - malformed parameters",
			plain:     testPassword,
			hash:      "$argon2id$v=19$invalid$salt$hash",
			wantMatch: false,
			wantErr:   true,
		},
		{
			name:      "invalid hash - bad base64 salt",
			plain:     testPassword,
			hash:      "$argon2id$v=19$m=65536,t=1,p=4$not-valid-base64!@#$hash",
			wantMatch: false,
			wantErr:   true,
		},
		{
			name:      "invalid hash - bad base64 hash",
			plain:     testPassword,
			hash:      "$argon2id$v=19$m=65536,t=1,p=4$dGVzdA$not-valid-base64!@#",
			wantMatch: false,
			wantErr:   true,
		},
		{
			name:      "wrong algorithm prefix",
			plain:     testPassword,
			hash:      "$bcrypt$rounds=10$saltsaltsaltsaltsalt",
			wantMatch: false,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match, err := Argon2id.Verify(tt.plain, tt.hash)
			if (err != nil) != tt.wantErr {
				t.Errorf("Argon2id.Verify() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if match != tt.wantMatch {
				t.Errorf("Argon2id.Verify() match = %v, wantMatch %v", match, tt.wantMatch)
			}
		})
	}
}

func TestArgon2ID_SaltUniqueness(t *testing.T) {
	password := "testpassword"
	iterations := 10

	hashes := make(map[string]bool)
	salts := make(map[string]bool)

	for i := 0; i < iterations; i++ {
		hash, err := Argon2id.Hash(password)
		if err != nil {
			t.Fatalf("Hash iteration %d failed: %v", i, err)
		}

		// Check hash uniqueness
		if hashes[hash] {
			t.Errorf("Duplicate hash generated at iteration %d", i)
		}
		hashes[hash] = true

		// Extract and check salt uniqueness
		parts := strings.Split(hash, "$")
		if len(parts) >= 5 {
			salt := parts[4]
			if salts[salt] {
				t.Errorf("Duplicate salt generated at iteration %d", i)
			}
			salts[salt] = true
		}

		// Verify each hash works
		match, err := Argon2id.Verify(password, hash)
		if err != nil || !match {
			t.Errorf("Hash %d failed verification: err=%v, match=%v", i, err, match)
		}
	}
}

func TestArgon2ID_HashFormat(t *testing.T) {
	password := "testformat"
	hash, err := Argon2id.Hash(password)
	if err != nil {
		t.Fatalf("Hash failed: %v", err)
	}

	parts := strings.Split(hash, "$")
	if len(parts) != 6 {
		t.Fatalf("Expected 6 parts, got %d: %v", len(parts), hash)
	}

	// Part 0 should be empty (before first $)
	if parts[0] != "" {
		t.Errorf("Part 0 should be empty, got: %v", parts[0])
	}

	// Part 1 should be "argon2id"
	if parts[1] != "argon2id" {
		t.Errorf("Part 1 should be 'argon2id', got: %v", parts[1])
	}

	// Part 2 should be version
	if !strings.HasPrefix(parts[2], "v=") {
		t.Errorf("Part 2 should start with 'v=', got: %v", parts[2])
	}

	// Part 3 should be parameters
	if !strings.Contains(parts[3], "m=") || !strings.Contains(parts[3], "t=") || !strings.Contains(parts[3], "p=") {
		t.Errorf("Part 3 should contain m=, t=, and p=, got: %v", parts[3])
	}

	// Part 4 should be base64 encoded salt
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		t.Errorf("Salt (part 4) is not valid base64: %v", err)
	}
	if len(salt) != int(Argon2id.saltLen) {
		t.Errorf("Salt length is %d, expected %d", len(salt), Argon2id.saltLen)
	}

	// Part 5 should be base64 encoded hash
	decodedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		t.Errorf("Hash (part 5) is not valid base64: %v", err)
	}
	if len(decodedHash) != int(Argon2id.keyLen) {
		t.Errorf("Hash length is %d, expected %d", len(decodedHash), Argon2id.keyLen)
	}
}

func TestArgon2ID_CaseModification(t *testing.T) {
	// Passwords should be case-sensitive
	password := "TestPassword"
	hash, err := Argon2id.Hash(password)
	if err != nil {
		t.Fatalf("Hash failed: %v", err)
	}

	// Correct case should match
	match, err := Argon2id.Verify(password, hash)
	if err != nil || !match {
		t.Errorf("Correct password failed: err=%v, match=%v", err, match)
	}

	// Wrong case should not match
	match, err = Argon2id.Verify("testpassword", hash)
	if err != nil {
		t.Errorf("Verify returned error: %v", err)
	}
	if match {
		t.Error("Password verification should be case-sensitive")
	}

	match, err = Argon2id.Verify("TESTPASSWORD", hash)
	if err != nil {
		t.Errorf("Verify returned error: %v", err)
	}
	if match {
		t.Error("Password verification should be case-sensitive")
	}
}

func TestArgon2ID_InvalidParameters(t *testing.T) {
	password := "testpassword"

	tests := []struct {
		name    string
		hash    string
		wantErr bool
	}{
		{
			name:    "negative memory parameter",
			hash:    "$argon2id$v=19$m=-1,t=1,p=4$dGVzdHNhbHQ$testhash",
			wantErr: true,
		},
		{
			name:    "negative time parameter",
			hash:    "$argon2id$v=19$m=65536,t=-1,p=4$dGVzdHNhbHQ$testhash",
			wantErr: true,
		},
		{
			name:    "negative parallelism parameter",
			hash:    "$argon2id$v=19$m=65536,t=1,p=-4$dGVzdHNhbHQ$testhash",
			wantErr: true,
		},
		{
			name:    "zero memory parameter",
			hash:    "$argon2id$v=19$m=0,t=1,p=4$dGVzdHNhbHQ$testhash",
			wantErr: false, // argon2 may handle this, we just test parsing
		},
		{
			name:    "missing parameter value",
			hash:    "$argon2id$v=19$m=,t=1,p=4$dGVzdHNhbHQ$testhash",
			wantErr: true,
		},
		{
			name:    "non-numeric parameter",
			hash:    "$argon2id$v=19$m=abc,t=1,p=4$dGVzdHNhbHQ$testhash",
			wantErr: true,
		},
		{
			name:    "missing parameters separator",
			hash:    "$argon2id$v=19$m=65536 t=1 p=4$dGVzdHNhbHQ$testhash",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Argon2id.Verify(password, tt.hash)
			if (err != nil) != tt.wantErr {
				t.Errorf("Argon2id.Verify() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestArgon2ID_ConcurrentHashing(t *testing.T) {
	password := "testpassword"
	concurrency := 10

	type result struct {
		hash string
		err  error
	}

	results := make(chan result, concurrency)

	// Generate hashes concurrently
	for i := 0; i < concurrency; i++ {
		go func() {
			hash, err := Argon2id.Hash(password)
			results <- result{hash: hash, err: err}
		}()
	}

	// Collect results
	hashes := make(map[string]bool)
	for i := 0; i < concurrency; i++ {
		res := <-results
		if res.err != nil {
			t.Errorf("Concurrent hash %d failed: %v", i, res.err)
			continue
		}

		// Check for duplicates
		if hashes[res.hash] {
			t.Errorf("Duplicate hash generated in concurrent test")
		}
		hashes[res.hash] = true

		// Verify each hash works
		match, err := Argon2id.Verify(password, res.hash)
		if err != nil || !match {
			t.Errorf("Hash %d failed verification: err=%v, match=%v", i, err, match)
		}
	}
}

func TestArgon2ID_VeryLongPassword(t *testing.T) {
	// Test with extremely long password (100KB)
	password := strings.Repeat("a", 100*1024)

	hash, err := Argon2id.Hash(password)
	if err != nil {
		t.Fatalf("Failed to hash very long password: %v", err)
	}

	match, err := Argon2id.Verify(password, hash)
	if err != nil {
		t.Fatalf("Failed to verify very long password: %v", err)
	}

	if !match {
		t.Error("Very long password failed verification")
	}

	// Verify wrong password still fails
	wrongPassword := strings.Repeat("b", 100*1024)
	match, err = Argon2id.Verify(wrongPassword, hash)
	if err != nil {
		t.Errorf("Verify returned error: %v", err)
	}
	if match {
		t.Error("Wrong very long password should not match")
	}
}
