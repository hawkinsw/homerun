package pacer

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"sync/atomic"
	"time"

	"github.com/hawkinsw/homerun/v2/debug"
	"github.com/hawkinsw/homerun/v2/trace"
	"github.com/hawkinsw/homerun/v2/utilities"
	"golang.org/x/net/http2"
)

// TODO: All 64-bit fields that are accessed atomically must
// appear at the top of this struct.
type PacerUpload struct {
	uploaded          uint64
	lastIntervalEnd   int64
	Path              string
	downloadStartTime time.Time
	lastDownloaded    uint64
	client            *http.Client
	debug             debug.DebugLevel
	valid             bool
	inprogress        bool
	KeyLogger         io.Writer
	clientId          uint64
	tracer            *httptrace.ClientTrace
	conn              net.Conn
}

func (pu *PacerUpload) SetDnsStartTimeInfo(
	now time.Time,
	dnsStartInfo httptrace.DNSStartInfo,
) {
}

func (pu *PacerUpload) SetDnsDoneTimeInfo(
	now time.Time,
	dnsDoneInfo httptrace.DNSDoneInfo,
) {
}

func (pu *PacerUpload) SetConnectStartTime(
	now time.Time,
) {
}

func (pu *PacerUpload) SetConnectDoneTimeError(
	now time.Time,
	err error,
) {
}

func (pu *PacerUpload) SetGetConnTime(now time.Time) {}

func (pu *PacerUpload) SetGotConnTimeInfo(
	now time.Time,
	gotConnInfo httptrace.GotConnInfo,
) {
	pu.conn = gotConnInfo.Conn
}

func (pu *PacerUpload) SetTLSHandshakeStartTime(
	now time.Time,
) {
}

func (pu *PacerUpload) SetTLSHandshakeDoneTimeState(
	now time.Time,
	connectionState tls.ConnectionState,
) {
}

func (pu *PacerUpload) SetHttpWroteRequestTimeInfo(
	now time.Time,
	info httptrace.WroteRequestInfo,
) {
}

func (pu *PacerUpload) SetHttpResponseReadyTime(
	now time.Time,
) {
}

func (pu *PacerUpload) Connection() utilities.Optional[net.Conn] {
	if utilities.IsInterfaceNil(pu.conn) {
		fmt.Printf("pu.Conn is a nil interface.\n")
		return utilities.None[net.Conn]()
	}
	return utilities.Some(pu.conn)
}

func (pu *PacerUpload) InProgress() bool {
	return pu.inprogress
}

func (pu *PacerUpload) Client() *http.Client {
	return pu.client
}

func (pu *PacerUpload) ClientId() uint64 {
	return pu.clientId
}

func (pu *PacerUpload) Valid() bool {
	return pu.valid
}

func (pu *PacerUpload) TransferredInInterval() (uint64, time.Duration) {
	transferred := atomic.SwapUint64(&pu.uploaded, 0)
	newIntervalEnd := (time.Now().Sub(pu.downloadStartTime)).Nanoseconds()
	previousIntervalEnd := atomic.SwapInt64(&pu.lastIntervalEnd, newIntervalEnd)
	intervalLength := time.Duration(newIntervalEnd - previousIntervalEnd)
	if debug.IsDebug(pu.debug) {
		fmt.Printf("upload: Transferred: %v bytes in %v.\n", transferred, intervalLength)
	}
	return transferred, intervalLength
}

func (pu *PacerUpload) Start(
	parentCtx context.Context,
	debugLevel debug.DebugLevel,
) bool {
	pu.uploaded = 0
	pu.clientId = utilities.GenerateUniqueId()
	transport := http2.Transport{}
	transport.TLSClientConfig = &tls.Config{}
	pu.tracer = trace.GenerateHttpTimingTracer(pu, pu.debug)

	if !utilities.IsInterfaceNil(pu.KeyLogger) {
		if debug.IsDebug(pu.debug) {
			fmt.Printf(
				"Using an SSL Key Logger for this load-generating download.\n",
			)
		}

		// The presence of a custom TLSClientConfig in a *generic* `transport`
		// means that go will default to HTTP/1.1 and cowardly avoid HTTP/2:
		// https://github.com/golang/go/blob/7ca6902c171b336d98adbb103d701a013229c806/src/net/http/transport.go#L278
		// Also, it would appear that the API's choice of HTTP vs HTTP2 can
		// depend on whether the url contains
		// https:// or http://:
		// https://github.com/golang/go/blob/7ca6902c171b336d98adbb103d701a013229c806/src/net/http/transport.go#L74
		transport.TLSClientConfig.KeyLogWriter = pu.KeyLogger
	}
	transport.TLSClientConfig.InsecureSkipVerify = true

	pu.client = &http.Client{Transport: &transport}
	pu.debug = debugLevel
	pu.valid = true
	pu.inprogress = true

	if debug.IsDebug(pu.debug) {
		fmt.Printf(
			"Started a load-generating download (id: %v).\n",
			pu.clientId,
		)
	}

	go pu.upload(parentCtx)
	return true
}

func (pu *PacerUpload) upload(ctx context.Context) {
	var request *http.Request = nil
	var err error = nil

	if request, err = http.NewRequestWithContext(
		httptrace.WithClientTrace(ctx, pu.tracer),
		"POST",
		pu.Path,
		&countingUploader{n: &pu.uploaded, ctx: ctx},
	); err != nil {
		fmt.Printf("upload failed: %v\n", err)
		pu.valid = false
		pu.inprogress = false
		return
	}

	request.Header.Set("Content-Type", "application/octet-stream")

	pu.downloadStartTime = time.Now()
	pu.lastIntervalEnd = 0

	if _, err = pu.client.Do(request); err != nil {
		fmt.Printf("upload failed: %v\n", err)
		pu.valid = false
		pu.inprogress = false
		return
	}
	if debug.IsDebug(pu.debug) {
		fmt.Printf("Ending a load-generating upload.\n")
	}
	pu.inprogress = false
}

func NewPacerUpload(url string) PacerUpload {
	return PacerUpload{Path: url, conn: nil}
}
