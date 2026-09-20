package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/aileron-projects/go-logicalfile"
)

// Use last modified time to calculate file ages.
var config1 = &logicalfile.FileConfig{
	DstDir:       "./logs",
	Pattern:      "application.%i.log",
	MaxAge:       30 * time.Second,
	UseParsedAge: false, // Use last modified time.
	MaxBytes:     500,   // for single file
}

// Use timestamp parsed from file name to calculate file ages.
// Timestamp points the time when files are archived.
var config2 = &logicalfile.FileConfig{
	DstDir:       "./logs",
	Pattern:      "application.%Y-%M-%D.%h-%m-%s.log",
	ActiveFile:   "application.log", // set non-empty
	MaxAge:       30 * time.Second,
	UseParsedAge: true, // Parse timestamp from filename.
	MaxBytes:     500,  // for single file
}

// Use timestamp parsed from file name to calculate file ages.
// Timestamp points the time when files are created.
var config3 = &logicalfile.FileConfig{
	DstDir:       "./logs",
	Pattern:      "application.%Y-%M-%D.%h-%m-%s.log",
	ActiveFile:   "", // do not set
	MaxAge:       30 * time.Second,
	UseParsedAge: true, // Parse timestamp from filename.
	MaxBytes:     500,  // for single file
}

func main() {
	_ = config1
	_ = config2
	_ = config3

	f, err := logicalfile.NewFile(config3)
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
