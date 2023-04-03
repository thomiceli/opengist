package ssh

import (
	"errors"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"
	"io"
	"opengist/internal/git"
	"opengist/internal/models"
	"os/exec"
	"strings"
)

func runGitCommand(ch ssh.Channel, gitCmd string, keyID uint, ip string) error {
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

	gist, err := models.GetGist(userName, gistName)
	if err != nil {
		return errors.New("gist not found")
	}

	if verb == "receive-pack" {
		user, err := models.GetUserBySSHKeyID(keyID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				log.Warn().Msg("Invalid SSH authentication attempt from " + ip)
				return errors.New("unauthorized")
			}
			errorSsh("Failed to get user by SSH key id", err)
			return errors.New("internal server error")
		}

		if user.ID != gist.UserID {
			log.Warn().Msg("Invalid SSH authentication attempt from " + ip)
			return errors.New("unauthorized")
		}
	}

	_ = models.SSHKeyLastUsedNow(keyID)

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
		_ = gist.UpdatePreviewAndCount()
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
