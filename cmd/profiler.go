package cmd

import (
	"log"
	"os"
	"runtime/pprof"
)

func createCpuProfile(filename *string) (*os.File, error) {
	f, err := os.Create(*filename)
	if err != nil {
		log.Printf("Error creating profiler file: %s err: %s", *filename, err.Error())
		return f, err
	}
	log.Println("Starting CPU Profile")
	err = pprof.StartCPUProfile(f)

	return f, err
}
