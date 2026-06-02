package main

import (
	"fmt"

	"github.com/scality/virtualip-manager/cmd/config"
)

func main() {
	fmt.Printf("Starting %s:%s\n", config.ApplicationName, config.ApplicationVersion)
}
