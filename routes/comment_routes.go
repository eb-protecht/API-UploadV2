package routes

import (
	"upload-service/controllers"

	"github.com/gorilla/mux"
)

func CommentRoutes(router *mux.Router) {
	router.HandleFunc("/uploadmicro/v1/addComment/{UserID}/{ReplyTo}/{Comment}/{ReplyToComment}", controllers.AddComment()).Methods("POST")                                 // implemented notifications
	router.HandleFunc("/uploadmicro/v1/addComment/{UserID}/{ReplyTo}/{ReplyToComment}", controllers.AddCommentWithBody()).Methods("POST")                                   // implemented notifications
	router.HandleFunc("/uploadmicro/v1/addComment/{UserID}/{ReplyTo}/{IsReply}/{OwnerUserID}/{ContentID}", controllers.AddCommentWithBodyWithOtherUserID()).Methods("POST") // implemented notifications
	router.HandleFunc("/uploadmicro/v1/editComment/{CommentID}/{Comment}", controllers.EditComment()).Methods("PUT")
	router.HandleFunc("/uploadmicro/v1/editComment/{CommentID}", controllers.EditCommentWithBody()).Methods("POST")
	router.HandleFunc("/uploadmicro/v1/deleteComment/{CommentID}", controllers.DeleteComment()).Methods("POST")
	router.HandleFunc("/uploadmicro/v1/backfillComments", controllers.BackfillComments()).Methods("GET") // implemented notifications
}
