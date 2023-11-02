// Package crifilelog provides the Logger implementation for CRI logging. This
// logger logs to files on the host server in the CRI format.
package criLogDriver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/daemon/logger/loggerutils"
	units "github.com/docker/go-units"
	"github.com/pkg/errors"
)

// Name of the driver
const Name = "cri-file"

const initialBufSize = 256

var buffersPool = sync.Pool{New: func() interface{} { return bytes.NewBuffer(make([]byte, 0, initialBufSize)) }}

// CRIFileLogger is the logger implementation for CRI Docker logging
type CRIFileLogger struct {
	writer *loggerutils.LogFile
	tag    string // tag values requested by the user to log
	extra  []byte
}

func init() {
	if err := logger.RegisterLogDriver(Name, NewCRIFileLogger); err != nil {
		panic(err)
	}
	if err := logger.RegisterLogOptValidator(Name, ValidateLogOpt); err != nil {
		panic(err)
	}
}

// NewCRIFileLogger creates new CRIFileLogger which writes to filename passed in
// on given context.
func NewCRIFileLogger(info logger.Info) (logger.Logger, error) {
	var capval int64 = -1
	if capacity, ok := info.Config["max-size"]; ok {
		var err error
		capval, err = units.FromHumanSize(capacity)
		if err != nil {
			return nil, err
		}
		if capval <= 0 {
			return nil, fmt.Errorf("max-size must be a positive number")
		}
	}
	var maxFiles = 1
	if maxFileString, ok := info.Config["max-file"]; ok {
		var err error
		maxFiles, err = strconv.Atoi(maxFileString)
		if err != nil {
			return nil, err
		}
		if maxFiles < 1 {
			return nil, fmt.Errorf("max-file cannot be less than 1")
		}
	}

	var compress bool
	if compressString, ok := info.Config["compress"]; ok {
		var err error
		compress, err = strconv.ParseBool(compressString)
		if err != nil {
			return nil, err
		}
		if compress && (maxFiles == 1 || capval == -1) {
			return nil, fmt.Errorf("compress cannot be true when max-file is less than 2 or max-size is not set")
		}
	}

	attrs, err := info.ExtraAttributes(nil)
	if err != nil {
		return nil, err
	}

	// no default template. only use a tag if the user asked for it
	tag, err := loggerutils.ParseLogTag(info, "")
	if err != nil {
		return nil, err
	}
	if tag != "" {
		attrs["tag"] = tag
	}

	var extra json.RawMessage
	if len(attrs) > 0 {
		var err error
		extra, err = json.Marshal(attrs)
		if err != nil {
			return nil, err
		}
	}

	// No read option from CRI. Just read from the JSON file.
	writer, err := loggerutils.NewLogFile(info.LogPath, capval, maxFiles, compress, nil, 0640, nil)
	if err != nil {
		return nil, err
	}

	return &CRIFileLogger{
		writer: writer,
		tag:    tag,
		extra:  extra,
	}, nil
}

// Log converts logger.Message to CRI format and serializes it to file.
func (c *CRIFileLogger) Log(msg *logger.Message) error {
	buf := buffersPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer buffersPool.Put(buf)

	timestamp := msg.Timestamp
	err := marshalMessage(msg, c.extra, buf)
	logger.PutMessage(msg)

	if err != nil {
		return err
	}

	return c.writer.WriteLogEntry(timestamp, buf.Bytes())
}

func marshalMessage(msg *logger.Message, extra json.RawMessage, buf *bytes.Buffer) error {
	logLine := msg.Line
	if msg.PLogMetaData == nil || (msg.PLogMetaData != nil && msg.PLogMetaData.Last) {
		logLine = append(msg.Line, '\n')
	}

	_, err := buf.WriteString(fmt.Sprintf("%s %s %s %s\n", msg.Timestamp.Format(time.RFC3339), msg.Source, extra, logLine))
	return errors.Wrap(err, "error writing log message to buffer")
}

// ValidateLogOpt looks for specific log options
func ValidateLogOpt(cfg map[string]string) error {
	for key := range cfg {
		switch key {
		case "max-file":
		case "max-size":
		case "compress":
		case "labels":
		case "labels-regex":
		case "env":
		case "env-regex":
		case "tag":
		default:
			return fmt.Errorf("unknown log opt '%s' for json-file log driver", key)
		}
	}
	return nil
}

// Close closes underlying file and signals all the readers
// that the logs producer is gone.
func (c *CRIFileLogger) Close() error {
	return c.writer.Close()
}

// Name returns name of this logger.
func (c *CRIFileLogger) Name() string {
	return Name
}
