package ssh

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/config"
)

// initConfigForTest initializes the global config with sane defaults so the
// SSH code paths that read config.C do not dereference a nil pointer. Fields
// relevant to the test are overridden by the caller.
func initConfigForTest(t *testing.T) {
	t.Helper()
	require.NoError(t, config.InitConfig(""))
	config.C.OpengistHome = t.TempDir()
}

// TestStart_NoopWhenNotBuiltin ensures Start is a no-op (returns nil) when the
// embedded SSH server is not enabled, so callers' nil-guarded Stop is correct.
func TestStart_NoopWhenNotBuiltin(t *testing.T) {
	initConfigForTest(t)
	config.C.SshGit = config.SshServerDisabled

	require.Nil(t, Start())
}

// TestStartAndStop is an integration check that Start binds the configured
// address (a real client can connect) and that Stop tears the listener down
// again — the core behavior added by the graceful-shutdown change.
func TestStartAndStop(t *testing.T) {
	initConfigForTest(t)
	config.C.SshGit = config.SshServerBuiltin
	config.C.SshHost = "127.0.0.1"
	config.C.SshPort = "0" // ephemeral free port chosen by the kernel

	s := Start()
	require.NotNil(t, s)
	addr := s.listener.Addr().String()

	// The server is accepting TCP connections on the bound address.
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	require.NoError(t, err)
	require.NoError(t, conn.Close())

	// Stop must return and stop accepting new connections.
	done := make(chan struct{})
	go func() { s.Stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * sshDrainTimeout):
		t.Fatal("Stop did not return within the drain timeout")
	}

	_, err = net.DialTimeout("tcp", addr, time.Second)
	require.Error(t, err, "listener should be closed after Stop")
}

// TestServer_StopDrainsInFlight verifies that Stop waits for an in-flight
// connection handler to finish before returning, instead of abandoning it.
func TestServer_StopDrainsInFlight(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	s := &Server{listener: ln}
	s.wg.Add(1)
	go s.accept(nil)

	// Simulate a long-running connection handler tracked by the WaitGroup.
	release := make(chan struct{})
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		<-release
	}()

	stopReturned := make(chan struct{})
	go func() { s.Stop(); close(stopReturned) }()

	// Stop should remain blocked while the handler is still running.
	select {
	case <-stopReturned:
		t.Fatal("Stop returned before in-flight handler finished")
	case <-time.After(100 * time.Millisecond):
	}

	// Releasing the handler lets Stop complete.
	close(release)
	select {
	case <-stopReturned:
	case <-time.After(2 * sshDrainTimeout):
		t.Fatal("Stop did not return after the in-flight handler finished")
	}

	// Idempotent: a second Stop is a no-op that returns promptly.
	done := make(chan struct{})
	go func() { s.Stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("second Stop call blocked")
	}

	// The listener is closed.
	_, err = ln.Accept()
	require.Error(t, err)
	require.True(t, errors.Is(err, net.ErrClosed))
}
