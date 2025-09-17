package main

import (
	"goconnect/internal/controller"
	"log"
	"net/http"
	"os"
)

func main() {
	port := os.Getenv("CONTROLLER_PORT")
	if port == "" {
		port = "2538"
	}
	store := controller.NewStore("controller_state.json")
	h := controller.NewHandler(store)
	log.Printf("GOConnect Controller başlatılıyor: http://localhost:%s", port)
	if err := http.ListenAndServe(":"+port, h); err != nil {
		log.Fatalf("Controller başlatılamadı: %v", err)
	}
}
