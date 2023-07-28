package main

import (
	"log"

	"github.com/tetsuzawa/alp-trace/cmd/alp-trace/cmd"
)

var version string

func main() {
	if err := cmd.Execute(version); err != nil {
		log.Fatal(err)
	}
}
