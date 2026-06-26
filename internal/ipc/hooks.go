package ipc

// HookRefUpdate is one line of a Git hook's stdin: the old/new revision of a
// ref being updated.
type HookRefUpdate struct {
	OldRev  string `json:"old_rev"`
	NewRev  string `json:"new_rev"`
	RefName string `json:"ref_name"`
}

// HookPreReceiveRequest is sent by the pre-receive hook subprocess to the
// daemon. The subprocess computes ChangedFiles itself — one raw `git log`
// output per updated ref — because the pushed objects are only visible to the
// hook's environment (push quarantine) until pre-receive succeeds; the daemon
// only applies the policy.
type HookPreReceiveRequest struct {
	ChangedFiles []string `json:"changed_files"`
}

// HookPreReceiveResponse carries the daemon's decision and, when rejected, the
// message to show the user pushing.
type HookPreReceiveResponse struct {
	Allowed bool   `json:"allowed"`
	Message string `json:"message"`
}

// HookPreReceive asks the daemon whether a push may proceed.
func HookPreReceive(req *HookPreReceiveRequest) (*HookPreReceiveResponse, error) {
	resp := &HookPreReceiveResponse{}
	if err := post("/api/ipc/hook/pre-receive", req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// HookPostReceiveRequest is sent by the post-receive hook subprocess to the
// daemon.
type HookPostReceiveRequest struct {
	GistID      string            `json:"gist_id"`
	GistURL     string            `json:"gist_url"`
	References  []HookRefUpdate   `json:"references"`
	PushOptions map[string]string `json:"push_options"`
}

// HookPostReceiveResponse carries the text the daemon wants shown to the user
// pushing.
type HookPostReceiveResponse struct {
	Output string `json:"output"`
}

// HookPostReceive forwards a post-receive event to the daemon, which performs
// the database and index work and returns any message to show the user.
func HookPostReceive(req *HookPostReceiveRequest) (*HookPostReceiveResponse, error) {
	resp := &HookPostReceiveResponse{}
	if err := post("/api/ipc/hook/post-receive", req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}
