package routes

import (
	"upload-service/controllers"

	"github.com/gorilla/mux"
)

// MediaURLRoutes registers routes for media URL conversion utilities
func MediaURLRoutes(router *mux.Router) {
	// Convert file path to URL
	router.HandleFunc("/media/path-to-url", controllers.ConvertPathToURL()).Methods("GET")

	// Convert URL to file path
	router.HandleFunc("/media/url-to-path", controllers.ConvertURLToPath()).Methods("GET")

	// Get media information with URLs
	router.HandleFunc("/media/{userID}/{fileType}/{filename}/info", controllers.GetMediaInfo()).Methods("GET")
}
