package main

import (
	"bytes"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"os/signal"
	"syscall"

	"github.com/AdguardTeam/gomitmproxy"
	"github.com/CorentinB/warc"
)

func main() {
	rotatorSettings := warc.NewRotatorSettings()

	rotatorSettings.Encryption = "ZSTD"
	rotatorSettings.OutputDirectory = "warcs"

	recordChannel, done, err := rotatorSettings.NewWARCRotator()
	if err != nil {
		panic(err)
	}

	proxy := gomitmproxy.NewProxy(gomitmproxy.Config{
		ListenAddr: &net.TCPAddr{
			IP:   net.IPv4(0, 0, 0, 0),
			Port: 8080,
		},
		OnResponse: func(session *gomitmproxy.Session) *http.Response {
			var batch = warc.NewRecordBatch()
			var res = session.Response()
			var req = session.Request()

			dumpRequest, err := httputil.DumpRequestOut(req, true)
			if err != nil {
				panic(err)
			}

			dumpResponse, err := httputil.DumpResponse(res, true)
			if err != nil {
				panic(err)
			}

			// Add the response to the exchange
			var responseRecord = warc.NewRecord()
			responseRecord.Header.Set("WARC-Type", "response")
			responseRecord.Header.Set("WARC-Payload-Digest", "sha1:"+warc.GetSHA1(dumpResponse))
			responseRecord.Header.Set("WARC-Target-URI", req.URL.String())
			responseRecord.Header.Set("Content-Type", "application/http; msgtype=response")

			responseRecord.Content = bytes.NewReader(dumpResponse)

			// Add the request to the exchange
			var requestRecord = warc.NewRecord()
			requestRecord.Header.Set("WARC-Type", "request")
			requestRecord.Header.Set("WARC-Payload-Digest", "sha1:"+warc.GetSHA1(dumpRequest))
			requestRecord.Header.Set("WARC-Target-URI", req.URL.String())
			requestRecord.Header.Set("Host", req.URL.Host)
			requestRecord.Header.Set("Content-Type", "application/http; msgtype=request")

			requestRecord.Content = bytes.NewReader(dumpRequest)

			// Append records to the record batch
			batch.Records = append(batch.Records, responseRecord, requestRecord)

			recordChannel <- batch

			return res
		},
	})

	err = proxy.Start()
	if err != nil {
		log.Fatal(err)
	}

	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, syscall.SIGINT, syscall.SIGTERM)
	<-signalChannel

	// Clean up
	proxy.Close()
	close(recordChannel)
	<-done
}
