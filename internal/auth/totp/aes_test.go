package totp

import (
	"bytes"
	"crypto/aes"
	"testing"
)

func TestAESEncrypt(t *testing.T) {
	tests := []struct {
		name    string
		key     []byte
		text    []byte
		wantErr bool
	}{
		{
			name:    "basic encryption with 16-byte key",
			key:     []byte("1234567890123456"), // 16 bytes (AES-128)
			text:    []byte("hello world"),
			wantErr: false,
		},
		{
			name:    "basic encryption with 24-byte key",
			key:     []byte("123456789012345678901234"), // 24 bytes (AES-192)
			text:    []byte("hello world"),
			wantErr: false,
		},
		{
			name:    "basic encryption with 32-byte key",
			key:     []byte("12345678901234567890123456789012"), // 32 bytes (AES-256)
			text:    []byte("hello world"),
			wantErr: false,
		},
		{
			name:    "empty text",
			key:     []byte("1234567890123456"),
			text:    []byte(""),
			wantErr: false,
		},
		{
			name:    "long text",
			key:     []byte("1234567890123456"),
			text:    []byte("This is a much longer text that spans multiple blocks and should be encrypted properly without any issues"),
			wantErr: false,
		},
		{
			name:    "binary data",
			key:     []byte("1234567890123456"),
			text:    []byte{0x00, 0x01, 0x02, 0x03, 0xFF, 0xFE, 0xFD},
			wantErr: false,
		},
		{
			name:    "invalid key length - too short",
			key:     []byte("short"),
			text:    []byte("hello world"),
			wantErr: true,
		},
		{
			name:    "invalid key length - 17 bytes",
			key:     []byte("12345678901234567"), // 17 bytes (invalid)
			text:    []byte("hello world"),
			wantErr: true,
		},
		{
			name:    "nil key",
			key:     nil,
			text:    []byte("hello world"),
			wantErr: true,
		},
		{
			name:    "empty key",
			key:     []byte(""),
			text:    []byte("hello world"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ciphertext, err := AESEncrypt(tt.key, tt.text)
			if (err != nil) != tt.wantErr {
				t.Errorf("AESEncrypt() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify ciphertext is not empty
				if len(ciphertext) == 0 {
					t.Error("AESEncrypt() returned empty ciphertext")
				}

				// Verify ciphertext length is correct (IV + encrypted text)
				expectedLen := aes.BlockSize + len(tt.text)
				if len(ciphertext) != expectedLen {
					t.Errorf("AESEncrypt() ciphertext length = %d, want %d", len(ciphertext), expectedLen)
				}

				// Verify ciphertext is different from plaintext (unless text is empty)
				if len(tt.text) > 0 && bytes.Equal(ciphertext[aes.BlockSize:], tt.text) {
					t.Error("AESEncrypt() ciphertext matches plaintext")
				}

				// Verify IV is present and non-zero
				iv := ciphertext[:aes.BlockSize]
				allZeros := true
				for _, b := range iv {
					if b != 0 {
						allZeros = false
						break
					}
				}
				if allZeros {
					t.Error("AESEncrypt() IV is all zeros")
				}
			}
		})
	}
}

func TestAESDecrypt(t *testing.T) {
	validKey := []byte("1234567890123456")
	validText := []byte("hello world")

	// Encrypt some data to use for valid test cases
	validCiphertext, err := AESEncrypt(validKey, validText)
	if err != nil {
		t.Fatalf("Failed to create valid ciphertext: %v", err)
	}

	tests := []struct {
		name       string
		key        []byte
		ciphertext []byte
		wantErr    bool
	}{
		{
			name:       "valid decryption",
			key:        validKey,
			ciphertext: validCiphertext,
			wantErr:    false,
		},
		{
			name:       "ciphertext too short - empty",
			key:        validKey,
			ciphertext: []byte(""),
			wantErr:    true,
		},
		{
			name:       "ciphertext too short - less than block size",
			key:        validKey,
			ciphertext: []byte("short"),
			wantErr:    true,
		},
		{
			name:       "ciphertext exactly block size (IV only, no data)",
			key:        validKey,
			ciphertext: make([]byte, aes.BlockSize),
			wantErr:    false,
		},
		{
			name:       "invalid key length",
			key:        []byte("short"),
			ciphertext: validCiphertext,
			wantErr:    true,
		},
		{
			name:       "wrong key",
			key:        []byte("6543210987654321"),
			ciphertext: validCiphertext,
			wantErr:    false, // Decryption succeeds but produces garbage
		},
		{
			name:       "nil key",
			key:        nil,
			ciphertext: validCiphertext,
			wantErr:    true,
		},
		{
			name:       "nil ciphertext",
			key:        validKey,
			ciphertext: nil,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plaintext, err := AESDecrypt(tt.key, tt.ciphertext)
			if (err != nil) != tt.wantErr {
				t.Errorf("AESDecrypt() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// For valid decryption with correct key, verify we get original text
				if tt.name == "valid decryption" && !bytes.Equal(plaintext, validText) {
					t.Errorf("AESDecrypt() plaintext = %v, want %v", plaintext, validText)
				}

				// For ciphertext with only IV, plaintext should be empty
				if tt.name == "ciphertext exactly block size (IV only, no data)" && len(plaintext) != 0 {
					t.Errorf("AESDecrypt() plaintext length = %d, want 0", len(plaintext))
				}
			}
		})
	}
}

func TestAESEncryptDecrypt_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		key  []byte
		text []byte
	}{
		{
			name: "basic round trip",
			key:  []byte("1234567890123456"),
			text: []byte("hello world"),
		},
		{
			name: "empty text round trip",
			key:  []byte("1234567890123456"),
			text: []byte(""),
		},
		{
			name: "long text round trip",
			key:  []byte("1234567890123456"),
			text: []byte("This is a very long text that contains multiple blocks of data and should be encrypted and decrypted correctly without any data loss or corruption"),
		},
		{
			name: "binary data round trip",
			key:  []byte("1234567890123456"),
			text: []byte{0x00, 0x01, 0x02, 0x03, 0xFF, 0xFE, 0xFD, 0xFC},
		},
		{
			name: "unicode text round trip",
			key:  []byte("1234567890123456"),
			text: []byte("Hello 世界! 🔐 Encryption"),
		},
		{
			name: "AES-192 round trip",
			key:  []byte("123456789012345678901234"),
			text: []byte("testing AES-192"),
		},
		{
			name: "AES-256 round trip",
			key:  []byte("12345678901234567890123456789012"),
			text: []byte("testing AES-256"),
		},
		{
			name: "special characters",
			key:  []byte("1234567890123456"),
			text: []byte("!@#$%^&*()_+-=[]{}|;':\",./<>?"),
		},
		{
			name: "newlines and tabs",
			key:  []byte("1234567890123456"),
			text: []byte("line1\nline2\tline3\r\nline4"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encrypt
			ciphertext, err := AESEncrypt(tt.key, tt.text)
			if err != nil {
				t.Fatalf("AESEncrypt() failed: %v", err)
			}

			// Decrypt
			plaintext, err := AESDecrypt(tt.key, ciphertext)
			if err != nil {
				t.Fatalf("AESDecrypt() failed: %v", err)
			}

			// Verify plaintext matches original
			if !bytes.Equal(plaintext, tt.text) {
				t.Errorf("Round trip failed: got %v, want %v", plaintext, tt.text)
			}
		})
	}
}

func TestAESEncrypt_Uniqueness(t *testing.T) {
	key := []byte("1234567890123456")
	text := []byte("hello world")
	iterations := 10

	ciphertexts := make(map[string]bool)

	for i := 0; i < iterations; i++ {
		ciphertext, err := AESEncrypt(key, text)
		if err != nil {
			t.Fatalf("Iteration %d failed: %v", i, err)
		}

		// Each encryption should produce different ciphertext (due to random IV)
		ciphertextStr := string(ciphertext)
		if ciphertexts[ciphertextStr] {
			t.Errorf("Duplicate ciphertext generated at iteration %d", i)
		}
		ciphertexts[ciphertextStr] = true

		// But all should decrypt to the same plaintext
		plaintext, err := AESDecrypt(key, ciphertext)
		if err != nil {
			t.Fatalf("Iteration %d decryption failed: %v", i, err)
		}
		if !bytes.Equal(plaintext, text) {
			t.Errorf("Iteration %d: decrypted text doesn't match original", i)
		}
	}
}

func TestAESEncrypt_IVUniqueness(t *testing.T) {
	key := []byte("1234567890123456")
	text := []byte("test data")
	iterations := 20

	ivs := make(map[string]bool)

	for i := 0; i < iterations; i++ {
		ciphertext, err := AESEncrypt(key, text)
		if err != nil {
			t.Fatalf("Iteration %d failed: %v", i, err)
		}

		// Extract IV (first block)
		iv := ciphertext[:aes.BlockSize]
		ivStr := string(iv)

		// Each IV should be unique
		if ivs[ivStr] {
			t.Errorf("Duplicate IV generated at iteration %d", i)
		}
		ivs[ivStr] = true
	}
}

func TestAESDecrypt_WrongKey(t *testing.T) {
	originalKey := []byte("1234567890123456")
	wrongKey := []byte("6543210987654321")
	text := []byte("secret message")

	// Encrypt with original key
	ciphertext, err := AESEncrypt(originalKey, text)
	if err != nil {
		t.Fatalf("AESEncrypt() failed: %v", err)
	}

	// Decrypt with wrong key - should not error but produce wrong plaintext
	plaintext, err := AESDecrypt(wrongKey, ciphertext)
	if err != nil {
		t.Fatalf("AESDecrypt() with wrong key failed: %v", err)
	}

	// Plaintext should be different from original
	if bytes.Equal(plaintext, text) {
		t.Error("AESDecrypt() with wrong key produced correct plaintext")
	}
}

func TestAESDecrypt_CorruptedCiphertext(t *testing.T) {
	key := []byte("1234567890123456")
	text := []byte("hello world")

	// Encrypt
	ciphertext, err := AESEncrypt(key, text)
	if err != nil {
		t.Fatalf("AESEncrypt() failed: %v", err)
	}

	// Corrupt the ciphertext (flip a bit in the encrypted data, not the IV)
	if len(ciphertext) > aes.BlockSize {
		corruptedCiphertext := make([]byte, len(ciphertext))
		copy(corruptedCiphertext, ciphertext)
		corruptedCiphertext[aes.BlockSize] ^= 0xFF

		// Decrypt corrupted ciphertext - should not error but produce wrong plaintext
		plaintext, err := AESDecrypt(key, corruptedCiphertext)
		if err != nil {
			t.Fatalf("AESDecrypt() with corrupted ciphertext failed: %v", err)
		}

		// Plaintext should be different from original
		if bytes.Equal(plaintext, text) {
			t.Error("AESDecrypt() with corrupted ciphertext produced correct plaintext")
		}
	}
}

func TestAESEncryptDecrypt_DifferentKeySizes(t *testing.T) {
	tests := []struct {
		name    string
		keySize int
	}{
		{"AES-128", 16},
		{"AES-192", 24},
		{"AES-256", 32},
	}

	text := []byte("test message for different key sizes")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Generate key of specified size
			key := make([]byte, tt.keySize)
			for i := range key {
				key[i] = byte(i)
			}

			// Encrypt
			ciphertext, err := AESEncrypt(key, text)
			if err != nil {
				t.Fatalf("AESEncrypt() failed: %v", err)
			}

			// Decrypt
			plaintext, err := AESDecrypt(key, ciphertext)
			if err != nil {
				t.Fatalf("AESDecrypt() failed: %v", err)
			}

			// Verify
			if !bytes.Equal(plaintext, text) {
				t.Errorf("Round trip failed for %s", tt.name)
			}
		})
	}
}
