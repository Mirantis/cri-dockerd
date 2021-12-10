// +build windows

package backend

import (
	"net"

	"github.com/pkg/errors"
)

func listenFD(addr string) (net.Listener, error) {
	return nil, errors.New("listening on a file descriptor is not supported on Windows")
}

func handleNotify() {
}
