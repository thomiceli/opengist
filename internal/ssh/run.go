package ssh

import (
	"errors"
	"github.com/rs/zerolog/log"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/db"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

func Start() {
	if !config.C.SshGit {
		return
	}

	sshConfig := &ssh.ServerConfig{
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			strKey := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key)))
			exists, err := db.SSHKeyDoesExists(strKey)
			if !exists || err != nil {
				if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
					return nil, err
				}

				log.Warn().Msg("Invalid SSH authentication attempt from " + conn.RemoteAddr().String())
				return nil, errors.New("unknown public key")
			}
			return &ssh.Permissions{Extensions: map[string]string{"key": strKey}}, nil
		},
	}

	key, err := setupHostKey()
	if err != nil {
		log.Fatal().Err(err).Msg("SSH: Could not setup host key")
	}

	sshConfig.AddHostKey(key)
	go listen(sshConfig)
}

func listen(serverConfig *ssh.ServerConfig) {
	log.Info().Msg("Starting SSH server on ssh://" + config.C.SshHost + ":" + config.C.SshPort)
	listener, err := net.Listen("tcp", config.C.SshHost+":"+config.C.SshPort)
	if err != nil {
		log.Fatal().Err(err).Msg("SSH: Failed to start SSH server")
	}
	defer listener.Close()

	for {
		nConn, err := listener.Accept()
		if err != nil {
			errorSsh("Failed to accept incoming connection", err)
			continue
		}

		go func() {
			sConn, channels, reqs, err := ssh.NewServerConn(nConn, serverConfig)
			if err != nil {
				if err != io.EOF && !errors.Is(err, syscall.ECONNRESET) {
					errorSsh("Failed to handshake", err)
				}
				return
			}

			go ssh.DiscardRequests(reqs)
			go handleConnexion(channels, sConn.Permissions.Extensions["key"], sConn.RemoteAddr().String())
		}()
	}
}

func handleConnexion(channels <-chan ssh.NewChannel, key string, ip string) {
	for channel := range channels {
		if channel.ChannelType() != "session" {
			_ = channel.Reject(ssh.UnknownChannelType, "Unknown channel type")
			continue
		}

		ch, reqs, err := channel.Accept()
		if err != nil {
			errorSsh("Could not accept channel", err)
			continue
		}

		go func(in <-chan *ssh.Request) {
			defer func() {
				_ = ch.Close()
			}()
			for req := range in {
				switch req.Type {
				case "env":

				case "shell":
					_, _ = ch.Write([]byte("Successfully connected to Opengist SSH server.\r\n"))
					_, _ = ch.SendRequest("exit-status", false, []byte{0, 0, 0, 0})
					return
				case "exec":
					payloadCmd := string(req.Payload)
					i := strings.Index(payloadCmd, "git")
					if i != -1 {
						payloadCmd = payloadCmd[i:]
					}

					if err = runGitCommand(ch, payloadCmd, key, ip); err != nil {
						_, _ = ch.Stderr().Write([]byte("Opengist: " + err.Error() + "\r\n"))
					}
					_, _ = ch.SendRequest("exit-status", false, []byte{0, 0, 0, 0})
					return
				}
			}
		}(reqs)
	}
}

func setupHostKey() (ssh.Signer, error) {
	dir := filepath.Join(config.GetHomeDir(), "ssh")

	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	keyPath := filepath.Join(dir, "opengist-ed25519")
	if _, err := os.Stat(keyPath); err != nil && !os.IsExist(err) {
		cmd := exec.Command(config.C.SshKeygen,
			"-t", "ssh-ed25519",
			"-f", keyPath,
			"-m", "PEM",
			"-N", "")
		err = cmd.Run()
		if err != nil {
			return nil, err
		}
	}

	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}

	signer, err := ssh.ParsePrivateKey(keyData)
	if err != nil {
		return nil, err
	}

	return signer, nil
}

func errorSsh(message string, err error) {
	log.Error().Err(err).Msg("SSH: " + message)
}
