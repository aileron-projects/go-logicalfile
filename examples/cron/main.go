package main

import (
	"log"
	"time"

	"github.com/aileron-projects/go-logicalfile"
)

func main() {
	now := time.Now()
	config := &logicalfile.FileConfig{
		DstDir:     "./logs",
		Pattern:    "application.%h-%m-%s.log",
		ActiveFile: "application.log",
		RotateAt: func() time.Time {
			next := now.Add(10 * time.Second)
			now = next
			return next
		},
	}

	f, err := logicalfile.NewFile(config)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	// Use logical file for log output.
	log.SetOutput(f)

	for {
		log.Println("hello, gopher !!")
		time.Sleep(time.Second)
	}
}
