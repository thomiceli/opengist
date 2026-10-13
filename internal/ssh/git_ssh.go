package ssh

import (
	"errors"
	"io"
	"os/exec"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/thomiceli/opengist/internal/auth"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/git"
	"gorm.io/gorm"
)

// AuthorizeGitCommand validates a single git pack command (upload-pack /
// receive-pack) and checks that the given SSH key may run it against the
// referenced gist, returning the gist and the bare verb. It is shared by the
// embedded SSH server (which has the key string) and the IPC handler backing the
// OpenSSH shim (which resolves the key id to its string).
func AuthorizeGitCommand(gitCmd string, key string, ip string) (*db.Gist, string, error) {
	verb, args := parseCommand(gitCmd)
	if !strings.HasPrefix(verb, "git-") {
		verb = ""
	}
	verb = strings.TrimPrefix(verb, "git-")

	if verb != "upload-pack" && verb != "receive-pack" {
		return nil, "", errors.New("invalid command")
	}

	repoFullName := strings.ToLower(strings.Trim(args, "'"))
	repoFields := strings.SplitN(repoFullName, "/", 2)
	if len(repoFields) != 2 {
		return nil, "", errors.New("invalid gist path")
	}

	userName := strings.ToLower(repoFields[0])
	gistName := strings.TrimSuffix(strings.ToLower(repoFields[1]), ".git")

	gist, err := db.GetGist(userName, gistName)
	if err != nil {
		return nil, "", errors.New("gist not found")
	}

	allowUnauthenticated, err := auth.ShouldAllowUnauthenticatedGistAccess(db.AuthInfo{}, true)
	if err != nil {
		errorSsh("Failed to get auth info", err)
		return nil, "", errors.New("internal server error")
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
				return nil, "", errors.New("gist not found")
			}
			errorSsh("Failed to get user by SSH key id", err)
			return nil, "", errors.New("internal server error")
		}
		_ = db.SSHKeyLastUsedNow(pubKey.Content)
	}

	// Refuse pushes to an archived gist only after the key has been validated
	// against the owner above, so we don't disclose the gist's existence.
	if verb == "receive-pack" && gist.Archived {
		return nil, "", errors.New("this gist is archived and is read-only")
	}

	return gist, verb, nil
}

// RunGitCommand authorizes and runs a single git pack command (upload-pack /
// receive-pack) for the gist referenced by gitCmd, on behalf of the given SSH
// key. It is transport-agnostic: the embedded SSH server passes its channel as
// in/out/errOut. (The OpenSSH shim authorizes over the IPC API and runs git
// itself, so it does not use this.)
func RunGitCommand(in io.Reader, out, errOut io.Writer, gitCmd string, key string, ip string) error {
	gist, verb, err := AuthorizeGitCommand(gitCmd, key, ip)
	if err != nil {
		return err
	}

	repositoryPath := git.RepositoryPath(gist.User.Username, gist.Uuid)

	cmd := exec.Command("git", verb, repositoryPath)
	cmd.Dir = repositoryPath

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

	// updatedAt is updated only if serviceType is receive-pack
	if verb == "receive-pack" {
		_ = gist.SetLastActiveNow()
		_ = gist.UpdatePreviewAndCount(false)
		gist.AddInIndex()
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
