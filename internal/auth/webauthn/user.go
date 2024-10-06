package webauthn

import (
	"encoding/binary"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/thomiceli/opengist/internal/db"
)

type user struct {
	*db.User
}

func (u *user) WebAuthnID() []byte {
	return uintToBytes(u.ID)
}

func (u *user) WebAuthnName() string {
	return u.Username
}

func (u *user) WebAuthnDisplayName() string {
	return u.Username
}

func (u *user) WebAuthnCredentials() []webauthn.Credential {
	dbCreds, err := db.GetAllWACredentialsForUser(u.ID)
	if err != nil {
		return nil
	}

	return dbCreds
}

func (u *user) Exclusions() []protocol.CredentialDescriptor {
	creds := u.WebAuthnCredentials()
	exclusions := make([]protocol.CredentialDescriptor, len(creds))
	for i, cred := range creds {
		exclusions[i] = cred.Descriptor()
	}

	return exclusions
}

func discoverUser(rawID []byte, _ []byte) (webauthn.User, error) {
	ogUser, err := db.GetUserByCredentialID(rawID)
	if err != nil {
		return nil, err
	}

	return &user{User: ogUser}, nil
}

func uintToBytes(n uint) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(n))
	return b
}
