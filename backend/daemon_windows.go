//go:build windows
// +build windows

package backend

import (
	"errors"
	"net"
)

func listenFD(addr string) (net.Listener, error) {
	return nil, errors.New("listening on a file descriptor is not supported on Windows")
}

func handleNotify() {
}
