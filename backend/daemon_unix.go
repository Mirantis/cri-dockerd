//go:build !windows
// +build !windows

package backend

import (
	"errors"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/coreos/go-systemd/v22/activation"
	"github.com/coreos/go-systemd/v22/daemon"
)

func listenFD(addr string) (net.Listener, error) {
	var (
		err       error
		listeners []net.Listener
	)
	// socket activation
	listeners, err = activation.Listeners()
	if err != nil {
		return nil, err
	}

	if len(listeners) == 0 {
		return nil, errors.New(
			"no sockets found via socket activation: make sure the service was started by systemd",
		)
	}

	// default to first fd
	if addr == "" {
		return listeners[0], nil
	}

	return nil, errors.New("not supported yet")
}

func sdNotify(state string) error {
	_, err := daemon.SdNotify(false, state)
	if err != nil {
		//TODO: log the notification error
		return err
	}
	return nil
}

func handleNotify() {
	sdNotify(daemon.SdNotifyReady)
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		sdNotify(daemon.SdNotifyStopping)
	}()
}
