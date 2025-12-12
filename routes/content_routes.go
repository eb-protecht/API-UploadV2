package routes

import (
	"upload-service/controllers"

	"github.com/gorilla/mux"
)

func ContentRoutes(router *mux.Router) {
	router.HandleFunc("/uploadmicro/v1/postprof/{UserID}/{IsCurrent}", controllers.PostProfilePicBase64()).Methods("POST")
	router.HandleFunc("/uploadmicro/v1/makeCurrentPostProf/{UserID}/{Filename}/{ToBeChanged}", controllers.UpdateOnProfilePic()).Methods("PUT")
	router.HandleFunc("/uploadmicro/v1/postpic/{UserID}/{Title}/{Description}/{Show}/{IsPayPerView}/{PPVPrice}/{IsDeleted}/{Tags}/{Visibility}", controllers.PostPic()).Methods("POST")
	router.HandleFunc("/uploadmicro/v1/postpic/{UserID}/{Show}/{IsPayPerView}/{PPVPrice}/{IsDeleted}/{Tags}/{Visibility}", controllers.PostPicWithBody()).Methods("POST")
	router.HandleFunc("/uploadmicro/v1/postvid/{UserID}/{Title}/{Description}/{Show}/{IsPayPerView}/{PPVPrice}/{IsDeleted}/{Tags}", controllers.PostVideo()).Methods("POST")
	router.HandleFunc("/uploadmicro/v1/postvidnt/{UserID}/{Title}/{Description}/{Show}/{IsPayPerView}/{PPVPrice}/{IsDeleted}/{Tags}/{Visibility}", controllers.PostVideoNT()).Methods("POST")
	router.HandleFunc("/uploadmicro/v1/postvidnt/{UserID}/{Show}/{IsPayPerView}/{PPVPrice}/{IsDeleted}/{Tags}/{Visibility}", controllers.PostVideoNTWithBody()).Methods("POST")
	router.HandleFunc("/uploadmicro/v1/postText/{UserID}/{Title}/{Description}/{Show}/{IsPayPerView}/{PPVPrice}/{IsDeleted}/{Tags}/{Posting}/{Visibility}", controllers.PostText()).Methods("POST")
	router.HandleFunc("/uploadmicro/v1/postText/{UserID}/{Show}/{IsPayPerView}/{PPVPrice}/{IsDeleted}/{Tags}/{Visibility}", controllers.PostTextWithBody()).Methods("POST")
	router.HandleFunc("/uploadmicro/v1/editcontent/{Type}/{ContentID}/{Title}/{Description}/{Show}/{IsPayPerView}/{PPVPrice}/{IsDeleted}/{Tags}/{Posting}", controllers.EditContent()).Methods("PUT")
	router.HandleFunc("/uploadmicro/v1/editcontent/{Type}/{ContentID}/{Show}/{IsPayPerView}/{PPVPrice}/{IsDeleted}/{Tags}", controllers.EditContentWithBody()).Methods("POST")
	router.HandleFunc("/uploadmicro/v2/editcontent/{Type}/{ContentID}/{Show}/{IsPayPerView}/{PPVPrice}/{IsDeleted}/{Tags}/{Visibility}", controllers.EditContentWithBodyV2()).Methods("POST")
	router.HandleFunc("/uploadmicro/v1/deletecontent/{ContentID}", controllers.DeleteContent()).Methods("DELETE")

	router.HandleFunc("/uploadmicro/v1/repostRequest", controllers.RepostRequest()).Methods("POST")              // implemented notifications
	router.HandleFunc("/uploadmicro/v1/approveRequest/{requestID}", controllers.ApproveRequest()).Methods("GET") // implemented notifications
	router.HandleFunc("/uploadmicro/v1/declineRequest/{requestID}", controllers.DeclineRequest()).Methods("GET")
	router.HandleFunc("/uploadmicro/v1/getFollowRequestsByUserID/{userID}/{limit}/{skip}", controllers.GetRepostRequestsByUserID()).Methods("GET")
	router.HandleFunc("/uploadmicro/v1/getMyFollowRequests/{userID}/{limit}/{skip}", controllers.GetMyRepostRequests()).Methods("GET")

	// STREAM
	router.HandleFunc("/uploadmicro/v1/startstream/{UserID}/{Title}/{Description}/{Show}/{IsPayPerView}/{PPVPrice}/{IsDeleted}/{Tags}/{Visibility}", controllers.StartStream()).Methods("POST")
	router.HandleFunc("/uploadmicro/v1/startstream/{UserID}/{Show}/{IsPayPerView}/{PPVPrice}/{IsDeleted}/{Tags}/{Visibility}", controllers.StartStreamWithBody()).Methods("POST")

	// NOTIFY WHEN STREAM STARTS | ENDS callback from nginx
	router.HandleFunc("/uploadmicro/v1/streamstarted", controllers.HandleStreamPublish()).Methods("POST")
	router.HandleFunc("/uploadmicro/v1/streamended", controllers.HandleStreamPublishDone()).Methods("POST")
	router.HandleFunc("/uploadmicro/v1/streamlookup", controllers.GetStreamUserID()).Methods("GET")
	



	// VIEW TRACKING
	router.HandleFunc("/uploadmicro/v1/stream/join/{MediaID}/{ViewerID}", controllers.StartView()).Methods("POST")
	router.HandleFunc("/uploadmicro/v1/stream/heartbeat/{MediaID}/{ViewerID}", controllers.ViewHeartbeat()).Methods("POST")
	router.HandleFunc("/uploadmicro/v1/stream/leave/{MediaID}/{ViewerID}", controllers.EndView()).Methods("POST")

	router.HandleFunc("/uploadmicro/v1/setInitialVisibility", controllers.SetInitialVisibility()).Methods("POST")
	router.HandleFunc("/uploadmicro/v1/setTranscodingStatus", controllers.SetTranscodingStatus()).Methods("GET")



	// MIGRATON
	router.HandleFunc("/uploadmicro/v1/upload-files", controllers.UploadMultipleFiles()).Methods("POST")

}
