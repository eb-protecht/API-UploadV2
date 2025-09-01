package routes

import (
	"upload-service/controllers"

	"github.com/gorilla/mux"
)

func PromotionRoutes(router *mux.Router) {
	router.HandleFunc("/post-promoting/{UserID}/{Title}/{Description}/{Show}/{IsPayPerView}/{PPVPrice}/{IsDeleted}/{Tags}", controllers.PostVideoToPostgres()).Methods("POST")
	router.HandleFunc("/promote/v1/{ContentID}/{Start}/{End}/{TargetAge}/{TargetGender}/{Amount}", controllers.Promote()).Methods("POST")
	router.HandleFunc("/register", controllers.RegisterUser()).Methods("POST")
}
