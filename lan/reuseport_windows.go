//go:build windows

package lan

import (
	"context"
	"errors"
	"net"
	"syscall"
)

// ErrUnexpectedConnType is returned when the connection is not a UDP connection.
var ErrUnexpectedConnType = errors.New("unexpected connection type")

// listenUDPReusable creates a UDP socket with SO_REUSEADDR set,
// allowing multiple processes to bind to the same port.
func listenUDPReusable(ctx context.Context, port int) (*net.UDPConn, error) {
	lc := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			var opErr error

			err := c.Control(func(fd uintptr) {
				// On Windows, SO_REUSEADDR allows port sharing
				opErr = syscall.SetsockoptInt(syscall.Handle(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
			})
			if err != nil {
				return err
			}

			return opErr
		},
	}

	conn, err := lc.ListenPacket(ctx, "udp4", (&net.UDPAddr{Port: port}).String())
	if err != nil {
		return nil, err
	}

	udpConn, ok := conn.(*net.UDPConn)
	if !ok {
		_ = conn.Close()

		return nil, ErrUnexpectedConnType
	}

	return udpConn, nil
}
