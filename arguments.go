package main

import (
	"fmt"
	"os"

	"github.com/akamensky/argparse"
)

var arguments = struct {
	OutputDirectory string
	Compression     string
	Address         string
	WARCPrefix      string
	WARCSize        float64
}{}

func parseArgs(args []string) {
	// Create new parser object
	parser := argparse.NewParser("go-warcprox", "WARC writing MITM HTTP/S proxy in Go")

	outputDirectory := parser.String("o", "output", &argparse.Options{
		Required: false,
		Help:     "Output directory for WARC files",
		Default:  "warcs"})

	compression := parser.String("c", "compression", &argparse.Options{
		Required: false,
		Help:     "Compression algorithm (GZIP or ZSTD)",
		Default:  "GZIP",
	})

	address := parser.String("a", "address", &argparse.Options{
		Required: false,
		Help:     "",
		Default:  ":8080",
	})

	warcPrefix := parser.String("p", "warc-prefix", &argparse.Options{
		Required: false,
		Help:     "WARC files prefix",
		Default:  "WARC",
	})

	warcSize := parser.Int("s", "warc-size", &argparse.Options{
		Required: false,
		Help:     "Size in MB of WARC files",
		Default:  1000,
	})

	// Parse input
	err := parser.Parse(args)
	if err != nil {
		// In case of error print error and print usage
		// This can also be done by passing -h or --help flags
		fmt.Println(parser.Usage(err))
		os.Exit(0)
	}

	arguments.OutputDirectory = *outputDirectory
	arguments.Compression = *compression
	arguments.Address = *address
	arguments.WARCPrefix = *warcPrefix
	arguments.WARCSize = float64(*warcSize)
}
