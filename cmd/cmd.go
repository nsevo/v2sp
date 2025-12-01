package cmd

import (
	log "github.com/sirupsen/logrus"

	_ "github.com/nsevo/v2sp/core/imports"
	"github.com/spf13/cobra"
)

var command = &cobra.Command{
	Use: "v2sp",
}

func Run() {
	err := command.Execute()
	if err != nil {
		log.WithField("err", err).Error("Execute command failed")
	}
}
