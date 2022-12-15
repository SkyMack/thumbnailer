package main

import (
	"github.com/SkyMack/thumbnailer/internal/clibase"
	"github.com/SkyMack/thumbnailer/internal/generator"
	log "github.com/sirupsen/logrus"
)

const (
	appName        = "thumbnailer"
	appDescription = "Generates sequentially numbered thumbnail images based on the specified image and text settings."
)

func main() {
	rootCmd := clibase.New(appName, appDescription)

	generator.AddCmdGeneratePng(rootCmd)

	if err := rootCmd.Execute(); err != nil {
		log.WithFields(
			log.Fields{
				"app.name": appName,
				"error":    err.Error(),
			},
		).Fatal("application exited with an error")
	}
}
