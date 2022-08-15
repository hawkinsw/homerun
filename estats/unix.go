//go:build dragonfly || freebsd || linux || netbsd || openbsd
// +build dragonfly freebsd linux netbsd openbsd

package estats

import (
	"crypto/tls"
	"fmt"
	"net"

	"github.com/hawkinsw/homerun/v2/utilities"
	"golang.org/x/sys/unix"
)

type ExtendedStats struct {
	MaxPathMtu           uint64
	MaxSendMss           uint64
	MaxRecvMss           uint64
	TotalRetransmissions uint64
	TotalReorderings     uint64
	AverageCwnd          float64
	AverageRtt           float64
	rtt_measurements     uint64
	total_rtt            float64
	cwnd_measurements    uint64
	total_cwnd           float64
}

func ExtendedStatsAvailable() bool {
	return true
}

func (es *ExtendedStats) IncorporateConnectionStats(rawConn net.Conn) error {
	tlsConn, ok := rawConn.(*tls.Conn)
	if !ok {
		return fmt.Errorf(
			"OOPS: Could not get the TCP info for the connection (not a TLS connection)",
		)
	}
	tcpConn, ok := tlsConn.NetConn().(*net.TCPConn)
	if !ok {
		return fmt.Errorf(
			"OOPS: Could not get the TCP info for the connection (not a TCP connection)",
		)
	}
	if info, err := getTCPInfo(tcpConn); err != nil {
		return fmt.Errorf("OOPS: Could not get the TCP info for the connection: %v", err)
	} else {
		es.MaxPathMtu = utilities.Max(es.MaxPathMtu, uint64(info.Pmtu))
		es.MaxRecvMss = utilities.Max(es.MaxRecvMss, uint64(info.Rcv_mss))
		es.MaxSendMss = utilities.Max(es.MaxSendMss, uint64(info.Snd_mss))
		// https://lkml.iu.edu/hypermail/linux/kernel/1705.0/01790.html
		es.TotalRetransmissions += uint64(info.Total_retrans)
		es.TotalReorderings += uint64(info.Reordering)
		es.total_rtt += float64(info.Rtt)
		es.rtt_measurements += 1
		es.AverageRtt = es.total_rtt / float64(es.rtt_measurements)
		es.total_cwnd += float64(info.Snd_cwnd)
		es.cwnd_measurements += 1
		es.AverageCwnd = es.total_cwnd / float64(es.cwnd_measurements)
	}
	return nil
}

func (es *ExtendedStats) Repr() string {
	return fmt.Sprintf(`Extended Statistics:
	Maximum Path MTU: %v
	Maximum Send MSS: %v
	Maximum Recv MSS: %v
	Total Retransmissions: %v
	Total Reorderings: %v
	Average RTT: %v
`, es.MaxPathMtu, es.MaxSendMss, es.MaxRecvMss, es.TotalRetransmissions, es.TotalReorderings, es.AverageRtt)
}

func getTCPInfo(connection net.Conn) (*unix.TCPInfo, error) {
	tcpConn, ok := connection.(*net.TCPConn)
	if !ok {
		return nil, fmt.Errorf("connection is not a net.TCPConn")
	}
	rawConn, err := tcpConn.SyscallConn()
	if err != nil {
		return nil, err
	}

	var info *unix.TCPInfo = nil
	rawConn.Control(func(fd uintptr) {
		info, err = unix.GetsockoptTCPInfo(int(fd), unix.SOL_TCP, unix.TCP_INFO)
	})
	return info, err
}
