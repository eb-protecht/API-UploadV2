package routes

import (
	"upload-service/controllers"

	"github.com/gorilla/mux"
)

func LikesRoutes(router *mux.Router) {
	router.HandleFunc("/uploadmicro/v1/like/{UserID}/{LikedContent}", controllers.Like()).Methods("POST") //notifications implemented
}
