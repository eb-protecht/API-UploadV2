package routes

import (
	"upload-service/controllers"

	"github.com/gorilla/mux"
)

func FeedbackRoutes(router *mux.Router) {
	router.HandleFunc("/uploadmicro/v1/feedback", controllers.SubmitFeedback()).Methods("POST")
	router.HandleFunc("/uploadmicro/v1/feedback", controllers.GetFeedback()).Methods("GET")
}
