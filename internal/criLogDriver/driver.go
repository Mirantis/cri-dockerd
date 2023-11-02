package criLogDriver

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/containerd/fifo"
	"github.com/docker/docker/api/types/plugins/logdriver"
	"github.com/docker/docker/daemon/logger"
	protoio "github.com/gogo/protobuf/io"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type Driver struct {
	mu    sync.Mutex
	fifos map[string]*logPair
	idx   map[string]*logPair
}

type logPair struct {
	criFileLogger logger.Logger
	stream        io.ReadCloser
	info          logger.Info
}

func NewDriver() *Driver {
	return &Driver{
		fifos: make(map[string]*logPair),
		idx:   make(map[string]*logPair),
	}
}

// StartLogging creates a fifo and starts a goroutine to consume it
func (d *Driver) StartLogging(fifoFile string, logCtx logger.Info) error {
	// Check if the fifo already exists
	d.mu.Lock()
	if _, exists := d.fifos[fifoFile]; exists {
		d.mu.Unlock()
		return fmt.Errorf("logger for %q already exists", fifoFile)
	}
	d.mu.Unlock()

	// Create the CRI file logger
	criFileLogger, err := createCRIFileLogger(logCtx)
	if err != nil {
		return errors.Wrap(err, "error creating CRI file logger")
	}

	logrus.WithField("id", logCtx.ContainerID).WithField("file", fifoFile).WithField("cri-log-path", logCtx.LogPath).Debugf("Start logging")
	f, err := fifo.OpenFifo(context.Background(), fifoFile, syscall.O_RDONLY, 0700)
	if err != nil {
		return errors.Wrapf(err, "error opening logger fifo: %q", fifoFile)
	}

	d.mu.Lock()
	lf := &logPair{criFileLogger, f, logCtx}
	d.fifos[fifoFile] = lf
	d.idx[logCtx.ContainerID] = lf
	d.mu.Unlock()

	go consumeLog(lf)
	return nil
}

// createCRIFileLogger creates a logger that writes to the kubelet/CRI log path
func createCRIFileLogger(logCtx logger.Info) (logger.Logger, error) {
	if err := os.MkdirAll(filepath.Dir(logCtx.LogPath), 0755); err != nil {
		return nil, errors.Wrap(err, "error setting up CRI logger path")
	}
	criFileLogger, err := NewCRIFileLogger(logCtx)
	if err != nil {
		return nil, errors.Wrap(err, "error creating CRI logger")
	}
	return criFileLogger, nil
}

// StopLogging closes the fifo and removes the logger from the map
func (d *Driver) StopLogging(fifoFile string) error {
	logrus.WithField("fifo", fifoFile).Debugf("Stop logging")

	d.mu.Lock()
	lf, ok := d.fifos[fifoFile]
	if ok {
		lf.stream.Close()
		delete(d.fifos, fifoFile)
	}
	d.mu.Unlock()
	return nil
}

// consumeLog reads from the fifo and writes to both file loggers
func consumeLog(lf *logPair) {
	dec := protoio.NewUint32DelimitedReader(lf.stream, binary.BigEndian, 1e6)
	defer dec.Close()
	var buf logdriver.LogEntry
	for {
		if err := dec.ReadMsg(&buf); err != nil {
			if err == io.EOF {
				logrus.WithField("id", lf.info.ContainerID).WithError(err).Debug("shutting down log logger")
				lf.stream.Close()
				return
			}
			dec = protoio.NewUint32DelimitedReader(lf.stream, binary.BigEndian, 1e6)
			continue
		}

		var msg logger.Message
		msg.Line = buf.Line
		msg.Source = buf.Source
		if buf.PartialLogMetadata != nil {
			msg.PLogMetaData.ID = buf.PartialLogMetadata.Id
			msg.PLogMetaData.Last = buf.PartialLogMetadata.Last
			msg.PLogMetaData.Ordinal = int(buf.PartialLogMetadata.Ordinal)
		}
		msg.Timestamp = time.Unix(0, buf.TimeNano)

		// Write to the CRI file logger
		if err := lf.criFileLogger.Log(&msg); err != nil {
			logrus.WithField("id", lf.info.ContainerID).WithError(err).WithField("message", msg).Error("error writing CRI log message")
			continue
		}

		buf.Reset()
	}
}

// ReadLogs is not supported by the CRI logger
// Just use the default logger
func (d *Driver) ReadLogs(info logger.Info, config logger.ReadConfig) (io.ReadCloser, error) {
	return nil, fmt.Errorf("CRI logger does not support reading")
}
