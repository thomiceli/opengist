package ldap

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/go-ldap/ldap/v3"
	"github.com/thomiceli/opengist/internal/config"
)

// TestMain sets up and tears down the test environment
func TestMain(m *testing.M) {
	// Only run tests if OG_TEST_LDAP=1
	if os.Getenv("OG_TEST_LDAP") != "1" {
		fmt.Println("Skipping LDAP tests. Set OG_TEST_LDAP=1 to run them.")
		os.Exit(0)
	}

	// Run tests
	code := m.Run()
	os.Exit(code)
}

// setupTestConfig configures the test LDAP connection using docker-test-openldap defaults
func setupTestConfig() {
	// docker-test-openldap default configuration
	// Container should be run with: docker run --rm -p 10389:10389 -p 10636:10636 rroemhild/test-openldap
	config.C = &config.AllConfig{
		LDAPUrl:             getEnvOrDefault("OG_LDAP_URL", "ldap://localhost:10389"),
		LDAPBindDn:          getEnvOrDefault("OG_LDAP_BIND_DN", "cn=admin,dc=planetexpress,dc=com"),
		LDAPBindCredentials: getEnvOrDefault("OG_LDAP_BIND_CREDENTIALS", "GoodNewsEveryone"),
		LDAPSearchBase:      getEnvOrDefault("OG_LDAP_SEARCH_BASE", "ou=people,dc=planetexpress,dc=com"),
		LDAPSearchFilter:    getEnvOrDefault("OG_LDAP_SEARCH_FILTER", "(uid=%s)"),
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// waitForLDAP waits for the LDAP server to be available
func waitForLDAP(url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := ldap.DialURL(url)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("LDAP server not available after %v", timeout)
}

func TestEnabled(t *testing.T) {
	tests := []struct {
		name    string
		ldapUrl string
		want    bool
	}{
		{
			name:    "LDAP enabled with URL",
			ldapUrl: "ldap://localhost:389",
			want:    true,
		},
		{
			name:    "LDAP disabled with empty URL",
			ldapUrl: "",
			want:    false,
		},
		{
			name:    "LDAP enabled with LDAPS URL",
			ldapUrl: "ldaps://localhost:636",
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original config
			originalConfig := config.C
			defer func() { config.C = originalConfig }()

			// Set test config
			config.C = &config.AllConfig{
				LDAPUrl: tt.ldapUrl,
			}

			if got := Enabled(); got != tt.want {
				t.Errorf("Enabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAuthenticate_Integration(t *testing.T) {
	setupTestConfig()

	// Wait for LDAP server to be ready
	if err := waitForLDAP(config.C.LDAPUrl, 10*time.Second); err != nil {
		t.Fatalf("LDAP server not available: %v\nMake sure docker-test-openldap is running: docker run --rm -p 10389:10389 rroemhild/test-openldap", err)
	}

	tests := []struct {
		name        string
		username    string
		password    string
		wantSuccess bool
		wantError   bool
	}{
		{
			name:        "valid credentials - fry",
			username:    "fry",
			password:    "fry",
			wantSuccess: true,
			wantError:   false,
		},
		{
			name:        "valid credentials - professor",
			username:    "professor",
			password:    "professor",
			wantSuccess: true,
			wantError:   false,
		},
		{
			name:        "valid credentials - leela",
			username:    "leela",
			password:    "leela",
			wantSuccess: true,
			wantError:   false,
		},
		{
			name:        "valid credentials - bender",
			username:    "bender",
			password:    "bender",
			wantSuccess: true,
			wantError:   false,
		},
		{
			name:        "invalid password",
			username:    "fry",
			password:    "wrongpassword",
			wantSuccess: false,
			wantError:   false,
		},
		{
			name:        "non-existent user",
			username:    "nonexistent",
			password:    "password",
			wantSuccess: false,
			wantError:   false,
		},
		{
			name:        "empty username",
			username:    "",
			password:    "password",
			wantSuccess: false,
			wantError:   false,
		},
		{
			name:        "empty password",
			username:    "fry",
			password:    "",
			wantSuccess: false,
			wantError:   false,
		},
		{
			name:        "both empty",
			username:    "",
			password:    "",
			wantSuccess: false,
			wantError:   false,
		},
		{
			name:        "username with special characters",
			username:    "fry'; DROP TABLE users--",
			password:    "password",
			wantSuccess: false,
			wantError:   false,
		},
		{
			name:        "username with spaces",
			username:    "fry user",
			password:    "password",
			wantSuccess: false,
			wantError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			success, err := Authenticate(tt.username, tt.password)
			if (err != nil) != tt.wantError {
				t.Errorf("Authenticate() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if success != tt.wantSuccess {
				t.Errorf("Authenticate() success = %v, want %v", success, tt.wantSuccess)
			}
		})
	}
}

func TestAuthenticate_InvalidURL(t *testing.T) {
	// Save original config
	originalConfig := config.C
	defer func() { config.C = originalConfig }()

	// Set invalid LDAP URL
	config.C = &config.AllConfig{
		LDAPUrl: "ldap://invalid.host.local:389",
	}

	success, err := Authenticate("testuser", "testpass")
	if err == nil {
		t.Error("Authenticate() expected error for invalid URL, got nil")
	}
	if success {
		t.Error("Authenticate() should not succeed with invalid URL")
	}
}

func TestAuthenticate_InvalidBindCredentials(t *testing.T) {
	setupTestConfig()

	// Wait for LDAP server to be ready
	if err := waitForLDAP(config.C.LDAPUrl, 10*time.Second); err != nil {
		t.Skipf("LDAP server not available: %v", err)
	}

	// Save original config
	originalConfig := config.C
	defer func() { config.C = originalConfig }()

	// Set invalid bind credentials
	config.C.LDAPBindCredentials = "wrongpassword"

	success, err := Authenticate("fry", "fry")
	if err == nil {
		t.Error("Authenticate() expected error for invalid bind credentials, got nil")
	}
	if success {
		t.Error("Authenticate() should not succeed with invalid bind credentials")
	}
}

func TestAuthenticate_InvalidBindDN(t *testing.T) {
	setupTestConfig()

	// Wait for LDAP server to be ready
	if err := waitForLDAP(config.C.LDAPUrl, 10*time.Second); err != nil {
		t.Skipf("LDAP server not available: %v", err)
	}

	// Save original config
	originalConfig := config.C
	defer func() { config.C = originalConfig }()

	// Set invalid bind DN
	config.C.LDAPBindDn = "cn=invalid,dc=planetexpress,dc=com"

	success, err := Authenticate("fry", "fry")
	if err == nil {
		t.Error("Authenticate() expected error for invalid bind DN, got nil")
	}
	if success {
		t.Error("Authenticate() should not succeed with invalid bind DN")
	}
}

func TestAuthenticate_InvalidSearchBase(t *testing.T) {
	setupTestConfig()

	// Wait for LDAP server to be ready
	if err := waitForLDAP(config.C.LDAPUrl, 10*time.Second); err != nil {
		t.Skipf("LDAP server not available: %v", err)
	}

	// Save original config
	originalConfig := config.C
	defer func() { config.C = originalConfig }()

	// Set invalid search base
	config.C.LDAPSearchBase = "ou=invalid,dc=planetexpress,dc=com"

	success, err := Authenticate("fry", "fry")
	if err == nil {
		t.Error("Authenticate() expected error for invalid search base, got nil")
	}
	if success {
		t.Error("Authenticate() should not succeed with invalid search base")
	}
}

func TestAuthenticate_InvalidSearchFilter(t *testing.T) {
	setupTestConfig()

	// Wait for LDAP server to be ready
	if err := waitForLDAP(config.C.LDAPUrl, 10*time.Second); err != nil {
		t.Skipf("LDAP server not available: %v", err)
	}

	tests := []struct {
		name         string
		searchFilter string
		username     string
		password     string
		wantSuccess  bool
	}{
		{
			name:         "filter matches no users",
			searchFilter: "(uid=nonexistent-%s)",
			username:     "fry",
			password:     "fry",
			wantSuccess:  false,
		},
		{
			name:         "malformed filter",
			searchFilter: "uid=%s",
			username:     "fry",
			password:     "fry",
			wantSuccess:  false,
		},
		{
			name:         "filter with multiple attributes",
			searchFilter: "(&(uid=%s)(objectClass=inetOrgPerson))",
			username:     "fry",
			password:     "fry",
			wantSuccess:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original config
			originalConfig := config.C
			defer func() { config.C = originalConfig }()

			// Update search filter
			setupTestConfig()
			config.C.LDAPSearchFilter = tt.searchFilter

			success, _ := Authenticate(tt.username, tt.password)
			if success != tt.wantSuccess {
				t.Errorf("Authenticate() success = %v, want %v", success, tt.wantSuccess)
			}
		})
	}
}

func TestAuthenticate_MultipleUsers(t *testing.T) {
	setupTestConfig()

	// Wait for LDAP server to be ready
	if err := waitForLDAP(config.C.LDAPUrl, 10*time.Second); err != nil {
		t.Skipf("LDAP server not available: %v", err)
	}

	// Save original config
	originalConfig := config.C
	defer func() { config.C = originalConfig }()

	// Use a filter that matches multiple users (should fail)
	config.C.LDAPSearchFilter = "(objectClass=inetOrgPerson)"

	success, err := Authenticate("fry", "fry")
	if err != nil {
		t.Errorf("Authenticate() unexpected error: %v", err)
	}
	if success {
		t.Error("Authenticate() should fail when search filter matches multiple users")
	}
}

func TestAuthenticate_CaseSensitivity(t *testing.T) {
	setupTestConfig()

	// Wait for LDAP server to be ready
	if err := waitForLDAP(config.C.LDAPUrl, 10*time.Second); err != nil {
		t.Skipf("LDAP server not available: %v", err)
	}

	tests := []struct {
		name        string
		username    string
		password    string
		wantSuccess bool
	}{
		{
			name:        "lowercase username",
			username:    "fry",
			password:    "fry",
			wantSuccess: true,
		},
		{
			name:        "uppercase username",
			username:    "FRY",
			password:    "fry",
			wantSuccess: false,
		},
		{
			name:        "mixed case username",
			username:    "Fry",
			password:    "fry",
			wantSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			success, _ := Authenticate(tt.username, tt.password)
			if success != tt.wantSuccess {
				t.Errorf("Authenticate() success = %v, want %v", success, tt.wantSuccess)
			}
		})
	}
}

func TestAuthenticate_ConcurrentAuthentications(t *testing.T) {
	setupTestConfig()

	// Wait for LDAP server to be ready
	if err := waitForLDAP(config.C.LDAPUrl, 10*time.Second); err != nil {
		t.Skipf("LDAP server not available: %v", err)
	}

	// Test concurrent authentication requests
	concurrency := 10
	results := make(chan bool, concurrency)
	errors := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			success, err := Authenticate("fry", "fry")
			results <- success
			errors <- err
		}()
	}

	// Collect results
	successCount := 0
	errorCount := 0
	for i := 0; i < concurrency; i++ {
		if <-results {
			successCount++
		}
		if err := <-errors; err != nil {
			errorCount++
			t.Logf("Concurrent authentication error: %v", err)
		}
	}

	// All should succeed
	if successCount != concurrency {
		t.Errorf("Expected %d successful authentications, got %d", concurrency, successCount)
	}
	if errorCount > 0 {
		t.Errorf("Expected no errors, got %d", errorCount)
	}
}

func TestAuthenticate_SequentialAuthentications(t *testing.T) {
	setupTestConfig()

	// Wait for LDAP server to be ready
	if err := waitForLDAP(config.C.LDAPUrl, 10*time.Second); err != nil {
		t.Skipf("LDAP server not available: %v", err)
	}

	// Test multiple sequential authentications with different users
	users := []struct {
		username string
		password string
	}{
		{"fry", "fry"},
		{"leela", "leela"},
		{"bender", "bender"},
		{"professor", "professor"},
	}

	for _, user := range users {
		t.Run(user.username, func(t *testing.T) {
			success, err := Authenticate(user.username, user.password)
			if err != nil {
				t.Errorf("Authenticate(%s) error: %v", user.username, err)
			}
			if !success {
				t.Errorf("Authenticate(%s) failed", user.username)
			}
		})
	}
}

func TestAuthenticate_LongPassword(t *testing.T) {
	setupTestConfig()

	// Wait for LDAP server to be ready
	if err := waitForLDAP(config.C.LDAPUrl, 10*time.Second); err != nil {
		t.Skipf("LDAP server not available: %v", err)
	}

	// Test with a very long password
	longPassword := string(make([]byte, 10000))
	for i := range longPassword {
		longPassword = longPassword[:i] + "a" + longPassword[i+1:]
	}

	success, err := Authenticate("fry", longPassword)
	if err != nil {
		t.Errorf("Authenticate() unexpected error: %v", err)
	}
	if success {
		t.Error("Authenticate() should fail with wrong password")
	}
}

func TestAuthenticate_SpecialCharactersInPassword(t *testing.T) {
	setupTestConfig()

	// Wait for LDAP server to be ready
	if err := waitForLDAP(config.C.LDAPUrl, 10*time.Second); err != nil {
		t.Skipf("LDAP server not available: %v", err)
	}

	// Test with passwords containing special characters
	// Note: These will fail since they're not the actual passwords,
	// but we're testing that the system handles them correctly
	specialPasswords := []string{
		"pass@word!",
		"p@$$w0rd",
		"pāşšŵōŕđ", // Unicode characters
		"pass\nword",
		"pass\tword",
		"pass word",
		"pass'word",
		"pass\"word",
		"pass\\word",
	}

	for _, password := range specialPasswords {
		t.Run(fmt.Sprintf("password_%s", password), func(t *testing.T) {
			success, err := Authenticate("fry", password)
			if err != nil {
				t.Errorf("Authenticate() unexpected error: %v", err)
			}
			if success {
				t.Error("Authenticate() should fail with wrong password")
			}
		})
	}
}
