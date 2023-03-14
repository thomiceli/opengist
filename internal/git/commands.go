package git

import (
	"io"
	"opengist/internal/config"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
)

func GetRepositoryPath(user string, gist string) (string, error) {
	return filepath.Join(config.GetHomeDir(), "repos", strings.ToLower(user), gist), nil
}

func getTmpRepositoryPath(gistId string) (string, error) {
	dirname, err := getTmpRepositoriesPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(dirname, gistId), nil
}

func getTmpRepositoriesPath() (string, error) {
	return filepath.Join(config.GetHomeDir(), "tmp", "repos"), nil
}

func InitRepository(user string, gist string) error {
	repositoryPath, err := GetRepositoryPath(user, gist)

	if err != nil {
		return err
	}

	cmd := exec.Command(
		"git",
		"init",
		"--bare",
		repositoryPath,
	)

	_, err = cmd.Output()
	if err != nil {
		return err
	}

	f1, err := os.OpenFile(filepath.Join(repositoryPath, "git-daemon-export-ok"), os.O_RDONLY|os.O_CREATE, 0644)
	defer f1.Close()

	preReceiveDst, err := os.OpenFile(filepath.Join(repositoryPath, "hooks", "pre-receive"), os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0744)
	if err != nil {
		return err
	}

	preReceiveSrc, err := os.OpenFile(filepath.Join("internal", "resources", "pre-receive"), os.O_RDONLY, os.ModeAppend)
	if err != nil {
		return err
	}
	_, err = io.Copy(preReceiveDst, preReceiveSrc)
	if err != nil {
		return err
	}

	defer preReceiveDst.Close()
	defer preReceiveSrc.Close()

	return err
}

func GetNumberOfCommitsOfRepository(user string, gist string) (string, error) {
	repositoryPath, err := GetRepositoryPath(user, gist)
	if err != nil {
		return "", err
	}

	cmd := exec.Command(
		"git",
		"rev-list",
		"--all",
		"--count",
	)
	cmd.Dir = repositoryPath

	stdout, err := cmd.Output()
	return strings.TrimSuffix(string(stdout), "\n"), err
}

func GetFilesOfRepository(user string, gist string, commit string) ([]string, error) {
	repositoryPath, err := GetRepositoryPath(user, gist)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(
		"git",
		"ls-tree",
		commit,
		"--name-only",
	)
	cmd.Dir = repositoryPath

	stdout, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	slice := strings.Split(string(stdout), "\n")
	return slice[:len(slice)-1], nil
}

func GetFileContent(user string, gist string, commit string, filename string) (string, error) {
	repositoryPath, err := GetRepositoryPath(user, gist)
	if err != nil {
		return "", err
	}

	cmd := exec.Command(
		"git",
		"--no-pager",
		"show",
		commit+":"+filename,
	)
	cmd.Dir = repositoryPath

	stdout, err := cmd.Output()
	return string(stdout), err
}

func GetLog(user string, gist string, skip string) (string, error) {
	repositoryPath, err := GetRepositoryPath(user, gist)
	if err != nil {
		return "", err
	}

	cmd := exec.Command(
		"git",
		"--no-pager",
		"log",
		"-n",
		"11",
		"--no-prefix",
		"--no-color",
		"-p",
		"--skip",
		skip,
		"--format=format:%n=commit %H:%aN:%at",
		"--shortstat",
		"--ignore-missing", // avoid errors if a wrong hash is given
		"HEAD",
	)
	cmd.Dir = repositoryPath

	stdout, err := cmd.Output()
	return string(stdout), err
}

func CloneTmp(user string, gist string, gistTmpId string) error {
	repositoryPath, err := GetRepositoryPath(user, gist)
	if err != nil {
		return err
	}

	tmpPath, err := getTmpRepositoriesPath()
	if err != nil {
		return err
	}

	tmpRepositoryPath := path.Join(tmpPath, gistTmpId)

	err = os.RemoveAll(tmpRepositoryPath)
	if err != nil {
		return err
	}

	cmd := exec.Command("git", "clone", repositoryPath, gistTmpId)
	cmd.Dir = tmpPath
	if err = cmd.Run(); err != nil {
		return err
	}

	cmd = exec.Command("git", "config", "user.name", user)
	cmd.Dir = tmpRepositoryPath
	if err = cmd.Run(); err != nil {
		return err
	}

	// remove every file (and not the .git directory!)
	cmd = exec.Command("find", ".", "-maxdepth", "1", "-type", "f", "-delete")
	cmd.Dir = tmpRepositoryPath
	return cmd.Run()
}

func SetFileContent(gistTmpId string, filename string, content string) error {
	repositoryPath, err := getTmpRepositoryPath(gistTmpId)
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(repositoryPath, filename), []byte(content), 0644)
}

func AddAll(gistTmpId string) error {
	tmpPath, err := getTmpRepositoryPath(gistTmpId)
	if err != nil {
		return err
	}

	// in case of a change where only a file name has its case changed
	cmd := exec.Command("git", "rm", "-r", "--cached", "--ignore-unmatch", ".")
	cmd.Dir = tmpPath
	err = cmd.Run()
	if err != nil {
		return err
	}

	cmd = exec.Command("git", "add", "-A")
	cmd.Dir = tmpPath

	return cmd.Run()
}

func Commit(gistTmpId string) error {
	cmd := exec.Command("git", "commit", "--allow-empty", "-m", `"Opengist commit"`)
	tmpPath, err := getTmpRepositoryPath(gistTmpId)
	if err != nil {
		return err
	}
	cmd.Dir = tmpPath

	return cmd.Run()
}

func Push(gistTmpId string) error {
	tmpRepositoryPath, err := getTmpRepositoryPath(gistTmpId)
	if err != nil {
		return err
	}
	cmd := exec.Command(
		"git",
		"push",
	)
	cmd.Dir = tmpRepositoryPath

	err = cmd.Run()
	if err != nil {
		return err
	}

	return os.RemoveAll(tmpRepositoryPath)
}

func DeleteRepository(user string, gist string) error {
	return os.RemoveAll(filepath.Join(config.GetHomeDir(), "repos", strings.ToLower(user), gist))
}

func UpdateServerInfo(user string, gist string) error {
	repositoryPath, err := GetRepositoryPath(user, gist)
	if err != nil {
		return err
	}

	cmd := exec.Command("git", "update-server-info")
	cmd.Dir = repositoryPath
	return cmd.Run()
}

func RPCRefs(user string, gist string, service string) ([]byte, error) {
	repositoryPath, err := GetRepositoryPath(user, gist)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command("git", service, "--stateless-rpc", "--advertise-refs", ".")
	cmd.Dir = repositoryPath
	stdout, err := cmd.Output()
	return stdout, err
}

func GetGitVersion() (string, error) {
	cmd := exec.Command("git", "--version")
	stdout, err := cmd.Output()
	if err != nil {
		return "", err
	}

	versionFields := strings.Fields(string(stdout))
	if len(versionFields) < 3 {
		return string(stdout), nil
	}

	return versionFields[2], nil
}
