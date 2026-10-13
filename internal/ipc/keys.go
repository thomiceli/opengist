package ipc

// SSHKeyLookupRequest asks the daemon to resolve a public key (in authorized
// keys form, "<type> <base64>") to its stored SSH key. Used by the `keys`
// command, which sshd runs as its AuthorizedKeysCommand on every offered key —
// the hot, pre-auth path — so it must not open the database itself.
type SSHKeyLookupRequest struct {
	Key string `json:"key"`
}

// SSHKeyLookupResponse reports whether the key is known and, if so, its id (to
// embed in the forced command).
type SSHKeyLookupResponse struct {
	Found bool `json:"found"`
	KeyID uint `json:"key_id"`
}

// LookupSSHKey resolves a public key against the daemon's database.
func LookupSSHKey(key string) (*SSHKeyLookupResponse, error) {
	resp := &SSHKeyLookupResponse{}
	if err := post("/api/ipc/ssh/keys", &SSHKeyLookupRequest{Key: key}, resp); err != nil {
		return nil, err
	}
	return resp, nil
}
