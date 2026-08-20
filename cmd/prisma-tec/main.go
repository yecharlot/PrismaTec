package main

import (
	"os"

	"redalset/internal/node"
)

func main() {
	// Render (and most PaaS) inject PORT. Prefer env, then argv, then 8080.
	port := os.Getenv("PORT")
	if port == "" && len(os.Args) > 1 {
		port = os.Args[1]
	}
	if port == "" {
		port = "8080"
	}
	node.Run(port)
}
