package git

import (
	"errors"
	"os/exec"
	"regexp"
)

type configEntry struct {
	value string
	fn    func(string, string) error
}

func InitGitConfig() error {
	configs := map[string]configEntry{
		"receive.advertisePushOptions": {value: "true", fn: setGitConfig},
		"safe.directory":               {value: "*", fn: addGitConfig},
	}

	for key, entry := range configs {
		if err := entry.fn(key, entry.value); err != nil {
			return err
		}
	}

	return nil
}

func setGitConfig(key, value string) error {
	_, err := getGitConfig(key, value)
	if err != nil && !checkErrorCode(err, 1) {
		return err
	}

	cmd := exec.Command("git", "config", "--global", key, value)
	return cmd.Run()
}

func addGitConfig(key, value string) error {
	_, err := getGitConfig(key, regexp.QuoteMeta(value))
	if err == nil {
		return nil
	}
	if checkErrorCode(err, 1) {
		cmd := exec.Command("git", "config", "--global", "--add", key, value)
		return cmd.Run()
	}
	return err
}

func getGitConfig(key, value string) (string, error) {
	cmd := exec.Command("git", "config", "--global", "--get", key, value)
	out, err := cmd.Output()
	return string(out), err
}

func checkErrorCode(err error, code int) bool {
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		return exitError.ExitCode() == code
	}
	return false
}
