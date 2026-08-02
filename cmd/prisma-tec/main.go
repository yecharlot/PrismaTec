package main

import (
	"os"

	"redalset/internal/node"
)

func main() {
	port := "8080"
	if len(os.Args) > 1 {
		port = os.Args[1]
	}
	node.Run(port)
}
