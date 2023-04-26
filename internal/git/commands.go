package git

import (
	"fmt"
	"opengist/internal/config"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

func RepositoryPath(user string, gist string) string {
	return filepath.Join(config.GetHomeDir(), "repos", strings.ToLower(user), gist)
}

func TmpRepositoryPath(gistId string) string {
	dirname := TmpRepositoriesPath()
	return filepath.Join(dirname, gistId)
}

func TmpRepositoriesPath() string {
	return filepath.Join(config.GetHomeDir(), "tmp", "repos")
}

func InitRepository(user string, gist string) error {
	repositoryPath := RepositoryPath(user, gist)

	cmd := exec.Command(
		"git",
		"init",
		"--bare",
		repositoryPath,
	)

	err := cmd.Run()
	if err != nil {
		return err
	}

	return copyFiles(repositoryPath)
}

func GetNumberOfCommitsOfRepository(user string, gist string) (string, error) {
	repositoryPath := RepositoryPath(user, gist)

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

func GetFilesOfRepository(user string, gist string, revision string) ([]string, error) {
	repositoryPath := RepositoryPath(user, gist)

	cmd := exec.Command(
		"git",
		"ls-tree",
		"--name-only",
		"--",
		revision,
	)
	cmd.Dir = repositoryPath

	stdout, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	slice := strings.Split(string(stdout), "\n")
	return slice[:len(slice)-1], nil
}

func GetFileContent(user string, gist string, revision string, filename string, truncate bool) (string, bool, error) {
	repositoryPath := RepositoryPath(user, gist)

	var maxBytes int64 = -1
	if truncate {
		maxBytes = 2 << 18
	}

	cmd := exec.Command(
		"git",
		"--no-pager",
		"show",
		revision+":"+filename,
	)
	cmd.Dir = repositoryPath

	stdout, _ := cmd.StdoutPipe()
	err := cmd.Start()
	if err != nil {
		return "", false, err
	}
	defer cmd.Wait()

	return truncateCommandOutput(stdout, maxBytes)
}

func GetLog(user string, gist string, skip int) ([]*Commit, error) {
	repositoryPath := RepositoryPath(user, gist)

	cmd := exec.Command(
		"git",
		"--no-pager",
		"log",
		"-n",
		"11",
		"--no-color",
		"-p",
		"--skip",
		strconv.Itoa(skip),
		"--format=format:c %H%na %aN%nm %ae%nt %at",
		"--shortstat",
		"HEAD",
	)
	cmd.Dir = repositoryPath
	stdout, _ := cmd.StdoutPipe()
	err := cmd.Start()
	if err != nil {
		return nil, err
	}
	defer cmd.Wait()

	return parseLog(stdout, 2<<18), nil
}

func CloneTmp(user string, gist string, gistTmpId string, email string) error {
	repositoryPath := RepositoryPath(user, gist)

	tmpPath := TmpRepositoriesPath()

	tmpRepositoryPath := path.Join(tmpPath, gistTmpId)

	err := os.RemoveAll(tmpRepositoryPath)
	if err != nil {
		return err
	}

	cmd := exec.Command("git", "clone", repositoryPath, gistTmpId)
	cmd.Dir = tmpPath
	if err = cmd.Run(); err != nil {
		return err
	}

	// remove every file (and not the .git directory!)
	cmd = exec.Command("find", ".", "-maxdepth", "1", "-type", "f", "-delete")
	cmd.Dir = tmpRepositoryPath
	if err = cmd.Run(); err != nil {
		return err
	}

	cmd = exec.Command("git", "config", "--local", "user.name", user)
	cmd.Dir = tmpRepositoryPath
	if err = cmd.Run(); err != nil {
		return err
	}

	cmd = exec.Command("git", "config", "--local", "user.email", email)
	cmd.Dir = tmpRepositoryPath
	return cmd.Run()
}

func ForkClone(userSrc string, gistSrc string, userDst string, gistDst string) error {
	repositoryPathSrc := RepositoryPath(userSrc, gistSrc)
	repositoryPathDst := RepositoryPath(userDst, gistDst)

	cmd := exec.Command("git", "clone", "--bare", repositoryPathSrc, repositoryPathDst)
	if err := cmd.Run(); err != nil {
		return err
	}

	return copyFiles(repositoryPathDst)
}

func SetFileContent(gistTmpId string, filename string, content string) error {
	repositoryPath := TmpRepositoryPath(gistTmpId)

	return os.WriteFile(filepath.Join(repositoryPath, filename), []byte(content), 0644)
}

func AddAll(gistTmpId string) error {
	tmpPath := TmpRepositoryPath(gistTmpId)

	// in case of a change where only a file name has its case changed
	cmd := exec.Command("git", "rm", "-r", "--cached", "--ignore-unmatch", ".")
	cmd.Dir = tmpPath
	err := cmd.Run()
	if err != nil {
		return err
	}

	cmd = exec.Command("git", "add", "-A")
	cmd.Dir = tmpPath

	return cmd.Run()
}

func CommitRepository(gistTmpId string, authorName string, authorEmail string) error {
	cmd := exec.Command("git",
		"commit",
		"--allow-empty",
		"-m",
		"Opengist commit",
		"--author",
		fmt.Sprintf("%s <%s>", authorName, authorEmail),
	)
	tmpPath := TmpRepositoryPath(gistTmpId)
	cmd.Dir = tmpPath

	return cmd.Run()
}

func Push(gistTmpId string) error {
	tmpRepositoryPath := TmpRepositoryPath(gistTmpId)
	cmd := exec.Command(
		"git",
		"push",
	)
	cmd.Dir = tmpRepositoryPath

	err := cmd.Run()
	if err != nil {
		return err
	}

	return os.RemoveAll(tmpRepositoryPath)
}

func DeleteRepository(user string, gist string) error {
	return os.RemoveAll(RepositoryPath(user, gist))
}

func UpdateServerInfo(user string, gist string) error {
	repositoryPath := RepositoryPath(user, gist)

	cmd := exec.Command("git", "update-server-info")
	cmd.Dir = repositoryPath
	return cmd.Run()
}

func RPC(user string, gist string, service string) ([]byte, error) {
	repositoryPath := RepositoryPath(user, gist)

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

func copyFiles(repositoryPath string) error {
	f1, err := os.OpenFile(filepath.Join(repositoryPath, "git-daemon-export-ok"), os.O_RDONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer f1.Close()

	preReceiveDst, err := os.OpenFile(filepath.Join(repositoryPath, "hooks", "pre-receive"), os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0744)
	if err != nil {
		return err
	}

	if _, err = preReceiveDst.WriteString(preReceive); err != nil {
		return err
	}
	defer preReceiveDst.Close()

	return nil
}

const preReceive = `#!/bin/sh

disallowed_files=""

while read -r old_rev new_rev ref
do
  while IFS= read -r file
  do
    case $file in
      */*)
        disallowed_files="${disallowed_files}${file} "
        ;;
    esac
  done <<EOF
$(git diff --name-only "$old_rev" "$new_rev")
EOF
done

if [ -n "$disallowed_files" ]; then
  echo "Pushing files in folders is not allowed:"
  for file in $disallowed_files; do
    echo "  $file"
  done
  exit 1
fi
`
