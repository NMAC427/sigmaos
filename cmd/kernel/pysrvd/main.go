package main

import (
	"os"

	db "sigmaos/debug"
	"sigmaos/pyenv/srv"
)

func main() {
	if len(os.Args) != 2 {
		db.DFatalf("Usage: %v kernelId", os.Args[0])
	}
	srv.Run(os.Args[1])
}
