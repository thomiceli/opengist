package password

import (
	"strings"
	"testing"
)

func TestHashPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{
			name:     "simple password",
			password: "password123",
			wantErr:  false,
		},
		{
			name:     "empty password",
			password: "",
			wantErr:  false,
		},
		{
			name:     "long password",
			password: strings.Repeat("a", 1000),
			wantErr:  false,
		},
		{
			name:     "special characters",
			password: "p@ssw0rd!#$%^&*()",
			wantErr:  false,
		},
		{
			name:     "unicode characters",
			password: "パスワード123",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := HashPassword(tt.password)
			if (err != nil) != tt.wantErr {
				t.Errorf("HashPassword() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				// Verify hash format
				if !strings.HasPrefix(hash, "$argon2id$") {
					t.Errorf("HashPassword() returned invalid hash format: %v", hash)
				}
				// Verify hash has correct number of parts
				parts := strings.Split(hash, "$")
				if len(parts) != 6 {
					t.Errorf("HashPassword() returned hash with incorrect number of parts: %v", len(parts))
				}
			}
		})
	}
}

func TestVerifyPassword(t *testing.T) {
	// Pre-generate a known hash for testing
	testPassword := "testpassword123"
	testHash, err := HashPassword(testPassword)
	if err != nil {
		t.Fatalf("Failed to generate test hash: %v", err)
	}

	tests := []struct {
		name      string
		password  string
		hash      string
		wantMatch bool
		wantErr   bool
	}{
		{
			name:      "correct password",
			password:  testPassword,
			hash:      testHash,
			wantMatch: true,
			wantErr:   false,
		},
		{
			name:      "incorrect password",
			password:  "wrongpassword",
			hash:      testHash,
			wantMatch: false,
			wantErr:   false,
		},
		{
			name:      "empty password against valid hash",
			password:  "",
			hash:      testHash,
			wantMatch: false,
			wantErr:   false,
		},
		{
			name:      "empty hash",
			password:  testPassword,
			hash:      "",
			wantMatch: false,
			wantErr:   false,
		},
		{
			name:      "invalid hash format",
			password:  testPassword,
			hash:      "invalid",
			wantMatch: false,
			wantErr:   true,
		},
		{
			name:      "malformed hash - wrong prefix",
			password:  testPassword,
			hash:      "$bcrypt$invalid$hash",
			wantMatch: false,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match, err := VerifyPassword(tt.password, tt.hash)
			if (err != nil) != tt.wantErr {
				t.Errorf("VerifyPassword() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if match != tt.wantMatch {
				t.Errorf("VerifyPassword() match = %v, wantMatch %v", match, tt.wantMatch)
			}
		})
	}
}

func TestHashPasswordUniqueness(t *testing.T) {
	password := "testpassword"

	// Generate multiple hashes of the same password
	hash1, err := HashPassword(password)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	hash2, err := HashPassword(password)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	// Hashes should be different due to different salts
	if hash1 == hash2 {
		t.Error("HashPassword() should generate unique hashes for the same password")
	}

	// But both should verify correctly
	match1, err := VerifyPassword(password, hash1)
	if err != nil || !match1 {
		t.Errorf("Failed to verify first hash: err=%v, match=%v", err, match1)
	}

	match2, err := VerifyPassword(password, hash2)
	if err != nil || !match2 {
		t.Errorf("Failed to verify second hash: err=%v, match=%v", err, match2)
	}
}

func TestPasswordRoundTrip(t *testing.T) {
	tests := []string{
		"simple",
		"with spaces and special chars !@#$%",
		"パスワード",
		strings.Repeat("long", 100),
		"",
	}

	for _, password := range tests {
		t.Run(password, func(t *testing.T) {
			hash, err := HashPassword(password)
			if err != nil {
				t.Fatalf("HashPassword() failed: %v", err)
			}

			match, err := VerifyPassword(password, hash)
			if err != nil {
				t.Fatalf("VerifyPassword() failed: %v", err)
			}

			if !match {
				t.Error("Password round trip failed: hashed password does not verify")
			}
		})
	}
}
