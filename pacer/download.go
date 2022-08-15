package pacer

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
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
type PacerDownload struct {
	downloaded        uint64
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

func (pd *PacerDownload) SetDnsStartTimeInfo(
	now time.Time,
	dnsStartInfo httptrace.DNSStartInfo,
) {
}

func (pd *PacerDownload) SetDnsDoneTimeInfo(
	now time.Time,
	dnsDoneInfo httptrace.DNSDoneInfo,
) {
}

func (pd *PacerDownload) SetConnectStartTime(
	now time.Time,
) {
}

func (pd *PacerDownload) SetConnectDoneTimeError(
	now time.Time,
	err error,
) {
}

func (pd *PacerDownload) SetGetConnTime(now time.Time) {}

func (pd *PacerDownload) SetGotConnTimeInfo(
	now time.Time,
	gotConnInfo httptrace.GotConnInfo,
) {
	pd.conn = gotConnInfo.Conn
}

func (pd *PacerDownload) SetTLSHandshakeStartTime(
	now time.Time,
) {
}

func (pd *PacerDownload) SetTLSHandshakeDoneTimeState(
	now time.Time,
	connectionState tls.ConnectionState,
) {
}

func (pd *PacerDownload) SetHttpWroteRequestTimeInfo(
	now time.Time,
	info httptrace.WroteRequestInfo,
) {
}

func (pd *PacerDownload) SetHttpResponseReadyTime(
	now time.Time,
) {
}

func (pd *PacerDownload) Client() *http.Client {
	return pd.client
}

func (pd *PacerDownload) ClientId() uint64 {
	return pd.clientId
}

func (pd *PacerDownload) Connection() utilities.Optional[net.Conn] {
	if utilities.IsInterfaceNil(pd.conn) {
		return utilities.None[net.Conn]()
	}
	return utilities.Some(pd.conn)
}

func (pd *PacerDownload) InProgress() bool {
	return pd.inprogress
}

func (pu *PacerDownload) TransferredInInterval() (uint64, time.Duration) {
	transferred := atomic.SwapUint64(&pu.downloaded, 0)
	newIntervalEnd := (time.Now().Sub(pu.downloadStartTime)).Nanoseconds()
	previousIntervalEnd := atomic.SwapInt64(&pu.lastIntervalEnd, newIntervalEnd)
	intervalLength := time.Duration(newIntervalEnd - previousIntervalEnd)
	if debug.IsDebug(pu.debug) {
		fmt.Printf("download: Transferred: %v bytes in %v.\n", transferred, intervalLength)
	}
	return transferred, intervalLength
}

func (pd *PacerDownload) Start(
	parentCtx context.Context,
	debugLevel debug.DebugLevel,
) bool {
	pd.downloaded = 0
	pd.clientId = utilities.GenerateUniqueId()
	transport := http2.Transport{}
	// This configuration option is only available with the patched version of the http2 library.
	// 4 << 20 is the default.
	//transport.AdvertisedStreamWindowSize = 4 << 20
	transport.TLSClientConfig = &tls.Config{}
	pd.tracer = trace.GenerateHttpTimingTracer(pd, pd.debug)

	if !utilities.IsInterfaceNil(pd.KeyLogger) {
		if debug.IsDebug(pd.debug) {
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
		transport.TLSClientConfig.KeyLogWriter = pd.KeyLogger
	}
	transport.TLSClientConfig.InsecureSkipVerify = true

	pd.client = &http.Client{Transport: &transport}
	pd.debug = debugLevel
	pd.valid = true
	pd.inprogress = true

	if debug.IsDebug(pd.debug) {
		fmt.Printf(
			"Started a load-generating download (id: %v).\n",
			pd.clientId,
		)
	}

	go pd.download(parentCtx)
	return true
}

func (pd *PacerDownload) Valid() bool {
	return pd.valid
}

func (pd *PacerDownload) download(ctx context.Context) {
	var request *http.Request = nil
	var get *http.Response = nil
	var err error = nil

	if request, err = http.NewRequestWithContext(
		httptrace.WithClientTrace(ctx, pd.tracer),
		"GET",
		pd.Path,
		nil,
	); err != nil {
		pd.valid = false
		pd.inprogress = false
		return
	}

	pd.downloadStartTime = time.Now()
	pd.lastIntervalEnd = 0

	if get, err = pd.client.Do(request); err != nil {
		pd.valid = false
		pd.inprogress = false
		return
	}
	cr := &countingDownloader{n: &pd.downloaded, ctx: ctx, downloadable: get.Body}
	_, _ = io.Copy(ioutil.Discard, cr)
	get.Body.Close()
	if debug.IsDebug(pd.debug) {
		fmt.Printf("Ending a load-generating download.\n")
	}
	pd.inprogress = false
}

func NewPacerDownload(url string) PacerDownload {
	return PacerDownload{Path: url, conn: nil}
}
