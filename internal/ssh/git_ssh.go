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
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"
)

func runGitCommand(ch ssh.Channel, gitCmd string, key string, ip string) error {
	verb, args := parseCommand(gitCmd)
	if !strings.HasPrefix(verb, "git-") {
		verb = ""
	}
	verb = strings.TrimPrefix(verb, "git-")

	if verb != "upload-pack" && verb != "receive-pack" {
		return errors.New("invalid command")
	}

	repoFullName := strings.ToLower(strings.Trim(args, "'"))
	repoFields := strings.SplitN(repoFullName, "/", 2)
	if len(repoFields) != 2 {
		return errors.New("invalid gist path")
	}

	userName := strings.ToLower(repoFields[0])
	gistName := strings.TrimSuffix(strings.ToLower(repoFields[1]), ".git")

	gist, err := db.GetGist(userName, gistName)
	if err != nil {
		return errors.New("gist not found")
	}

	allowUnauthenticated, err := auth.ShouldAllowUnauthenticatedGistAccess(db.AuthInfo{}, true)
	if err != nil {
		return errors.New("internal server error")
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
				return errors.New("gist not found")
			}
			errorSsh("Failed to get user by SSH key id", err)
			return errors.New("internal server error")
		}
		_ = db.SSHKeyLastUsedNow(pubKey.Content)
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
		_, _ = io.Copy(stdin, ch)
	}()
	_, _ = io.Copy(ch, stdout)
	_, _ = io.Copy(ch, stderr)

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
