package main

import (
	"os"

	"github.com/lmzuccarelli/golang-imageindex-list/pkg/cli"
	clog "github.com/lmzuccarelli/golang-imageindex-list/pkg/log"
)

func main() {

	log := clog.New("info")
	cmd := cli.NewCliCmd(log)
	err := cmd.Execute()
	if err != nil {
		log.Error("[main] %v ", err)
		os.Exit(1)
	}

}
