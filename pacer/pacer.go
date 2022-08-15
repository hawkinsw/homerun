package pacer

import (
	"context"
	"io"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/hawkinsw/homerun/v2/debug"
	"github.com/hawkinsw/homerun/v2/utilities"
)

type Pacer interface {
	Start(context.Context, debug.DebugLevel) bool
	TransferredInInterval() (uint64, time.Duration)
	Client() *http.Client
	Valid() bool
	InProgress() bool
	ClientId() uint64
	Connection() utilities.Optional[net.Conn]
}

type countingDownloader struct {
	n            *uint64
	ctx          context.Context
	downloadable io.Reader
}

func (cr *countingDownloader) Read(p []byte) (n int, err error) {
	if cr.ctx.Err() != nil {
		return 0, io.EOF
	}
	n, err = cr.downloadable.Read(p)
	atomic.AddUint64(cr.n, uint64(n))
	return
}

type countingUploader struct {
	n   *uint64
	ctx context.Context
}

func (cr *countingUploader) Read(p []byte) (n int, err error) {
	if cr.ctx.Err() != nil {
		return 0, io.EOF
	}

	err = nil
	n = len(p)

	atomic.AddUint64(cr.n, uint64(n))
	return
}

func (cr *countingUploader) Close() error {
	return nil
}
