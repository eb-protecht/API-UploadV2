// Package main API UploadV2 Service
//
// This is the API UploadV2 microservice for the Syn platform.
// It handles content upload and management functionality.
//
// Terms Of Service: http://swagger.io/terms/
//
// Schemes: http, https
// Host: localhost:30970
// BasePath: /
// Version: 2.0.0
//
// Consumes:
// - application/json
// - multipart/form-data
//
// Produces:
// - application/json
//
// swagger:meta
package main

import (
	"fmt"
	"log"
	"net/http"
	"upload-service/configs"
	"upload-service/routes"

	_ "upload-service/docs" // This is required for swagger docs

	"github.com/gorilla/mux"
	httpSwagger "github.com/swaggo/http-swagger"
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

	// Swagger endpoint
	router.PathPrefix("/swagger/").Handler(httpSwagger.WrapHandler)

	log.Fatal(http.ListenAndServe(":30970", router))
}
