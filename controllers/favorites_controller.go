package controllers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
	"upload-service/configs"
	"upload-service/models"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func AddContentToFavorites() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		vars := mux.Vars(r)
		userID := vars["UserID"]
		contentID := vars["ContentID"]
		album := vars["AlbumTitle"]

		log.Printf("AddContentToFavorites: userID=%s, contentID=%s, album=%s", userID, contentID, album)

		// Test database connection
		if err := configs.DB.Ping(ctx, nil); err != nil {
			log.Printf("AddContentToFavorites: database ping failed: %v", err)
			errorResponse(rw, fmt.Errorf("database connection failed"), 500)
			return
		}
		log.Printf("AddContentToFavorites: database connection OK")

		// Get collection dynamically to ensure it's properly initialized
		favoritesCollection := configs.GetCollection(configs.DB, "favorites")

		// Normalize album name - trim spaces and handle empty/default cases
		album = strings.TrimSpace(album)
		if album == "" {
			album = "favorites"
		}

		log.Printf("AddContentToFavorites: normalized album=%s", album)

		favorites := models.Favorite{}
		var err error

		res := favoritesCollection.FindOne(ctx, bson.M{"userid": userID})
		if err := res.Decode(&favorites); err != nil {
			if err != mongo.ErrNoDocuments {
				log.Printf("AddContentToFavorites: error finding user favorites: %v", err)
				errorResponse(rw, err, 500)
				return
			}
			// User doesn't exist, create new favorites document
			log.Printf("AddContentToFavorites: user doesn't exist, creating new favorites document")
			favorites.UserID = userID
			favorites.DateCreated = time.Now()
			var albums []models.Album
			albumObj := models.Album{
				Title:       album,
				DateCreated: time.Now(),
				Content:     []string{contentID},
			}
			albums = append(albums, albumObj)
			favorites.Albums = albums

			log.Printf("AddContentToFavorites: inserting favorites document: %+v", favorites)
			result, err := favoritesCollection.InsertOne(ctx, favorites)
			if err != nil {
				log.Printf("AddContentToFavorites: error inserting new favorites: %v", err)
				errorResponse(rw, err, 500)
				return
			}
			log.Printf("AddContentToFavorites: successfully created new favorites document with ID: %v", result.InsertedID)

			// Verify the insertion by reading it back
			var verifyDoc models.Favorite
			err = favoritesCollection.FindOne(ctx, bson.M{"_id": result.InsertedID}).Decode(&verifyDoc)
			if err != nil {
				log.Printf("AddContentToFavorites: verification failed - could not read back inserted document: %v", err)
			} else {
				log.Printf("AddContentToFavorites: verification successful - document found with %d albums", len(verifyDoc.Albums))
			}
			successResponse(rw, "Content added to favorites successfully")
			return
		}

		var index int
		var albumExists bool
		contentAlreadyExists := false

		log.Printf("AddContentToFavorites: user exists, checking %d albums", len(favorites.Albums))

		// Check if content already exists in any album
		for i, v := range favorites.Albums {
			log.Printf("AddContentToFavorites: checking album %d: title='%s'", i, v.Title)
			if strings.TrimSpace(v.Title) == album {
				index = i
				albumExists = true
				log.Printf("AddContentToFavorites: album exists at index %d", index)
			}
			for _, c := range v.Content {
				if c == contentID {
					contentAlreadyExists = true
					log.Printf("AddContentToFavorites: content already exists in album '%s'", v.Title)
					break
				}
			}
			if contentAlreadyExists {
				break
			}
		}

		if contentAlreadyExists {
			log.Printf("AddContentToFavorites: content already exists, returning success")
			successResponse(rw, "Content already exists in favorites")
			return
		}

		if albumExists {
			log.Printf("AddContentToFavorites: adding content to existing album at index %d", index)
			favorites.Albums[index].Content = append(favorites.Albums[index].Content, contentID)
		} else {
			log.Printf("AddContentToFavorites: creating new album '%s'", album)
			newAlbum := models.Album{
				Title:       album,
				DateCreated: time.Now(),
				Content:     []string{contentID},
			}
			favorites.Albums = append(favorites.Albums, newAlbum)
		}

		log.Printf("AddContentToFavorites: updating document with %d albums", len(favorites.Albums))
		log.Printf("AddContentToFavorites: updating favorites document: %+v", favorites)
		updateResult, err := favoritesCollection.UpdateOne(ctx, bson.M{"userid": userID}, bson.M{"$set": bson.M{"albums": favorites.Albums}})
		if err != nil {
			log.Printf("AddContentToFavorites: error updating favorites: %v", err)
			errorResponse(rw, err, 500)
			return
		}
		log.Printf("AddContentToFavorites: update result - modified count: %d, matched count: %d", updateResult.ModifiedCount, updateResult.MatchedCount)

		// Verify the update by reading it back
		var verifyUpdateDoc models.Favorite
		err = favoritesCollection.FindOne(ctx, bson.M{"userid": userID}).Decode(&verifyUpdateDoc)
		if err != nil {
			log.Printf("AddContentToFavorites: verification failed - could not read back updated document: %v", err)
		} else {
			log.Printf("AddContentToFavorites: verification successful - updated document has %d albums", len(verifyUpdateDoc.Albums))
			for i, album := range verifyUpdateDoc.Albums {
				log.Printf("AddContentToFavorites: album %d '%s' has %d content items: %v", i, album.Title, len(album.Content), album.Content)
			}
		}
		successResponse(rw, "Content added to favorites successfully")
	}
}

func RemoveContentFromFavorites() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		vars := mux.Vars(r)
		userID := vars["UserID"]
		contentID := vars["ContentID"]
		favorites := models.Favorite{}

		log.Printf("RemoveContentFromFavorites: userID=%s, contentID=%s", userID, contentID)

		favoritesCollection := configs.GetCollection(configs.DB, "favorites")
		res := favoritesCollection.FindOne(ctx, bson.M{"userid": userID})
		if err := res.Decode(&favorites); err != nil {
			if err == mongo.ErrNoDocuments {
				log.Printf("RemoveContentFromFavorites: user has no favorites document")
				errorResponse(rw, fmt.Errorf("user has no favorites or doesn't exist"), 404)
				return
			}
			log.Printf("RemoveContentFromFavorites: error decoding favorites: %v", err)
			errorResponse(rw, err, 500)
			return
		}

		log.Printf("RemoveContentFromFavorites: found favorites document with %d albums", len(favorites.Albums))

		contentFound := false
		modifiedAlbums := make([]models.Album, 0, len(favorites.Albums))

		for _, album := range favorites.Albums {
			log.Printf("RemoveContentFromFavorites: checking album '%s' with %d content items: %v", album.Title, len(album.Content), album.Content)

			// Create a new content slice without the content to remove
			var newContent []string
			foundInThisAlbum := false

			for _, content := range album.Content {
				if content == contentID {
					log.Printf("RemoveContentFromFavorites: found content in album '%s', removing it", album.Title)
					foundInThisAlbum = true
					contentFound = true
					// Skip this content item (don't add it to newContent)
				} else {
					newContent = append(newContent, content)
				}
			}

			// Only add the album if it still has content
			if len(newContent) > 0 {
				modifiedAlbum := models.Album{
					Title:       album.Title,
					DateCreated: album.DateCreated,
					Content:     newContent,
				}
				modifiedAlbums = append(modifiedAlbums, modifiedAlbum)
				log.Printf("RemoveContentFromFavorites: album '%s' now has %d content items: %v", album.Title, len(newContent), newContent)
			} else if foundInThisAlbum {
				log.Printf("RemoveContentFromFavorites: album '%s' is now empty, removing it", album.Title)
			} else if len(album.Content) > 0 {
				// Album has content but doesn't contain the content we're looking for
				modifiedAlbums = append(modifiedAlbums, album)
			}
		}

		if !contentFound {
			log.Printf("RemoveContentFromFavorites: content %s not found in any album", contentID)
			errorResponse(rw, fmt.Errorf("content not found in favorites"), 404)
			return
		}

		log.Printf("RemoveContentFromFavorites: after cleanup, %d albums remain", len(modifiedAlbums))

		// If no albums remain, delete the entire favorites document
		if len(modifiedAlbums) == 0 {
			log.Printf("RemoveContentFromFavorites: no albums remain, deleting entire favorites document")
			deleteResult, err := favoritesCollection.DeleteOne(ctx, bson.M{"userid": userID})
			if err != nil {
				log.Printf("RemoveContentFromFavorites: error deleting favorites document: %v", err)
				errorResponse(rw, err, 500)
				return
			}
			log.Printf("RemoveContentFromFavorites: delete result - deleted count: %d", deleteResult.DeletedCount)

			if deleteResult.DeletedCount == 0 {
				log.Printf("RemoveContentFromFavorites: WARNING - no documents were deleted")
				errorResponse(rw, fmt.Errorf("failed to delete favorites document"), 500)
				return
			}

			log.Printf("RemoveContentFromFavorites: successfully deleted favorites document - user now has no favorites")
		} else {
			// Update the document with the modified albums
			log.Printf("RemoveContentFromFavorites: updating favorites document with %d albums", len(modifiedAlbums))
			updateResult, err := favoritesCollection.UpdateOne(ctx, bson.M{"userid": userID}, bson.M{"$set": bson.M{"albums": modifiedAlbums}})
			if err != nil {
				log.Printf("RemoveContentFromFavorites: error updating document: %v", err)
				errorResponse(rw, err, 500)
				return
			}
			log.Printf("RemoveContentFromFavorites: update result - modified count: %d, matched count: %d", updateResult.ModifiedCount, updateResult.MatchedCount)

			if updateResult.ModifiedCount == 0 {
				log.Printf("RemoveContentFromFavorites: WARNING - no documents were modified")
				errorResponse(rw, fmt.Errorf("failed to update favorites - no documents modified"), 500)
				return
			}

			// Verify the update by reading it back
			var verifyDoc models.Favorite
			err = favoritesCollection.FindOne(ctx, bson.M{"userid": userID}).Decode(&verifyDoc)
			if err != nil {
				log.Printf("RemoveContentFromFavorites: verification failed - could not read back updated document: %v", err)
			} else {
				log.Printf("RemoveContentFromFavorites: verification successful - updated document has %d albums", len(verifyDoc.Albums))
				for i, album := range verifyDoc.Albums {
					log.Printf("RemoveContentFromFavorites: album %d '%s' has %d content items: %v", i, album.Title, len(album.Content), album.Content)
				}
			}
		}

		log.Printf("RemoveContentFromFavorites: successfully removed content")
		successResponse(rw, "Content removed from favorites successfully")
	}
}

func CreateNewAlbum() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		vars := mux.Vars(r)
		userID := vars["UserID"]
		album := vars["AlbumTitle"]
		favoritesCollection := configs.GetCollection(configs.DB, "favorites")
		result := favoritesCollection.FindOne(ctx, bson.M{"userid": userID})
		favorites := models.Favorite{}
		err := result.Decode(&favorites)
		if err != nil {
			if err != mongo.ErrNoDocuments {
				errorResponse(rw, err, 200)
				return
			}
			//if no favorites
			favorites.UserID = userID
			favorites.DateCreated = time.Now()
			//if a new album is added when there were no favorites before
			albumObj := models.Album{}
			albumObj.Title = album
			albumObj.DateCreated = time.Now()
			favorites.Albums = append(favorites.Albums, albumObj)
			_, err := favoritesCollection.InsertOne(ctx, favorites)
			if err != nil {
				errorResponse(rw, err, 200)
				return
			}
			successResponse(rw, "OK")
		} else {
			albumObj := models.Album{}
			albumObj.Title = album
			albumObj.DateCreated = time.Now()
			favorites.Albums = append(favorites.Albums, albumObj)
			updateRes, err := favoritesCollection.UpdateOne(ctx, bson.M{"_id": favorites.ID}, bson.M{"$set": bson.M{"albums": favorites.Albums}})
			if err != nil {
				errorResponse(rw, err, 200)
				return
			}
			successResponse(rw, updateRes.ModifiedCount)
		}
	}
}

func RemoveAlbum() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		vars := mux.Vars(r)
		userID := vars["UserID"]
		album := vars["AlbumTitle"]
		favoritesCollection := configs.GetCollection(configs.DB, "favorites")
		result := favoritesCollection.FindOne(ctx, bson.M{"userid": userID})
		favorites := models.Favorite{}
		err := result.Decode(&favorites)
		if err != nil {
			errorResponse(rw, err, 200)
			return
		}
		var shouldUpdate bool
		for i, v := range favorites.Albums {
			if v.Title == album {
				favorites.Albums = append(favorites.Albums[:i], favorites.Albums[i+1:]...)
				shouldUpdate = true
				break
			}
		}
		if shouldUpdate {
			res, err := favoritesCollection.UpdateOne(ctx, bson.M{"userid": userID}, bson.M{"$set": bson.M{"albums": favorites.Albums}})
			if err != nil {
				errorResponse(rw, err, 200)
				return
			}
			successResponse(rw, res.ModifiedCount)
		} else {
			errorResponse(rw, fmt.Errorf("couldn't find album named:"+album), 200)
		}
	}
}

func MoveFavorite() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		vars := mux.Vars(r)
		userID := vars["UserID"]
		fromAblum := vars["FromAblum"]
		toAlbum := vars["ToAlbum"]
		contentID := vars["ContentID"]
		favorites := models.Favorite{}
		var err error
		favoritesCollection := configs.GetCollection(configs.DB, "favorites")
		res := favoritesCollection.FindOne(ctx, bson.M{"userid": userID})
		if err := res.Decode(&favorites); err != nil {
			if err != mongo.ErrNoDocuments {
				errorResponse(rw, err, 200)
				return
			}
		}
		added := false
		removed := false
		for k, v := range favorites.Albums {
			if v.Title == toAlbum {
				favorites.Albums[k].Content = append(favorites.Albums[k].Content, contentID)
				added = true
				continue
			}
			if v.Title == fromAblum {
				for j, content := range v.Content {
					if content == contentID {
						favorites.Albums[k].Content = append(favorites.Albums[k].Content[:j], favorites.Albums[k].Content[j+1:]...)
						removed = true
						continue
					}
				}
			}
		}
		if !added {
			errorResponse(rw, fmt.Errorf("didn't add the content to: ["+toAlbum+"] maybe the album doesn't exist"), 200)
			return
		}
		if !removed {
			errorResponse(rw, fmt.Errorf("didn't remove the content from: ["+fromAblum+"] maybe the content or the album doesn't exist"), 200)
			return
		}
		_, err = favoritesCollection.UpdateOne(ctx, bson.M{"userid": userID}, bson.M{"$set": bson.M{"albums": favorites.Albums}})
		if err != nil {
			errorResponse(rw, err, 200)
			return
		}
		successResponse(rw, "Content moved successfully")
	}
}

func GetUserFavorites() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		vars := mux.Vars(r)
		userID := vars["UserID"]

		favorites := models.Favorite{}

		favoritesCollection := configs.GetCollection(configs.DB, "favorites")
		res := favoritesCollection.FindOne(ctx, bson.M{"userid": userID})
		if err := res.Decode(&favorites); err != nil {
			if err == mongo.ErrNoDocuments {
				// User has no favorites, return empty response
				successResponse(rw, map[string]interface{}{
					"favorites": map[string]interface{}{
						"userID": userID,
						"albums": []models.Album{},
					},
				})
				return
			}
			errorResponse(rw, err, 500)
			return
		}

		successResponse(rw, map[string]interface{}{
			"favorites": favorites,
		})
	}
}
