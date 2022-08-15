package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/hawkinsw/homerun/v2/ccw"
	"github.com/hawkinsw/homerun/v2/debug"
	"github.com/hawkinsw/homerun/v2/estats"
	"github.com/hawkinsw/homerun/v2/pacer"
	"github.com/hawkinsw/homerun/v2/utilities"
)

func statter(ctx context.Context, testDuration time.Duration, sampleDuration time.Duration, p pacer.Pacer) uint64 {
	startTime := time.Now()
	totalXfer := uint64(0)
	fmt.Printf("RTT, Cwnd, Xfer\n")
	for ctx.Err() == nil {
		if !p.InProgress() || !p.Valid() {
			fmt.Printf("Leaving statter early -- the connection is not ongoing or became invalid.\n")
			break
		}
		time.Sleep(sampleDuration)
		stats := estats.ExtendedStats{}
		if connection := p.Connection(); utilities.IsSome(connection) {
			connection := utilities.GetSome(connection)
			stats.IncorporateConnectionStats(connection)
			xfer, duration := p.TransferredInInterval()
			totalXfer += xfer
			fmt.Printf("%v, %v, %v\n", stats.AverageRtt, stats.AverageCwnd, float64(xfer)/duration.Seconds())
		}
		elapsedTime := time.Now().Sub(startTime)
		if elapsedTime > testDuration {
			break
		}
	}
	return totalXfer
}

func main() {
	keyFilename := "key.key"
	var sslKeyFileConcurrentWriter *ccw.ConcurrentWriter = nil
	if sslKeyFileHandle, err := os.OpenFile(keyFilename, os.O_RDWR|os.O_CREATE, os.FileMode(0600)); err != nil {
		fmt.Printf("Alert: Could not open the keyfile for writing: %v! Its contents will be invalid.\n", err)
		sslKeyFileConcurrentWriter = nil
	} else {
		if err = utilities.SeekForAppend(sslKeyFileHandle); err != nil {
			fmt.Printf("Alert: Could not seek to the end of the keyfile: %v Its contents will be invalid.\n", err)
		} else {
			sslKeyFileConcurrentWriter = ccw.NewConcurrentFileWriter(sslKeyFileHandle)
			defer sslKeyFileHandle.Close()
		}
	}

	runnerCtx, runnerCtxCancel := context.WithCancel(context.Background())
	//p := pacer.NewPacerUpload("https://rpm.obs.cr:4043/slurp")
	p := pacer.NewPacerDownload("https://rpm.obs.cr/large/")
	p.KeyLogger = sslKeyFileConcurrentWriter
	if p.Start(runnerCtx, debug.Debug) != true {
		fmt.Printf("Oops: Error occurred starting the pacer!\n")
		return
	}
	fmt.Printf("Starting statter ...\n")
	totalXfer := statter(runnerCtx, 100*time.Second, 500*time.Millisecond, &p)
	fmt.Printf("Total transferred: %v\n", totalXfer)
	fmt.Printf("Done\n")
	runnerCtxCancel()
	fmt.Printf("Done sleeping!\n")
}
