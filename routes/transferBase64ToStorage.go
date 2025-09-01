package routes

import (
	"upload-service/controllers"

	"github.com/gorilla/mux"
)

func TransferRoutes(router *mux.Router) {
	router.HandleFunc("/transfer", controllers.TransferProfilePics()).Methods("POST")
}
