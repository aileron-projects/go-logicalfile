package main

import (
	"log"
	"time"

	"github.com/aileron-projects/go-logicalfile"
)

func main() {
	// Create config with no physical file rotations.
	config := &logicalfile.FileConfig{
		DstDir:     "./logs",
		ActiveFile: "application.log",
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
