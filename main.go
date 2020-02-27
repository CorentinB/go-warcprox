package main

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"os/signal"
	"regexp"
	"syscall"
	"time"

	"github.com/CorentinB/warc"
	"github.com/elazarl/goproxy"
)

func main() {
	rotatorSettings := warc.NewRotatorSettings()

	rotatorSettings.Encryption = "ZSTD"
	rotatorSettings.OutputDirectory = "warcs"

	recordChannel, doneRecordChannel, err := rotatorSettings.NewWARCRotator()
	if err != nil {
		panic(err)
	}

	proxy := goproxy.NewProxyHttpServer()
	proxy.Verbose = true

	proxy.OnRequest(goproxy.ReqHostMatches(regexp.MustCompile("^.*$"))).
		HandleConnect(goproxy.AlwaysMitm)
	proxy.OnResponse().DoFunc(
		func(r *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
			var batch = warc.NewRecordBatch()
			var res = r
			var req = r.Request

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
		})

	srv := &http.Server{
		Addr:    ":8080",
		Handler: proxy,
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()
	log.Print("Server Started")

	<-done
	log.Print("Server Stopped")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer func() {
		// extra handling here
		cancel()
	}()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server Shutdown Failed:%+v", err)
	}
	log.Print("Server Exited Properly")

	close(recordChannel)
	<-doneRecordChannel
}
