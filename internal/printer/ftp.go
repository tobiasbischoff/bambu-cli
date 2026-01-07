package printer

import (
	"crypto/tls"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/jlaffaye/ftp"
)

type FTPClient struct {
	addr      string
	user      string
	pass      string
	timeout   time.Duration
	tlsConfig *tls.Config
}

func NewFTPClient(ip, accessCode, username string, port int, timeout time.Duration) *FTPClient {
	if username == "" {
		username = "bblp"
	}
	if port == 0 {
		port = 990
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &FTPClient{
		addr:      fmt.Sprintf("%s:%d", ip, port),
		user:      username,
		pass:      accessCode,
		timeout:   timeout,
		tlsConfig: &tls.Config{InsecureSkipVerify: true},
	}
}

func (c *FTPClient) withConn(fn func(*ftp.ServerConn) error) error {
	conn, err := ftp.Dial(c.addr, ftp.DialWithTimeout(c.timeout), ftp.DialWithTLS(c.tlsConfig))
	if err != nil {
		return err
	}
	defer conn.Quit()
	if err := conn.Login(c.user, c.pass); err != nil {
		return err
	}
	return fn(conn)
}

func (c *FTPClient) List(path string) ([]string, error) {
	var entries []string
	err := c.withConn(func(conn *ftp.ServerConn) error {
		list, err := conn.List(path)
		if err != nil {
			return err
		}
		for _, e := range list {
			entries = append(entries, e.Name)
		}
		return nil
	})
	return entries, err
}

func (c *FTPClient) Upload(localPath, remotePath string) error {
	return c.withConn(func(conn *ftp.ServerConn) error {
		f, err := os.Open(localPath)
		if err != nil {
			return err
		}
		defer f.Close()
		return conn.Stor(remotePath, f)
	})
}

func (c *FTPClient) UploadReader(r io.Reader, remotePath string) error {
	return c.withConn(func(conn *ftp.ServerConn) error {
		return conn.Stor(remotePath, r)
	})
}

func (c *FTPClient) Download(remotePath string, w io.Writer) error {
	return c.withConn(func(conn *ftp.ServerConn) error {
		r, err := conn.Retr(remotePath)
		if err != nil {
			return err
		}
		defer r.Close()
		_, err = io.Copy(w, r)
		return err
	})
}

func (c *FTPClient) Delete(remotePath string) error {
	return c.withConn(func(conn *ftp.ServerConn) error {
		return conn.Delete(remotePath)
	})
}
