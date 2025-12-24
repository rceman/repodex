package main

import (
	"os"

	"github.com/memkit/repodex/internal/app"
)

func main() {
	code := app.Run(os.Args[1:])
	os.Exit(code)
}
