package transport

import (
	"log/slog"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// SSHHostKeyCallback returns an ssh.HostKeyCallback that verifies against
// the user's ~/.ssh/known_hosts file. If the file doesn't exist or can't
// be parsed, it falls back to ssh.InsecureIgnoreHostKey with a warning.
func SSHHostKeyCallback() ssh.HostKeyCallback {
	home, err := os.UserHomeDir()
	if err != nil {
		slog.Warn("ssh: cannot determine home directory, host key verification disabled")
		return ssh.InsecureIgnoreHostKey()
	}

	knownHostsPath := filepath.Join(home, ".ssh", "known_hosts")
	cb, err := knownhosts.New(knownHostsPath)
	if err != nil {
		slog.Warn("ssh: cannot load known_hosts, host key verification disabled",
			"path", knownHostsPath, "err", err)
		return ssh.InsecureIgnoreHostKey()
	}
	return cb
}
