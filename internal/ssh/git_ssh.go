package ssh

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/thomiceli/opengist/internal/auth"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/git"
	"github.com/thomiceli/opengist/internal/validator"
	"gorm.io/gorm"
)

// AuthorizeGitCommand validates a single git pack command (upload-pack /
// receive-pack) and checks that the given SSH key may run it against the
// referenced gist, returning the gist and the bare verb. It is shared by the
// embedded SSH server (which has the key string) and the IPC handler backing the
// OpenSSH shim (which resolves the key id to its string).
//
// As over HTTP, a push can create the gist it targets: pushing to "init"
// creates a gist with a random identifier, and pushing to a gist of the key's
// user that does not exist yet creates it under that URL. The returned bool
// reports whether the gist was created for this push; the caller must then pass
// it to DiscardEmptyGist once git exits.
func AuthorizeGitCommand(gitCmd string, key string, ip string) (*db.Gist, string, bool, error) {
	verb, args := parseCommand(gitCmd)
	if !strings.HasPrefix(verb, "git-") {
		verb = ""
	}
	verb = strings.TrimPrefix(verb, "git-")

	if verb != "upload-pack" && verb != "receive-pack" {
		return nil, "", false, errors.New("invalid command")
	}

	repoPath := strings.Trim(args, "'")

	if verb == "receive-pack" && strings.TrimSuffix(strings.ToLower(repoPath), ".git") == "init" {
		gist, err := createGist(key, ip, "", "")
		if err != nil {
			return nil, "", false, err
		}
		return gist, verb, true, nil
	}

	repoFields := strings.SplitN(repoPath, "/", 2)
	if len(repoFields) != 2 {
		return nil, "", false, errors.New("invalid gist path")
	}

	userName := strings.ToLower(repoFields[0])
	gistName := strings.TrimSuffix(strings.ToLower(repoFields[1]), ".git")

	gist, err := db.GetGist(userName, gistName)
	if verb == "receive-pack" && errors.Is(err, gorm.ErrRecordNotFound) {
		gist, err = createGist(key, ip, userName, strings.TrimSuffix(repoFields[1], ".git"))
		if err != nil {
			return nil, "", false, err
		}
		return gist, verb, true, nil
	}
	if err != nil {
		return nil, "", false, errors.New("gist not found")
	}

	allowUnauthenticated, err := auth.ShouldAllowUnauthenticatedGistAccess(db.AuthInfo{}, true)
	if err != nil {
		errorSsh("Failed to get auth info", err)
		return nil, "", false, errors.New("internal server error")
	}

	// Check for the key if :
	// - user wants to push the gist
	// - user wants to clone a private gist
	// - gist is not found (obfuscation)
	// - admin setting to require login is set to true
	if verb == "receive-pack" ||
		gist.Private == db.PrivateVisibility ||
		gist.ID == 0 ||
		!allowUnauthenticated {

		var userToCheckPermissions *db.User
		if gist.Private != db.PrivateVisibility && verb == "upload-pack" {
			userToCheckPermissions, _ = db.GetUserFromSSHKey(key)
		} else {
			userToCheckPermissions = &gist.User
		}

		pubKey, err := db.SSHKeyExistsForUser(key, userToCheckPermissions.ID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				log.Warn().Msg("Invalid SSH authentication attempt from " + ip)
				return nil, "", false, errors.New("gist not found")
			}
			errorSsh("Failed to get user by SSH key id", err)
			return nil, "", false, errors.New("internal server error")
		}
		_ = db.SSHKeyLastUsedNow(pubKey.Content)
	}

	// Refuse pushes to an archived gist only after the key has been validated
	// against the owner above, so we don't disclose the gist's existence.
	if verb == "receive-pack" && gist.Archived {
		return nil, "", false, errors.New("this gist is archived and is read-only")
	}

	return gist, verb, false, nil
}

// createGist creates the gist a push is about to fill, owned by the user of the
// SSH key. A push to "init" (empty userName) gets a random identifier; a push
// to <userName>/<url> may only target the key user's own namespace, and url
// becomes the gist URL.
func createGist(key, ip, userName, url string) (*db.Gist, error) {
	user, err := db.GetUserFromSSHKey(key)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Warn().Msg("Invalid SSH authentication attempt from " + ip)
			return nil, errors.New("gist not found")
		}
		errorSsh("Failed to get user by SSH key", err)
		return nil, errors.New("internal server error")
	}

	if userName != "" {
		// Answer as for any missing gist, so as not to disclose anything about
		// another user's gists.
		if !strings.EqualFold(user.Username, userName) {
			return nil, errors.New("gist not found")
		}
		if validator.NewValidator().Var(url, "max=32,alphanumdash") != nil {
			return nil, errors.New("invalid gist URL: only alphanumeric characters and dashes are allowed, up to 32 characters")
		}
	}

	gist, err := db.CreateGistForPush(user, url)
	if err != nil {
		errorSsh("Failed to create gist", err)
		return nil, errors.New("internal server error")
	}
	_ = db.SSHKeyLastUsedNow(key)

	return gist, nil
}

// DiscardEmptyGist deletes a gist created for a push (see AuthorizeGitCommand)
// if the push stored nothing in it: the client had nothing to push, a hook
// rejected it, or the connection dropped. It is called once git exits; errors
// are logged, as there is nothing more the caller can do about them.
func DiscardEmptyGist(gist *db.Gist) {
	empty, err := git.HasNoCommits(gist.User.Username, gist.Uuid)
	if err != nil {
		errorSsh("Failed to check if the gist has commits", err)
		return
	}
	if !empty {
		return
	}
	if err = gist.Delete(); err != nil {
		errorSsh("Failed to delete empty gist", err)
	}
}

// RunGitCommand authorizes and runs a single git pack command (upload-pack /
// receive-pack) for the gist referenced by gitCmd, on behalf of the given SSH
// key. It is transport-agnostic: the embedded SSH server passes its channel as
// in/out/errOut. (The OpenSSH shim authorizes over the IPC API and runs git
// itself, so it does not use this.)
func RunGitCommand(in io.Reader, out, errOut io.Writer, gitCmd string, key string, ip string) error {
	gist, verb, created, err := AuthorizeGitCommand(gitCmd, key, ip)
	if err != nil {
		return err
	}
	if created {
		defer DiscardEmptyGist(gist)
	}

	repositoryPath := git.RepositoryPath(gist.User.Username, gist.Uuid)

	cmd := exec.Command("git", verb, repositoryPath)
	cmd.Dir = repositoryPath
	// As on the other push paths, the post-receive hook updates the gist over
	// the IPC API: preview, search index, push options, new gist message.
	cmd.Env = append(os.Environ(),
		"OPENGIST_REPOSITORY_ID="+strconv.FormatUint(uint64(gist.ID), 10),
		"OPENGIST_PROTOCOL_INTERNAL=ssh",
	)

	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err = cmd.Start(); err != nil {
		errorSsh("Failed to start git command", err)
		return errors.New("internal server error")
	}

	// avoid blocking
	go func() {
		_, _ = io.Copy(stdin, in)
	}()
	_, _ = io.Copy(out, stdout)
	_, _ = io.Copy(errOut, stderr)

	err = cmd.Wait()
	if err != nil {
		errorSsh("Failed to wait for git command", err)
		return errors.New("internal server error")
	}

	return nil
}

func parseCommand(cmd string) (string, string) {
	split := strings.SplitN(cmd, " ", 2)

	if len(split) != 2 {
		return "", ""
	}

	return split[0], strings.Replace(split[1], "'/", "'", 1)
}
