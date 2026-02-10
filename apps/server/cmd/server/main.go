package main

import (
	"log"

	"github.com/lifei6671/plaindoc/apps/server/internal/config"
	"github.com/lifei6671/plaindoc/apps/server/internal/server"
)

func main() {
	cfg := config.Load()

	router := server.NewRouter(cfg)

	log.Printf("server starting on %s (env=%s)", cfg.Addr, cfg.Env)
	if err := router.Run(cfg.Addr); err != nil {
		log.Fatalf("server exited: %v", err)
	}
}
