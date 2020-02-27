package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"syscall"
	"time"

	"github.com/CorentinB/warc"
	"github.com/elazarl/goproxy"
)

var counter = 0
var date = time.Now().Format("2006-01-02_15:04:05")

func main() {
	rotatorSettings := warc.NewRotatorSettings()

	rotatorSettings.Encryption = "ZSTD"
	rotatorSettings.OutputDirectory = "warcs"

	recordChannel, doneRecordChannel, err := rotatorSettings.NewWARCRotator()
	if err != nil {
		panic(err)
	}

	proxy := goproxy.NewProxyHttpServer()

	proxy.OnRequest(goproxy.ReqHostMatches(regexp.MustCompile("^.*$"))).
		HandleConnect(goproxy.AlwaysMitm)
	proxy.OnRequest().DoFunc(
		func(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
			if time.Now().Format("2006-01-02_15:04:05") != date {
				date = time.Now().Format("2006-01-02_15:04:05")
				fmt.Println(strconv.Itoa(counter) + "req/s")
				counter = 0
			} else {
				counter++
			}

			var batch = warc.NewRecordBatch()
			var req = r

			dumpRequest, err := httputil.DumpRequest(req, true)
			if err != nil {
				panic(err)
			}

			// Add the request to the exchange
			var requestRecord = warc.NewRecord()
			requestRecord.Header.Set("WARC-Type", "request")
			requestRecord.Header.Set("WARC-Payload-Digest", "sha1:"+warc.GetSHA1(dumpRequest))
			requestRecord.Header.Set("WARC-Target-URI", req.URL.String())
			requestRecord.Header.Set("Host", req.URL.Host)
			requestRecord.Header.Set("Content-Type", "application/http; msgtype=request")

			requestRecord.Content = bytes.NewReader(dumpRequest)

			// Append record to the record batch
			batch.Records = append(batch.Records, requestRecord)

			recordChannel <- batch

			return r, nil
		})
	proxy.OnResponse().DoFunc(
		func(r *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
			var batch = warc.NewRecordBatch()
			var res = r

			dumpResponse, err := httputil.DumpResponse(res, true)
			if err != nil {
				panic(err)
			}

			// Add the response to the exchange
			var responseRecord = warc.NewRecord()
			responseRecord.Header.Set("WARC-Type", "response")
			responseRecord.Header.Set("WARC-Payload-Digest", "sha1:"+warc.GetSHA1(dumpResponse))
			responseRecord.Header.Set("WARC-Target-URI", res.Request.URL.String())
			responseRecord.Header.Set("Content-Type", "application/http; msgtype=response")

			responseRecord.Content = bytes.NewReader(dumpResponse)

			// Append records to the record batch
			batch.Records = append(batch.Records, responseRecord)

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
