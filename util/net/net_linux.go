//go:build !android

package net

import (
	"fmt"
	"os"
	"syscall"

	log "github.com/sirupsen/logrus"
)

const envSkipSocketMark = "NB_SKIP_SOCKET_MARK"

// SetSocketMark sets the SO_MARK option on the given socket connection
func SetSocketMark(conn syscall.Conn) error {
	sysconn, err := conn.SyscallConn()
	if err != nil {
		return fmt.Errorf("get raw conn: %w", err)
	}

	return SetRawSocketMark(sysconn)
}

func SetRawSocketMark(conn syscall.RawConn) error {
	var setErr error

	err := conn.Control(func(fd uintptr) {
		setErr = SetSocketOpt(int(fd))
	})
	if err != nil {
		return fmt.Errorf("control: %w", err)
	}

	if setErr != nil {
		return fmt.Errorf("set SO_MARK failed: %w", setErr)
	}

	return nil
}

func SetSocketOpt(fd int) error {
	log.Debugf("Attempting to set SO_MARK on fd %d", fd)
	if CustomRoutingDisabled() {
		log.Infof("Custom routing is disabled, skipping SO_MARK")
		return nil
	}

	// Check for the new environment variable
	if skipSocketMark := os.Getenv(envSkipSocketMark); skipSocketMark == "true" {
		log.Info("NB_SKIP_SOCKET_MARK is set to true, skipping SO_MARK")
		return nil
	}

	err := syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_MARK, NetbirdFwmark)
	if err == nil {
		log.Debugf("Successfully set SO_MARK to %d", NetbirdFwmark)
	}
	return err
}
