package main

import (
	"log"

	"github.com/socheatsok78/sentrytunnel"
)

func main() {
	if err := sentrytunnel.Run(); err != nil {
		log.Fatal(err)
	}
}
