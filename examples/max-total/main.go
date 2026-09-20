package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/aileron-projects/go-logicalfile"
)

func main() {
	config := &logicalfile.FileConfig{
		DstDir:        "./logs",
		Pattern:       "application.%i.log",
		ActiveFile:    "application.log",
		MaxTotalBytes: 4 * 500, // 4 files
		MaxBytes:      500,     // for single file
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
		time.Sleep(500 * time.Millisecond)                                           // 100 bytes in a second.
	}
}
