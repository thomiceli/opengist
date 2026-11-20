package git_test

import (
	"os"
	"os/exec"
	"testing"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/db"
	webtest "github.com/thomiceli/opengist/internal/web/test"
)

// gitCloneOutput is gitClone, returning what git prints. Git runs in the C
// locale so it sends no Accept-Language header and the server answers in en-US.
func gitCloneOutput(baseUrl, creds, user, gistId, destDir string) (string, error) {
	authUrl := "http://" + creds + "@" + baseUrl[len("http://"):]
	cmd := exec.Command("git", "clone", authUrl+"/"+user+"/"+gistId+".git", destDir)
	cmd.Env = append(os.Environ(), "LC_ALL=C", "LANGUAGE=")
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// TestGitAuthWithMFA checks that over Git HTTP a password alone is refused for
// users with MFA enabled, while access tokens keep working for them.
func TestGitAuthWithMFA(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	baseUrl := s.StartHttpServer(t)

	const mfaMsg = "Multi-factor authentication is enabled for this account. Use an access token instead of your password."

	tests := []struct {
		name      string
		username  string
		enableMFA func(t *testing.T, userID uint)
	}{
		{"NoMFA", "nomfa", nil},
		{"TOTP", "totp", func(t *testing.T, userID uint) {
			require.NoError(t, (&db.TOTP{UserID: userID}).Create())
		}},
		{"Passkey", "passkey", func(t *testing.T, userID uint) {
			_, err := db.CreateFromCrendential(userID, "key", &webauthn.Credential{ID: []byte("passkey-id"), PublicKey: []byte("public-key")})
			require.NoError(t, err)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s.Register(t, tt.username)
			_, gist, _, gistId := s.CreateGistAs(t, tt.username, "2")

			// Create the token first: once MFA is enabled, the web login needs
			// the second factor.
			s.Login(t, tt.username)
			token := s.CreateAccessToken(t, "git", db.ReadWritePermission, db.NoPermission)
			s.Logout()

			passwordWorks := tt.enableMFA == nil
			if !passwordWorks {
				tt.enableMFA(t, gist.UserID)
			}

			passwordCreds := tt.username + ":" + tt.username
			tokenCreds := tt.username + ":" + token
			gistUrl := func(creds string) string {
				return "http://" + creds + "@" + baseUrl[len("http://"):] + "/" + tt.username + "/" + gistId + ".git"
			}
			requirePasswordResult := func(err error) {
				t.Helper()
				if passwordWorks {
					require.NoError(t, err)
				} else {
					require.Error(t, err)
				}
			}

			// Clone with the password; when refused, git shows the reason.
			out, err := gitCloneOutput(baseUrl, passwordCreds, tt.username, gistId, t.TempDir())
			if passwordWorks {
				require.NoError(t, err, out)
			} else {
				require.Error(t, err)
				require.Contains(t, out, "remote: "+mfaMsg)

				infoRefs := "/" + tt.username + "/" + gistId + ".git/info/refs?service=git-upload-pack"
				code, body := dumbGet(t, baseUrl, infoRefs, passwordCreds)
				require.Equal(t, 401, code)
				require.Equal(t, mfaMsg, string(body))

				// A wrong password does not tell that MFA is enabled.
				code, body = dumbGet(t, baseUrl, infoRefs, tt.username+":wrong")
				require.Equal(t, 404, code)
				require.NotContains(t, string(body), mfaMsg)
			}

			// Push, push-to-create and /init with the password.
			dest := t.TempDir()
			require.NoError(t, gitClone(baseUrl, tokenCreds, tt.username, gistId, dest))
			require.NoError(t, exec.Command("git", "-C", dest, "remote", "set-url", "origin", gistUrl(passwordCreds)).Run())
			requirePasswordResult(gitPush(dest, "password.txt", "from password"))
			requirePasswordResult(gitInitAndPushTo(baseUrl, passwordCreds, "/"+tt.username+"/password-gist.git", "file.txt", "content", t.TempDir()))
			requirePasswordResult(gitInitAndPushTo(baseUrl, passwordCreds, "/init", "file.txt", "content", t.TempDir()))
			if !passwordWorks {
				count, err := db.CountAllGistsFromUser(gist.UserID, gist.UserID)
				require.NoError(t, err)
				require.EqualValues(t, 1, count, "no gist should be created with the password")
			}

			// The access token works with or without MFA.
			require.NoError(t, exec.Command("git", "-C", dest, "remote", "set-url", "origin", gistUrl(tokenCreds)).Run())
			require.NoError(t, gitPush(dest, "token.txt", "from token"))
			require.NoError(t, gitInitAndPushTo(baseUrl, tokenCreds, "/"+tt.username+"/token-gist.git", "file.txt", "content", t.TempDir()))
			require.NoError(t, gitInitAndPushTo(baseUrl, tokenCreds, "/init", "file.txt", "content", t.TempDir()))
		})
	}
}
