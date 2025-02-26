package main

import "github.com/socheatsok78/sentrytunnel"

func main() {
	if err := sentrytunnel.Run(); err != nil {
		panic(err)
	}
}
