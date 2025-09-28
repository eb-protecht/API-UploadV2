package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"upload-service/configs"
	"upload-service/routes"

	"github.com/gorilla/mux"
)

func main() {
	router := mux.NewRouter()
	configs.ConnectDB()
	configs.ConnectPSQLDatabase()
	routes.ContentRoutes(router)
	routes.CommentRoutes(router)
	routes.LikesRoutes(router)
	routes.FavoritesRoutes(router)
	routes.TransferRoutes(router)
	routes.PromotionRoutes(router)
	configs.ConnectREDISDB()
	router.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "OK")
	})

	// Ready check endpoint (optional)
	router.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Ready")
	})
	port := os.Getenv("PORT")
	if port == "" {
		port = "3001"
	}
	log.Fatal(http.ListenAndServe(":"+port, router))
}
