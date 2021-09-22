// +build !dockerless windows

package server

import (
	"net"

	"github.com/pkg/errors"
)

func listenFD(addr string) (net.Listener, error) {
	return nil, errors.New("listening server on fd not supported on windows")
}

func handleNotify() {
}
