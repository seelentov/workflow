package ssh

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/sftp"
	gossh "golang.org/x/crypto/ssh"
)

// Client wraps an SSH connection.
type Client struct {
	Host string
	conn *gossh.Client
}

// Config holds connection parameters.
type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	KeyFile  string
}

// Connect establishes an SSH connection.
func Connect(cfg Config) (*Client, error) {
	var authMethods []gossh.AuthMethod

	if cfg.Password != "" {
		authMethods = append(authMethods, gossh.Password(cfg.Password))
	}

	if cfg.KeyFile != "" {
		signer, err := loadKey(cfg.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("load key %s: %w", cfg.KeyFile, err)
		}
		authMethods = append(authMethods, gossh.PublicKeys(signer))
	}

	// Fall back to well-known default key paths.
	if len(authMethods) == 0 {
		for _, p := range defaultKeyPaths() {
			if signer, err := loadKey(p); err == nil {
				authMethods = append(authMethods, gossh.PublicKeys(signer))
			}
		}
	}

	if cfg.Port == 0 {
		cfg.Port = 22
	}

	sshCfg := &gossh.ClientConfig{
		User:            cfg.User,
		Auth:            authMethods,
		HostKeyCallback: gossh.InsecureIgnoreHostKey(), //nolint:gosec
		Timeout:         30 * time.Second,
	}

	addr := net.JoinHostPort(cfg.Host, fmt.Sprintf("%d", cfg.Port))
	conn, err := gossh.Dial("tcp", addr, sshCfg)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", addr, err)
	}
	return &Client{Host: cfg.Host, conn: conn}, nil
}

// Close closes the SSH connection.
func (c *Client) Close() {
	_ = c.conn.Close()
}

// RunResult holds the output of a remote command.
type RunResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Run executes a shell command on the remote host.
func (c *Client) Run(command string) (*RunResult, error) {
	sess, err := c.conn.NewSession()
	if err != nil {
		return nil, fmt.Errorf("new session: %w", err)
	}
	defer sess.Close()

	var stdout, stderr bytes.Buffer
	sess.Stdout = &stdout
	sess.Stderr = &stderr

	exitCode := 0
	if err := sess.Run(command); err != nil {
		if exitErr, ok := err.(*gossh.ExitError); ok {
			exitCode = exitErr.ExitStatus()
		} else {
			return nil, err
		}
	}

	return &RunResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}, nil
}

// Upload copies a local file to the remote host via SFTP.
func (c *Client) Upload(localPath, remotePath string, mode os.FileMode) error {
	sftpClient, err := sftp.NewClient(c.conn)
	if err != nil {
		return fmt.Errorf("sftp client: %w", err)
	}
	defer sftpClient.Close()

	// Ensure parent directory exists.
	if err := sftpClient.MkdirAll(filepath.ToSlash(filepath.Dir(remotePath))); err != nil {
		return fmt.Errorf("mkdir remote parent: %w", err)
	}

	src, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := sftpClient.OpenFile(remotePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
	if err != nil {
		return fmt.Errorf("open remote file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return err
	}

	if mode != 0 {
		return sftpClient.Chmod(remotePath, mode)
	}
	return nil
}

// UploadBytes writes in-memory content to a remote file.
func (c *Client) UploadBytes(content []byte, remotePath string, mode os.FileMode) error {
	sftpClient, err := sftp.NewClient(c.conn)
	if err != nil {
		return fmt.Errorf("sftp client: %w", err)
	}
	defer sftpClient.Close()

	if err := sftpClient.MkdirAll(filepath.ToSlash(filepath.Dir(remotePath))); err != nil {
		return fmt.Errorf("mkdir remote parent: %w", err)
	}

	dst, err := sftpClient.OpenFile(remotePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
	if err != nil {
		return fmt.Errorf("open remote file: %w", err)
	}
	defer dst.Close()

	if _, err := dst.Write(content); err != nil {
		return err
	}

	if mode != 0 {
		return sftpClient.Chmod(remotePath, mode)
	}
	return nil
}

// Download copies a remote file to a local path.
func (c *Client) Download(remotePath, localPath string) error {
	sftpClient, err := sftp.NewClient(c.conn)
	if err != nil {
		return fmt.Errorf("sftp client: %w", err)
	}
	defer sftpClient.Close()

	src, err := sftpClient.Open(remotePath)
	if err != nil {
		return fmt.Errorf("open remote file: %w", err)
	}
	defer src.Close()

	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return err
	}

	dst, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	return err
}

func loadKey(path string) (gossh.Signer, error) {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, path[2:])
	}
	path = os.ExpandEnv(path)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return gossh.ParsePrivateKey(data)
}

func defaultKeyPaths() []string {
	home, _ := os.UserHomeDir()
	return []string{
		filepath.Join(home, ".ssh", "id_rsa"),
		filepath.Join(home, ".ssh", "id_ed25519"),
		filepath.Join(home, ".ssh", "id_ecdsa"),
	}
}
