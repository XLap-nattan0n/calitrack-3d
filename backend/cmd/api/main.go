package main

import (
	"log"
	"net/http"

	"github.com/XLap-nattan0n/calitrack-3d/backend/internal/config"
	"github.com/XLap-nattan0n/calitrack-3d/backend/internal/httpapi"
)

// Vercel runs this as a server on $PORT; locally it is the same router: `go run ./cmd/api`.
func main() {
	cfg := config.Load()
	log.Printf("listening on :%s", cfg.Port)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, httpapi.NewRouter(cfg)))
}
