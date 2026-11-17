package main

import (
	"log"
	"net/http"
	"os"

	"github.com/XLap-nattan0n/calitrack-3d/backend/internal/httpapi"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Fatal(http.ListenAndServe(":"+port, httpapi.NewRouter()))
}
