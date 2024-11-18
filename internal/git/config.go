package git

import "os/exec"

func InitGitConfig() error {
	configs := map[string]string{
		"receive.advertisePushOptions": "true",
		"safe.directory":               "*",
	}

	for key, value := range configs {
		if err := setGitConfig(key, value); err != nil {
			return err
		}
	}

	return nil
}

func setGitConfig(key, value string) error {
	cmd := exec.Command("git", "config", "--global", key, value)
	return cmd.Run()
}
