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
// Message is shown to the connecting user. Created reports that the gist was
// created for this push, in which case the shim calls DiscardEmptyGist once git
// exits.
type SSHCommandResponse struct {
	Authorized bool   `json:"authorized"`
	Message    string `json:"message"`
	Verb       string `json:"verb"`
	RepoPath   string `json:"repo_path"`
	GistID     string `json:"gist_id"`
	Created    bool   `json:"created"`
}

// AuthorizeSSHCommand authorizes an SSH git command against the daemon.
func AuthorizeSSHCommand(req *SSHCommandRequest) (*SSHCommandResponse, error) {
	resp := &SSHCommandResponse{}
	if err := post("/api/ipc/ssh/command", req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// DiscardEmptyGistRequest identifies a gist created for a push over SSH (see
// SSHCommandResponse.Created) whose push is over.
type DiscardEmptyGistRequest struct {
	GistID string `json:"gist_id"`
}

// DiscardEmptyGist asks the daemon to delete a gist created for a push if the
// push stored nothing in it, so no empty gist is left behind.
func DiscardEmptyGist(gistID string) error {
	return post("/api/ipc/ssh/discard-empty-gist", &DiscardEmptyGistRequest{GistID: gistID}, nil)
}
