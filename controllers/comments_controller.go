package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
	"upload-service/configs"
	"upload-service/models"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

var commentsCollection *mongo.Collection = configs.GetCollection(configs.DB, "comments")

func AddComment() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		vars := mux.Vars(r)
		userID := vars["UserID"]
		replyTo := vars["ReplyTo"]
		commentTxtEncoded := vars["Comment"]
		commentTxtDecoded, _ := url.QueryUnescape(commentTxtEncoded)

		replyToComment, err := strconv.ParseBool(vars["ReplyToComment"])
		if err != nil {
			errorResponse(rw, err, 200)
			return
		}
		comment := models.Comment{
			UserID:         userID,
			ReplyTo:        replyTo,
			Comment:        commentTxtDecoded,
			ReplyToComment: replyToComment,
			DateCreated:    time.Now(),
		}

		res, err := commentsCollection.InsertOne(ctx, comment)
		if err != nil {
			errorResponse(rw, err, 200)
			return
		}
		oCID, err := primitive.ObjectIDFromHex(replyTo)
		if err != nil {
			fmt.Println("couldn't get content from ", replyTo)
			successResponse(rw, res.InsertedID)
			return
		}
		content := models.Content{}
		err = contentCollection.FindOne(ctx, bson.M{"_id": oCID}).Decode(&content)
		if err != nil {
			fmt.Println("couldn't get decode content from ", oCID)
			successResponse(rw, res.InsertedID)
			return
		}
		sendNotificationWithData(content.UserID, userID, "commented in your post: "+commentTxtDecoded, replyTo, models.CommentNotification, ctx)
		successResponse(rw, res.InsertedID)
	}
}

func AddCommentWithBody() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		vars := mux.Vars(r)
		userID := vars["UserID"]
		replyTo := vars["ReplyTo"]

		commentBody := models.CommentBody{}

		err := json.NewDecoder(r.Body).Decode(&commentBody)
		if err != nil {
			errorResponse(rw, fmt.Errorf("bad request"), 200)
			return
		}
		commentTxtEncoded := commentBody.Comment

		commentTxtDecoded, _ := url.QueryUnescape(commentTxtEncoded)

		replyToComment, err := strconv.ParseBool(vars["ReplyToComment"])
		if err != nil {
			errorResponse(rw, err, 200)
			return
		}
		comment := models.Comment{
			UserID:         userID,
			ReplyTo:        replyTo,
			Comment:        commentTxtDecoded,
			ReplyToComment: replyToComment,
			DateCreated:    time.Now(),
		}

		res, err := commentsCollection.InsertOne(ctx, comment)
		if err != nil {
			errorResponse(rw, err, 200)
			return
		}
		oCID, err := primitive.ObjectIDFromHex(replyTo)
		if err != nil {
			fmt.Println("couldn't get content from ", replyTo)
			successResponse(rw, res.InsertedID)
			return
		}
		content := models.Content{}
		err = contentCollection.FindOne(ctx, bson.M{"_id": oCID}).Decode(&content)
		if err != nil {
			fmt.Println("couldn't get decode content from ", oCID)
			successResponse(rw, res.InsertedID)
			return
		}
		sendNotificationWithData(content.UserID, userID, "commented in your post: "+commentTxtDecoded, replyTo, models.CommentNotification, ctx)
		successResponse(rw, res.InsertedID)
	}
}

func AddCommentWithBodyWithOtherUserID() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		vars := mux.Vars(r)
		userID := vars["UserID"]
		ownerUserID := vars["OwnerUserID"]
		contentID := vars["ContentID"]
		replyTo := vars["ReplyTo"]

		isReply, err := strconv.ParseBool(vars["IsReply"])
		if err != nil {
			errorResponse(rw, fmt.Errorf("invalid IsReply value: %v", err), http.StatusBadRequest)
			return
		}

		var commentBody models.CommentBody
		if err := json.NewDecoder(r.Body).Decode(&commentBody); err != nil {
			errorResponse(rw, fmt.Errorf("invalid JSON body: %v", err), http.StatusBadRequest)
			return
		}

		commentTxtDecoded, err := url.QueryUnescape(commentBody.Comment)
		if err != nil {
			errorResponse(rw, fmt.Errorf("failed to unescape comment text: %v", err), http.StatusBadRequest)
			return
		}

		comment := models.Comment{
			UserID:         userID,
			ReplyTo:        replyTo,
			Comment:        commentTxtDecoded,
			ReplyToComment: isReply,
			ContentID:      contentID,
			DateCreated:    time.Now(),
		}

		insertRes, err := commentsCollection.InsertOne(ctx, comment)
		if err != nil {
			errorResponse(rw, fmt.Errorf("failed to insert comment: %v", err), http.StatusInternalServerError)
			return
		}

		notifyOnComment(ctx, isReply, userID, ownerUserID, contentID, commentTxtDecoded)

		successResponse(rw, insertRes.InsertedID)
	}
}

// notifyOnComment handles the difference between a direct comment on content vs a reply to another comment.
// Splitting this logic out makes AddCommentWithBodyWithOtherUserID shorter and easier to read.
func notifyOnComment(
	ctx context.Context,
	isReply bool,
	userID, ownerUserID, contentID, commentTxtDecoded string,
) {
	if !isReply {
		// Notify content owner
		sendNotificationWithData(
			ownerUserID,
			userID,
			"commented in your post: "+commentTxtDecoded,
			contentID,
			models.CommentNotification,
			ctx,
		)
	} else {
		// A reply => we notify the owner of the original comment
		sendNotificationWithData(
			ownerUserID,
			userID,
			"replied to your comment: "+commentTxtDecoded,
			contentID,
			models.ReplyNotification,
			ctx,
		)
	}
}

func BackfillComments() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		cursor, err := commentsCollection.Find(ctx, bson.M{})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer cursor.Close(ctx)

		commentsByID := make(map[string]*models.Comment)

		failingIndex := 0
		for cursor.Next(ctx) {
			var c models.Comment
			if err := cursor.Decode(&c); err != nil {
				fmt.Println("failed to decode", err, failingIndex)
				failingIndex++
				continue
			}
			commentsByID[c.ID.Hex()] = &c
		}

		for _, c := range commentsByID {
			c.ContentID = findUltimateContentID(c, commentsByID, 0)
		}

		var writeModels []mongo.WriteModel
		for _, c := range commentsByID {
			filter := bson.M{"_id": c.ID}
			update := bson.M{"$set": bson.M{"contentid": c.ContentID}}
			writeModels = append(writeModels,
				mongo.NewUpdateOneModel().SetFilter(filter).SetUpdate(update))
		}

		if len(writeModels) > 0 {
			_, err := commentsCollection.BulkWrite(ctx, writeModels)
			if err != nil {
				fmt.Println("couldn't bulkWrite", err)
			}
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Backfill complete!"))
	}
}

func findUltimateContentID(
	c *models.Comment,
	commentsByID map[string]*models.Comment,
	depth int,
) string {
	if _, isComment := commentsByID[c.ReplyTo]; !isComment {
		return c.ReplyTo
	}

	parent := commentsByID[c.ReplyTo]
	return findUltimateContentID(parent, commentsByID, depth+1)
}

func EditComment() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		vars := mux.Vars(r)
		commentID := vars["CommentID"]
		commentTxtEncoded := vars["Comment"]
		commentTxtDecoded, _ := url.QueryUnescape(commentTxtEncoded)
		oID, err := primitive.ObjectIDFromHex(commentID)
		if err != nil {
			errorResponse(rw, err, 200)
			return
		}
		filter := bson.M{"_id": oID}
		update := bson.M{"$set": bson.M{"comment": commentTxtDecoded}}
		err = commentsCollection.FindOneAndUpdate(ctx, filter, update).Err()
		if err != nil {
			errorResponse(rw, err, 200)
			return
		}
		successResponse(rw, "Updated")
	}
}

func EditCommentWithBody() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		vars := mux.Vars(r)
		commentID := vars["CommentID"]
		commentBody := models.CommentBody{}

		err := json.NewDecoder(r.Body).Decode(&commentBody)
		if err != nil {
			errorResponse(rw, fmt.Errorf("bad request"), 200)
			return
		}
		commentTxt := commentBody.Comment

		oID, err := primitive.ObjectIDFromHex(commentID)
		if err != nil {
			errorResponse(rw, err, 200)
			return
		}
		filter := bson.M{"_id": oID}
		update := bson.M{"$set": bson.M{"comment": commentTxt}}
		err = commentsCollection.FindOneAndUpdate(ctx, filter, update).Err()
		if err != nil {
			errorResponse(rw, err, 200)
			return
		}
		successResponse(rw, "Updated")
	}
}

func DeleteComment() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		vars := mux.Vars(r)
		commentID := vars["CommentID"]
		commentOID, err := primitive.ObjectIDFromHex(commentID)
		if err != nil {
			errorResponse(rw, err, 200)
			return
		}
		err = commentsCollection.FindOneAndUpdate(ctx, bson.M{"_id": commentOID}, bson.M{"$set": bson.M{"isdeleted": true}}).Err()
		if err != nil {
			errorResponse(rw, err, 200)
			return
		}
		successResponse(rw, "Deleted")
	}
}
