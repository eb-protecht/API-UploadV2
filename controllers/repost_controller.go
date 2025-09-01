package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
	"upload-service/configs"
	"upload-service/models"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	repostRequestCollection *mongo.Collection = configs.GetCollection(configs.DB, "repost_requests")
	//usersCollection *mongo.Collection = configs.GetCollection(configs.DB,"users")
)

const (
	STATUS_PENDING  = "pending"
	STATUS_ACCEPTED = "accepted"
	STATUS_DECLINED = "decline"
)

func RepostRequest() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		repostRequest := models.RepostRequest{}

		err := json.NewDecoder(r.Body).Decode(&repostRequest)
		if err != nil {
			errorResponse(w, fmt.Errorf("couldn't decode body"), 200)
			return
		}
		if repostRequest.RepostRequest == repostRequest.RequestTo {
			errorResponse(w, fmt.Errorf("invalid repost request"), 200)
			return
		}
		response := struct {
			Action string
			Result interface{}
		}{}

		//checking settings for the default repost action
		userObj := models.User{}
		ownerID := repostRequest.Content.UserID

		oID, err := primitive.ObjectIDFromHex(ownerID)
		if err != nil {
			errorResponse(w, fmt.Errorf("invalid object id"), 200)
			return
		}
		err = usersCollection.FindOne(ctx, bson.M{"_id": oID}).Decode(&userObj)
		if err != nil {
			errorResponse(w, fmt.Errorf("couldn't find/decode user"), 200)
			return
		}
		if userObj.MySettings.RepostRequestAction == "Approve" {
			contentOID, err := primitive.ObjectIDFromHex(repostRequest.ContentID)
			if err != nil {
				errorResponse(w, err, 200)
				return
			}
			contentResult := contentCollection.FindOne(ctx, bson.M{"_id": contentOID})
			content := models.Content{}
			err = contentResult.Decode(&content)
			if err != nil {
				errorResponse(w, err, 200)
				return
			}
			content.OriginalID = repostRequest.ContentID
			content.Poster = repostRequest.RepostRequest
			content.DateCreated = time.Now()
			content.Id = primitive.NilObjectID
			contentRes, err := contentCollection.InsertOne(ctx, content)
			if err != nil {
				errorResponse(w, fmt.Errorf("couldn't insert into content"), 200)
				return
			}
			response.Result = contentRes
			repostRequest.Status = STATUS_ACCEPTED
		} else {
			sendNotificationWithData(userObj.UserID, repostRequest.RepostRequest, "requested to repost", repostRequest.ContentID, models.SharePostRequestNotification, ctx)
			repostRequest.Status = STATUS_PENDING
		}
		repostRequest.CreatedAt = time.Now()
		repostRequest.UpdatedAt = time.Now()
		found := repostRequestCollection.FindOne(ctx, bson.M{"repostRequest": repostRequest.RepostRequest, "contentid": repostRequest.ContentID})
		exists := models.RepostRequest{}
		err = found.Decode(&exists)

		if err != nil {
			if err == mongo.ErrNoDocuments {
				res, err := repostRequestCollection.InsertOne(ctx, repostRequest)
				if err != nil {
					fmt.Println(err)
					errorResponse(w, fmt.Errorf("couldn't insert into repost requests"), 200)
					return
				}
				response.Action = "Repost Request"
				response.Result = res
				successResponse(w, response)
				return
			}
			errorResponse(w, fmt.Errorf("something went wrong"), 200)
			return
		}
		//if it exists delete repost request
		deleteResult, err := repostRequestCollection.DeleteOne(ctx, bson.M{"repostRequest": repostRequest.RepostRequest, "requestTo": repostRequest.RequestTo})
		if err != nil {
			errorResponse(w, fmt.Errorf("couldn't remove repost request"), 200)
			return
		}
		response.Action = "Repost Request Removed"
		response.Result = deleteResult
		successResponse(w, response)
	}
}

func ApproveRequest() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		vars := mux.Vars(r)
		requestID := vars["requestID"]

		oID, err := primitive.ObjectIDFromHex(requestID)
		if err != nil {
			errorResponse(w, fmt.Errorf("invalid object id"), 200)
			return
		}
		filter := bson.M{"_id": oID}
		res := repostRequestCollection.FindOne(ctx, filter)
		request := models.RepostRequest{}
		err = res.Decode(&request)
		if err != nil {
			errorResponse(w, err, 200)
			return
		}
		contentOID, err := primitive.ObjectIDFromHex(request.ContentID)
		if err != nil {
			errorResponse(w, err, 200)
			return
		}
		contentResult := contentCollection.FindOne(ctx, bson.M{"_id": contentOID})
		content := models.Content{}
		err = contentResult.Decode(&content)
		if err != nil {
			errorResponse(w, err, 200)
			return
		}
		content.Poster = request.RepostRequest
		content.DateCreated = time.Now()
		content.Id = primitive.NilObjectID
		contentRes, err := contentCollection.InsertOne(ctx, content)
		if err != nil {
			errorResponse(w, fmt.Errorf("couldn't insert into content"), 200)
			return
		}
		_, err = repostRequestCollection.UpdateOne(ctx, filter, bson.M{"$set": bson.M{"status": STATUS_ACCEPTED}})
		if err != nil {
			contentCollection.DeleteOne(ctx, bson.M{"_id": contentRes.InsertedID})
			errorResponse(w, fmt.Errorf("failed to accept request"), 200)
			return
		}
		response := struct {
			Action string
			Result interface{}
		}{}
		response.Action = "Repost Request Accepted"
		response.Result = contentRes
		sendNotificationWithData(request.RepostRequest, content.UserID, "accepted your repost request", request.ContentID, models.SharePostRequestAcceptNotification, ctx)
		successResponse(w, response)
	}
}

func DeclineRequest() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		vars := mux.Vars(r)
		requestID := vars["requestID"]

		oID, err := primitive.ObjectIDFromHex(requestID)
		if err != nil {
			errorResponse(w, fmt.Errorf("invalid object id"), 200)
			return
		}
		filter := bson.M{"_id": oID}
		res, err := repostRequestCollection.UpdateOne(ctx, filter, bson.M{"$set": bson.M{"status": STATUS_DECLINED}})
		if err != nil {
			errorResponse(w, fmt.Errorf("couldn't decline the request"), 200)
			return
		}

		response := struct {
			Action string
			Result interface{}
		}{}
		response.Action = "Repost Request Declined Successfully "
		response.Result = res.ModifiedCount
		successResponse(w, response)
	}
}

func GetRepostRequestsByUserID() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		vars := mux.Vars(r)
		responder := vars["userID"]

		limit, err := strconv.ParseInt(vars["limit"], 10, 64)
		if err != nil {
			fmt.Println("limit not an int")
			errorResponse(w, err, 400)
			return
		}
		skip, err := strconv.ParseInt(vars["skip"], 10, 64)
		if err != nil {
			fmt.Println("skip not an int")
			errorResponse(w, err, 400)
			return
		}

		paginate := options.Find()
		paginate.SetSkip(skip)
		paginate.SetLimit(limit)
		filter := bson.M{"requestTo": responder, "status": STATUS_PENDING}

		cur, err := repostRequestCollection.Find(ctx, filter, paginate)
		if err != nil {
			errorResponse(w, fmt.Errorf("decoding db results"), 200)
			return
		}
		requests := []models.RepostRequest{}
		if err := cur.All(ctx, &requests); err != nil {
			errorResponse(w, fmt.Errorf("error parsing results from db"), 200)
			return
		}
		var result []interface{}
		for _, v := range requests {
			contentOID, err := primitive.ObjectIDFromHex(v.ContentID)
			if err != nil {
				continue
			}
			singleRes := contentCollection.FindOne(ctx, bson.M{"_id": contentOID})
			var content models.Content
			if err = singleRes.Decode(&content); err != nil {
				continue
			}
			v.Content = content
			result = append(result, v)
		}
		successResponse(w, result)
	}
}

func GetMyRepostRequests() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		vars := mux.Vars(r)
		requester := vars["userID"]

		limit, err := strconv.ParseInt(vars["limit"], 10, 64)
		if err != nil {
			fmt.Println("limit not an int")
			errorResponse(w, err, 400)
			return
		}
		skip, err := strconv.ParseInt(vars["skip"], 10, 64)
		if err != nil {
			fmt.Println("skip not an int")
			errorResponse(w, err, 400)
			return
		}

		paginate := options.Find()
		paginate.SetSkip(skip)
		paginate.SetLimit(limit)
		filter := bson.M{"repostRequest": requester, "status": STATUS_PENDING}

		cur, err := repostRequestCollection.Find(ctx, filter, paginate)
		if err != nil {
			errorResponse(w, fmt.Errorf("decoding db results"), 200)
			return
		}

		requests := []models.RepostRequest{}
		if err := cur.All(ctx, &requests); err != nil {
			errorResponse(w, fmt.Errorf("error parsing results from db"), 200)
			return
		}
		var result []interface{}
		for _, v := range requests {
			contentOID, err := primitive.ObjectIDFromHex(v.ContentID)
			if err != nil {
				continue
			}
			singleRes := contentCollection.FindOne(ctx, bson.M{"_id": contentOID})
			var content models.Content
			if err = singleRes.Decode(&content); err != nil {
				continue
			}
			v.Content = content
			result = append(result, v)
		}
		successResponse(w, result)
	}
}
