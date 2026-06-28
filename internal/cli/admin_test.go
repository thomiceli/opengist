package cli

import (
	"crypto/md5"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/auth/password"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/urfave/cli/v2"
)

// runCreateUser runs `admin create-user` against an isolated, temporary
// Opengist home so the command's own initialize() connects to a throwaway
// SQLite database. It returns whatever error the command produced.
func runCreateUser(t *testing.T, args ...string) error {
	t.Helper()

	home := t.TempDir()
	// InitLog (called by initialize) creates a "log" subdir under home.
	require.NoError(t, os.MkdirAll(filepath.Join(home, "log"), 0755))
	t.Setenv("OG_OPENGIST_HOME", home)

	t.Cleanup(func() {
		_ = db.Close()
	})

	app := &cli.App{
		Name: "opengist",
		Commands: []*cli.Command{
			{Name: "start", Action: func(*cli.Context) error { return nil }},
			{Name: "admin", Subcommands: []*cli.Command{&CmdAdminCreateUser}},
		},
		DefaultCommand: "start",
	}
	return app.Run(append([]string{"opengist", "admin", "create-user"}, args...))
}

func TestCmdAdminCreateUser_BasicAdmin(t *testing.T) {
	require.NoError(t, runCreateUser(t, "--username", "admin", "--password", "s3cret!", "--admin", "--email", "admin@example.com"))

	user, err := db.GetUserByUsername("admin")
	require.NoError(t, err)
	require.True(t, user.IsAdmin, "user should be an admin")
	require.Equal(t, "admin@example.com", user.Email)
	require.NotEmpty(t, user.Password)

	match, err := password.VerifyPassword("s3cret!", user.Password)
	require.NoError(t, err)
	require.True(t, match, "password should verify")

	// Gravatar hash must match the lowercased email, like the web flows.
	require.Equal(t, fmt.Sprintf("%x", md5.Sum([]byte("admin@example.com"))), user.MD5Hash)
}

// TestCmdAdminCreateUser_AdminNamedAdmin is the critical regression case: a
// user literally named "admin" combined with the --admin flag used to trigger
// urfave/cli's default-command misrouting. With the all-flags design there is
// no positional argument, so this must create the user correctly.
func TestCmdAdminCreateUser_AdminNamedAdmin(t *testing.T) {
	require.NoError(t, runCreateUser(t, "--username", "admin", "--password", "pw", "--admin"))

	user, err := db.GetUserByUsername("admin")
	require.NoError(t, err)
	require.True(t, user.IsAdmin)

	match, err := password.VerifyPassword("pw", user.Password)
	require.NoError(t, err)
	require.True(t, match)
}

func TestCmdAdminCreateUser_NotAdminByDefault(t *testing.T) {
	require.NoError(t, runCreateUser(t, "--username", "bob", "-p", "password123"))

	user, err := db.GetUserByUsername("bob")
	require.NoError(t, err)
	require.False(t, user.IsAdmin)
	require.Empty(t, user.Email)
	require.Empty(t, user.MD5Hash, "no email => no gravatar hash, matching registration")
}

func TestCmdAdminCreateUser_FlagsInAnyOrder(t *testing.T) {
	// Flags may appear in any order; there is no positional argument.
	require.NoError(t, runCreateUser(t, "--admin", "--password", "pw", "--username", "zoe"))

	user, err := db.GetUserByUsername("zoe")
	require.NoError(t, err)
	require.True(t, user.IsAdmin)
}

func TestCmdAdminCreateUser_ReRunIsNoOp(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(home, "log"), 0755))
	t.Setenv("OG_OPENGIST_HOME", home)
	t.Cleanup(func() { _ = db.Close() })

	run := func() error {
		app := &cli.App{
			Name: "opengist",
			Commands: []*cli.Command{
				{Name: "start", Action: func(*cli.Context) error { return nil }},
				{Name: "admin", Subcommands: []*cli.Command{&CmdAdminCreateUser}},
			},
			DefaultCommand: "start",
		}
		return app.Run([]string{"opengist", "admin", "create-user", "--username", "seed", "--password", "pw", "--admin"})
	}

	require.NoError(t, run(), "first run should create the user")
	require.NoError(t, run(), "second run should be a no-op, not an error")

	count, err := db.CountAll(&db.User{})
	require.NoError(t, err)
	require.Equal(t, int64(1), count, "idempotent run must not duplicate the user")
}

func TestCmdAdminCreateUser_MissingUsername(t *testing.T) {
	err := runCreateUser(t, "--password", "pw")
	require.Error(t, err)
}

func TestCmdAdminCreateUser_MissingPassword(t *testing.T) {
	err := runCreateUser(t, "--username", "nopass")
	require.Error(t, err)
}

func TestCmdAdminCreateUser_InvalidUsername(t *testing.T) {
	// Reserved names must be rejected with the same rules as the web form.
	err := runCreateUser(t, "--username", "login", "--password", "pw")
	require.Error(t, err)

	_, fetchErr := db.GetUserByUsername("login")
	require.Error(t, fetchErr, "reserved user must not be persisted")
}

// TestCmdAdminCreateUser_PositionalArgGivesGuidance checks that a user who
// tries the positional style (as the sibling commands use) gets a helpful
// pointer to --username rather than a confusing failure.
func TestCmdAdminCreateUser_PositionalArgGivesGuidance(t *testing.T) {
	err := runCreateUser(t, "alice", "--password", "pw")
	require.Error(t, err)
	require.Contains(t, err.Error(), "--username")
}

func TestCmdAdminCreateUser_PasswordStdin(t *testing.T) {
	// Feed the password through stdin to keep it out of the process list.
	r, w, err := os.Pipe()
	require.NoError(t, err)
	_, err = w.Write([]byte("stdin-secret\n"))
	require.NoError(t, err)
	require.NoError(t, w.Close())

	oldStdin := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = oldStdin })

	require.NoError(t, runCreateUser(t, "--username", "pipe", "--password-stdin"))

	user, err := db.GetUserByUsername("pipe")
	require.NoError(t, err)
	match, err := password.VerifyPassword("stdin-secret", user.Password)
	require.NoError(t, err)
	require.True(t, match)
}

func TestCmdAdminCreateUser_PasswordStdinAndFlagMutuallyExclusive(t *testing.T) {
	err := runCreateUser(t, "--username", "x", "--password", "pw", "--password-stdin")
	require.Error(t, err)
}

func TestGravatarHash(t *testing.T) {
	require.Empty(t, gravatarHash(""))
	require.Equal(t, fmt.Sprintf("%x", md5.Sum([]byte("User@Example.com"))), gravatarHash("User@Example.com"))
	// A 32-char lowercase hex digest.
	require.Regexp(t, `^[0-9a-f]{32}$`, gravatarHash("a@b.c"))
}
