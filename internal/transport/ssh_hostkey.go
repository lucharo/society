package transport

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

var (
	hostKeyCBOnce sync.Once
	hostKeyCB     ssh.HostKeyCallback
)

// SSHHostKeyCallback returns an ssh.HostKeyCallback that verifies against
// the user's ~/.ssh/known_hosts file. The callback is loaded once and cached
// for the lifetime of the process. If the file doesn't exist or can't be
// parsed, all connections are rejected with a descriptive error.
func SSHHostKeyCallback() ssh.HostKeyCallback {
	hostKeyCBOnce.Do(func() {
		home, err := os.UserHomeDir()
		if err != nil {
			slog.Warn("ssh: cannot determine home directory, host key verification will reject all connections")
			hostKeyCB = rejectAllHostKeys("cannot determine home directory: " + err.Error())
			return
		}

		knownHostsPath := filepath.Join(home, ".ssh", "known_hosts")
		cb, err := knownhosts.New(knownHostsPath)
		if err != nil {
			slog.Warn("ssh: cannot load known_hosts, host key verification will reject all connections",
				"path", knownHostsPath, "err", err)
			hostKeyCB = rejectAllHostKeys(fmt.Sprintf("cannot load %s: %v", knownHostsPath, err))
			return
		}
		hostKeyCB = cb
	})
	return hostKeyCB
}

// BuildSSHClientConfig reads an SSH private key and returns a configured
// ssh.ClientConfig with known_hosts verification and a 10-second timeout.
func BuildSSHClientConfig(user, keyPath string) (*ssh.ClientConfig, error) {
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("reading SSH key: %w", err)
	}
	signer, err := ssh.ParsePrivateKey(keyData)
	if err != nil {
		return nil, fmt.Errorf("parsing SSH key: %w", err)
	}
	return &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: SSHHostKeyCallback(),
		Timeout:         10 * time.Second,
	}, nil
}

// rejectAllHostKeys returns a callback that rejects every host key with
// the given reason, guiding the user to populate their known_hosts file.
func rejectAllHostKeys(reason string) ssh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		return fmt.Errorf("ssh: host key verification failed for %s (%s); add the host to ~/.ssh/known_hosts or connect manually once with ssh", hostname, reason)
	}
}
