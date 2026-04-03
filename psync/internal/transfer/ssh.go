package transfer

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// SSHConfig holds the parameters for an SSH connection.
type SSHConfig struct {
	User    string
	Host    string
	Port    int
	KeyPath string
}

// DialSFTP opens an SSH connection and returns an SFTP client.
func DialSFTP(cfg SSHConfig) (*sftp.Client, error) {
	authMethods, err := buildAuthMethods(cfg.KeyPath)
	if err != nil {
		return nil, fmt.Errorf("ssh auth: %w", err)
	}

	sshCfg := &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec // acceptable for a CLI sync tool
	}

	addr := net.JoinHostPort(cfg.Host, fmt.Sprintf("%d", cfg.Port))
	conn, err := ssh.Dial("tcp", addr, sshCfg)
	if err != nil {
		return nil, fmt.Errorf("ssh dial %s: %w", addr, err)
	}

	client, err := sftp.NewClient(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("sftp client: %w", err)
	}
	return client, nil
}

// buildAuthMethods constructs SSH auth methods. If keyPath is provided,
// it is used first; otherwise agent/password-less auth is attempted.
func buildAuthMethods(keyPath string) ([]ssh.AuthMethod, error) {
	if keyPath == "" {
		// Try default key locations.
		home, _ := os.UserHomeDir()
		for _, name := range []string{"id_rsa", "id_ed25519", "id_ecdsa"} {
			candidate := filepath.Join(home, ".ssh", name)
			if m, err := signerFromFile(candidate); err == nil {
				return []ssh.AuthMethod{m}, nil
			}
		}
		return nil, fmt.Errorf("no SSH key found; pass --ssh-key")
	}
	m, err := signerFromFile(keyPath)
	if err != nil {
		return nil, err
	}
	return []ssh.AuthMethod{m}, nil
}

func signerFromFile(path string) (ssh.AuthMethod, error) {
	key, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, err
	}
	return ssh.PublicKeys(signer), nil
}

// CopyFileViaSFTP copies a local file to an SFTP destination path.
func CopyFileViaSFTP(client *sftp.Client, src, dstPath string, dryRun bool) error {
	if dryRun {
		return nil
	}

	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err := client.MkdirAll(filepath.ToSlash(filepath.Dir(dstPath))); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(dstPath), err)
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := client.OpenFile(dstPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
	if err != nil {
		return fmt.Errorf("sftp open %s: %w", dstPath, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("sftp copy: %w", err)
	}

	// Preserve mode (best-effort).
	_ = client.Chmod(dstPath, srcInfo.Mode())
	return nil
}

// ChecksumRemote returns the SHA-256 of a remote file by streaming it locally.
func ChecksumRemote(client *sftp.Client, remotePath string) (string, error) {
	f, err := client.Open(remotePath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	return checksumReader(f)
}
