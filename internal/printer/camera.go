package printer

import (
	"bytes"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"time"
)

type CameraClient struct {
	addr      string
	username  string
	access    string
	timeout   time.Duration
	tlsConfig *tls.Config
}

func NewCameraClient(ip, accessCode, username string, port int, timeout time.Duration) *CameraClient {
	if username == "" {
		username = "bblp"
	}
	if port == 0 {
		port = 6000
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &CameraClient{
		addr:      fmt.Sprintf("%s:%d", ip, port),
		username:  username,
		access:    accessCode,
		timeout:   timeout,
		tlsConfig: &tls.Config{InsecureSkipVerify: true},
	}
}

func (c *CameraClient) Snapshot() ([]byte, error) {
	dialer := &net.Dialer{Timeout: c.timeout}
	conn, err := tls.DialWithDialer(dialer, "tcp", c.addr, c.tlsConfig)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	auth := buildCameraAuth(c.username, c.access)
	if _, err := conn.Write(auth); err != nil {
		return nil, err
	}

	deadline := time.Now().Add(c.timeout)
	_ = conn.SetDeadline(deadline)

	jpegStart := []byte{0xff, 0xd8, 0xff, 0xe0}
	jpegEnd := []byte{0xff, 0xd9}

	var img []byte
	payloadSize := 0

	buf := make([]byte, 4096)
	for time.Now().Before(deadline) {
		n, err := conn.Read(buf)
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				break
			}
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				break
			}
			return nil, err
		}
		if n == 0 {
			continue
		}
		chunk := buf[:n]

		if img != nil && payloadSize > 0 {
			img = append(img, chunk...)
			if len(img) >= payloadSize {
				img = img[:payloadSize]
				if bytes.HasPrefix(img, jpegStart) && bytes.HasSuffix(img, jpegEnd) {
					return img, nil
				}
				img = nil
				payloadSize = 0
			}
			continue
		}

		if n == 16 {
			payloadSize = int(chunk[0]) | int(chunk[1])<<8 | int(chunk[2])<<16
			if payloadSize > 0 {
				img = make([]byte, 0, payloadSize)
			}
			continue
		}
	}

	return nil, errors.New("no camera frame received")
}

func buildCameraAuth(username, access string) []byte {
	buf := &bytes.Buffer{}
	_ = binary.Write(buf, binary.LittleEndian, uint32(0x40))
	_ = binary.Write(buf, binary.LittleEndian, uint32(0x3000))
	_ = binary.Write(buf, binary.LittleEndian, uint32(0))
	_ = binary.Write(buf, binary.LittleEndian, uint32(0))

	writePadded(buf, []byte(username), 32)
	writePadded(buf, []byte(access), 32)
	return buf.Bytes()
}

func writePadded(buf *bytes.Buffer, b []byte, size int) {
	if len(b) > size {
		buf.Write(b[:size])
		return
	}
	buf.Write(b)
	pad := make([]byte, size-len(b))
	buf.Write(pad)
}
