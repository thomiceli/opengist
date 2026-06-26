package ipc

// SSHCommandRequest asks the daemon to authorize a git command for a connecting
// SSH key (identified by the id embedded in the forced command) and report what
// to run. Used by the `shell` command — the forced command sshd runs after a key
// matches.
type SSHCommandRequest struct {
	KeyID   uint   `json:"key_id"`
	Command string `json:"command"`
	IP      string `json:"ip"`
}

// SSHCommandResponse carries the daemon's decision. When authorized, it tells
// the shim which git pack command to run and against which repository; when not,
// Message is shown to the connecting user.
type SSHCommandResponse struct {
	Authorized bool   `json:"authorized"`
	Message    string `json:"message"`
	Verb       string `json:"verb"`
	RepoPath   string `json:"repo_path"`
	GistID     string `json:"gist_id"`
}

// AuthorizeSSHCommand authorizes an SSH git command against the daemon.
func AuthorizeSSHCommand(req *SSHCommandRequest) (*SSHCommandResponse, error) {
	resp := &SSHCommandResponse{}
	if err := post("/api/ipc/ssh/command", req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}
