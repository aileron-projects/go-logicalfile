package main

import (
	"compress/gzip"
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/aileron-projects/go-logicalfile"
)

func main() {
	sigCtx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	config := &logicalfile.FileConfig{
		SrcDir:     "./src",
		DstDir:     "./dst",
		Pattern:    "application.%i.log",
		ActiveFile: "application.log",
		MaxHistory: 3,
		MaxBytes:   500, // for single file
		CompressLv: gzip.BestCompression,
	}

	f, err := logicalfile.NewFile(config)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	counter := 0
	for {
		counter++
		t := time.Now().Format("15:04:05.000")
		fmt.Fprintf(f, "%04d: %s %s\n", counter, t, strings.Repeat("1234567890", 3)) // 50 bytes in a row.
		select {
		case <-time.After(500 * time.Millisecond):
		case <-sigCtx.Done():
			return
		}
	}
}
