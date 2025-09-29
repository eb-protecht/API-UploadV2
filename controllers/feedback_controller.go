package controllers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
	"upload-service/configs"
	"upload-service/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var feedbackCollection *mongo.Collection = configs.GetCollection(configs.DB, "feedback")

// SubmitFeedback handles POST requests to submit feedback
func SubmitFeedback() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		var feedbackRequest models.FeedbackRequest
		if err := json.NewDecoder(r.Body).Decode(&feedbackRequest); err != nil {
			errorResponse(rw, err, http.StatusBadRequest)
			return
		}

		// Validate that at least email or phone is provided
		if feedbackRequest.Email == "" && feedbackRequest.Phone == "" {
			errorResponse(rw, &json.SyntaxError{}, http.StatusBadRequest)
			return
		}

		// Validate that comment is not empty
		if feedbackRequest.Comment == "" {
			errorResponse(rw, &json.SyntaxError{}, http.StatusBadRequest)
			return
		}

		feedback := models.Feedback{
			Email:       feedbackRequest.Email,
			Phone:       feedbackRequest.Phone,
			Comment:     feedbackRequest.Comment,
			DateCreated: time.Now(),
		}

		result, err := feedbackCollection.InsertOne(ctx, feedback)
		if err != nil {
			errorResponse(rw, err, http.StatusInternalServerError)
			return
		}

		successResponse(rw, result.InsertedID)
	}
}

// GetFeedback handles GET requests to retrieve all feedback
func GetFeedback() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Set up options for sorting by date created (newest first)
		findOptions := options.Find()
		findOptions.SetSort(bson.M{"date_created": -1})

		cursor, err := feedbackCollection.Find(ctx, bson.M{}, findOptions)
		if err != nil {
			errorResponse(rw, err, http.StatusInternalServerError)
			return
		}
		defer cursor.Close(ctx)

		var feedback []models.Feedback
		if err = cursor.All(ctx, &feedback); err != nil {
			errorResponse(rw, err, http.StatusInternalServerError)
			return
		}

		// Ensure we always return an empty array instead of null
		if feedback == nil {
			feedback = []models.Feedback{}
		}

		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(feedback)
	}
}
