package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"wizardboardgame/internal/api"
	"wizardboardgame/internal/game"
	"wizardboardgame/internal/match"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	rankings := flag.String("rankings", "data/rankings.json", "ranking file used by GET /api/ai")
	saves := flag.String("saves", "data/saves", "folder for debug save files")
	flag.Parse()

	svc := match.NewService(game.DefaultRules())
	handler := api.New(svc, *rankings, *saves)

	srv := &http.Server{
		Addr:              *addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("Wizard Boardgame listening on http://localhost%s", *addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
