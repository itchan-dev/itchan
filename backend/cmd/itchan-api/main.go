package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/itchan-dev/itchan/backend/internal/router"
	"github.com/itchan-dev/itchan/backend/internal/setup"
	"github.com/itchan-dev/itchan/shared/config"
)

func main() {
	log.SetFlags(log.Lshortfile)

	var configFolder string
	flag.StringVar(&configFolder, "config_folder", "backend/config", "path to folder with configs")
	flag.Parse()

	cfg := config.MustLoad(configFolder)

	deps, err := setup.SetupDependencies(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize dependencies: %v", err)
	}
	defer deps.Storage.Cleanup()

	r := router.New(deps)

	httpPort := os.Getenv("PORT")
	if httpPort == "" {
		httpPort = "8080"
	}

	log.Printf("Server started on port %s", httpPort)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", httpPort), r))
}
