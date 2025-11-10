package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
	"upload-service/configs"
	"upload-service/models"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func AddContentToFavorites() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		
		vars := mux.Vars(r)
		userID := vars["UserID"]
		contentID := vars["ContentID"]

		/**
		If album id is not sent, we create a favorite album and add the content to it.
		*/
		albumID := vars["AlbumID"]
		
		log.Printf("AddContentToFavorites: userID=%s, contentID=%s, albumID=%s", userID, contentID, albumID)
		
		// Get collections
		albumsCollection := configs.GetCollection(configs.DB, "albums")
		favoritesCollection := configs.GetCollection(configs.DB, "favorites")
		
		if albumID == "" {
			log.Printf("AddContentToFavorites: no albumID provided, looking for default 'favorites' album")
			
			var defaultAlbum models.Album
			err := albumsCollection.FindOne(ctx, bson.M{
				"userID": userID,
				"title":  "favorites",
			}).Decode(&defaultAlbum)
			
			if err == mongo.ErrNoDocuments {
				// Create default "favorites" album
				log.Printf("AddContentToFavorites: creating default 'favorites' album")
				defaultAlbum = models.Album{
					UserID:       userID,
					Title:        "favorites",
					DateCreated:  time.Now(),
					DateModified: time.Now(),
				}
				
				result, err := albumsCollection.InsertOne(ctx, defaultAlbum)
				if err != nil {
					log.Printf("AddContentToFavorites: error creating default album: %v", err)
					errorResponse(rw, err, 500)
					return
				}
				defaultAlbum.ID = result.InsertedID.(primitive.ObjectID)
				log.Printf("AddContentToFavorites: created default album with ID: %s", defaultAlbum.ID.Hex())
			} else if err != nil {
				log.Printf("AddContentToFavorites: error finding default album: %v", err)
				errorResponse(rw, err, 500)
				return
			}
			
			albumID = defaultAlbum.ID.Hex()
			log.Printf("AddContentToFavorites: using album ID: %s", albumID)
		}
		
		// Verify album exists and belongs to user
		albumObjectID, err := primitive.ObjectIDFromHex(albumID)
		if err != nil {
			log.Printf("AddContentToFavorites: invalid album ID format: %v", err)
			errorResponse(rw, fmt.Errorf("invalid album ID"), 400)
			return
		}
		
		var album models.Album
		err = albumsCollection.FindOne(ctx, bson.M{
			"_id":    albumObjectID,
			"userID": userID,
		}).Decode(&album)
		
		if err == mongo.ErrNoDocuments {
			log.Printf("AddContentToFavorites: album not found or doesn't belong to user")
			errorResponse(rw, fmt.Errorf("album not found"), 404)
			return
		}
		if err != nil {
			log.Printf("AddContentToFavorites: error finding album: %v", err)
			errorResponse(rw, err, 500)
			return
		}
		
		// Check if favorite already exists
		var existingFavorite models.Favorite
		err = favoritesCollection.FindOne(ctx, bson.M{
			"userID":    userID,
			"contentID": contentID,
			"albumID":   albumID,
		}).Decode(&existingFavorite)
		
		if err == nil {
			log.Printf("AddContentToFavorites: content already exists in this album")
			successResponse(rw, "Content already exists in favorites")
			return
		}
		
		// Create new favorite
		newFavorite := models.Favorite{
			UserID:    userID,
			ContentID: contentID,
			AlbumID:   albumID,
			DateAdded: time.Now(),
		}
		
		result, err := favoritesCollection.InsertOne(ctx, newFavorite)
		if err != nil {
			log.Printf("AddContentToFavorites: error inserting favorite: %v", err)
			errorResponse(rw, err, 500)
			return
		}
		
		log.Printf("AddContentToFavorites: successfully added favorite with ID: %v", result.InsertedID)
		
		_, err = albumsCollection.UpdateOne(ctx,
			bson.M{"_id": albumObjectID},
			bson.M{"$set": bson.M{"dateModified": time.Now()}},
		)
		if err != nil {
			log.Printf("AddContentToFavorites: warning - could not update album dateModified: %v", err)
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
		albumID := vars["AlbumID"] // Required now
		
		log.Printf("RemoveContentFromFavorites: userID=%s, contentID=%s, albumID=%s", userID, contentID, albumID)
		
		if albumID == "" {
			log.Printf("RemoveContentFromFavorites: albumID is required")
			errorResponse(rw, fmt.Errorf("albumID is required"), 400)
			return
		}
		
		favoritesCollection := configs.GetCollection(configs.DB, "favorites")
		albumsCollection := configs.GetCollection(configs.DB, "albums")
		
		// Verify album exists and belongs to user
		albumObjectID, err := primitive.ObjectIDFromHex(albumID)
		if err != nil {
			log.Printf("RemoveContentFromFavorites: invalid albumID format: %v", err)
			errorResponse(rw, fmt.Errorf("invalid albumID"), 400)
			return
		}
		
		var album models.Album
		err = albumsCollection.FindOne(ctx, bson.M{
			"_id":    albumObjectID,
			"userID": userID,
		}).Decode(&album)
		
		if err == mongo.ErrNoDocuments {
			log.Printf("RemoveContentFromFavorites: album not found or doesn't belong to user")
			errorResponse(rw, fmt.Errorf("album not found"), 404)
			return
		}
		if err != nil {
			log.Printf("RemoveContentFromFavorites: error finding album: %v", err)
			errorResponse(rw, err, 500)
			return
		}
		
		// Delete the favorite document
		filter := bson.M{
			"userID":    userID,
			"contentID": contentID,
			"albumID":   albumID,
		}
		
		deleteResult, err := favoritesCollection.DeleteOne(ctx, filter)
		if err != nil {
			log.Printf("RemoveContentFromFavorites: error deleting favorite: %v", err)
			errorResponse(rw, err, 500)
			return
		}
		
		if deleteResult.DeletedCount == 0 {
			log.Printf("RemoveContentFromFavorites: favorite not found")
			errorResponse(rw, fmt.Errorf("content not found in this album"), 404)
			return
		}
		
		log.Printf("RemoveContentFromFavorites: successfully deleted favorite")
		
		// Update album's dateModified
		_, err = albumsCollection.UpdateOne(ctx,
			bson.M{"_id": albumObjectID},
			bson.M{"$set": bson.M{"dateModified": time.Now()}},
		)
		if err != nil {
			log.Printf("RemoveContentFromFavorites: warning - could not update album dateModified: %v", err)
		}
		
		successResponse(rw, "Content removed from favorites successfully")
	}
}

type CreateAlbumRequest struct {
	Title       string `json:"title" validate:"required"`
	Description string `json:"description,omitempty"` // Optional
}

func CreateNewAlbum() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		
		vars := mux.Vars(r)
		userID := vars["UserID"]
		
		log.Printf("CreateNewAlbum: userID=%s", userID)
		
		// Parse request body
		var req CreateAlbumRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Printf("CreateNewAlbum: invalid request body: %v", err)
			errorResponse(rw, fmt.Errorf("invalid request body"), 400)
			return
		}
		
		// Validate title
		req.Title = strings.TrimSpace(req.Title)
		if req.Title == "" {
			log.Printf("CreateNewAlbum: album title is required")
			errorResponse(rw, fmt.Errorf("album title is required"), 400)
			return
		}
		
		albumsCollection := configs.GetCollection(configs.DB, "albums")
		
		// Check if album with same title already exists for this user
		var existingAlbum models.Album
		err := albumsCollection.FindOne(ctx, bson.M{
			"userID": userID,
			"title":  req.Title,
		}).Decode(&existingAlbum)
		
		if err == nil {
			// Album already exists
			log.Printf("CreateNewAlbum: album with title '%s' already exists", req.Title)
			errorResponse(rw, fmt.Errorf("album with this title already exists"), 409)
			return
		}
		
		if err != mongo.ErrNoDocuments {
			log.Printf("CreateNewAlbum: error checking existing album: %v", err)
			errorResponse(rw, err, 500)
			return
		}
		
		// Create new album
		newAlbum := models.Album{
			UserID:       userID,
			Title:        req.Title,
			Description:  req.Description,
			DateCreated:  time.Now(),
			DateModified: time.Now(),
		}
		
		result, err := albumsCollection.InsertOne(ctx, newAlbum)
		if err != nil {
			log.Printf("CreateNewAlbum: error inserting album: %v", err)
			errorResponse(rw, err, 500)
			return
		}
		
		newAlbum.ID = result.InsertedID.(primitive.ObjectID)
		
		log.Printf("CreateNewAlbum: successfully created album with ID: %s", newAlbum.ID.Hex())
		
		// Return the created album with its ID
		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusCreated)
		json.NewEncoder(rw).Encode(map[string]interface{}{
			"status":  201,
			"message": "Album created successfully",
			"data": map[string]interface{}{
				"album": newAlbum,
			},
		})
	}
}

func RemoveAlbum() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		
		vars := mux.Vars(r)
		userID := vars["UserID"]
		albumID := vars["AlbumID"]
		
		log.Printf("RemoveAlbum: userID=%s, albumID=%s", userID, albumID)
		
		// Validate albumID format
		albumObjectID, err := primitive.ObjectIDFromHex(albumID)
		if err != nil {
			log.Printf("RemoveAlbum: invalid albumID format: %v", err)
			errorResponse(rw, fmt.Errorf("invalid album ID"), 400)
			return
		}
		
		albumsCollection := configs.GetCollection(configs.DB, "albums")
		favoritesCollection := configs.GetCollection(configs.DB, "favorites")
		
		// 1. Verify album exists and belongs to user
		var album models.Album
		err = albumsCollection.FindOne(ctx, bson.M{
			"_id":    albumObjectID,
			"userID": userID,
		}).Decode(&album)
		
		if err == mongo.ErrNoDocuments {
			log.Printf("RemoveAlbum: album not found or doesn't belong to user")
			errorResponse(rw, fmt.Errorf("album not found"), 404)
			return
		}
		if err != nil {
			log.Printf("RemoveAlbum: error finding album: %v", err)
			errorResponse(rw, err, 500)
			return
		}
		
		// 2. Delete all favorites in this album first
		deleteResult, err := favoritesCollection.DeleteMany(ctx, bson.M{
			"userID":  userID,
			"albumID": albumID,
		})
		if err != nil {
			log.Printf("RemoveAlbum: error deleting favorites from album: %v", err)
			errorResponse(rw, err, 500)
			return
		}
		
		log.Printf("RemoveAlbum: deleted %d favorites from album", deleteResult.DeletedCount)
		
		// 3. Delete the album itself
		albumDeleteResult, err := albumsCollection.DeleteOne(ctx, bson.M{
			"_id":    albumObjectID,
			"userID": userID,
		})
		if err != nil {
			log.Printf("RemoveAlbum: error deleting album: %v", err)
			errorResponse(rw, err, 500)
			return
		}
		
		if albumDeleteResult.DeletedCount == 0 {
			log.Printf("RemoveAlbum: album was not deleted (possible race condition)")
			errorResponse(rw, fmt.Errorf("failed to delete album"), 500)
			return
		}
		
		log.Printf("RemoveAlbum: successfully deleted album '%s' and %d favorites", album.Title, deleteResult.DeletedCount)
		
		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(map[string]interface{}{
			"status":  200,
			"message": fmt.Sprintf("Album '%s' and %d favorites deleted successfully", album.Title, deleteResult.DeletedCount),
			"data": map[string]interface{}{
				"albumTitle":       album.Title,
				"favoritesDeleted": deleteResult.DeletedCount,
			},
		})
	}
}


func MoveFavorite() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		
		vars := mux.Vars(r)
		userID := vars["UserID"]
		contentID := vars["ContentID"]
		fromAlbumID := vars["FromAlbumID"]
		toAlbumID := vars["ToAlbumID"]
		
		log.Printf("MoveFavorite: userID=%s, contentID=%s, from=%s, to=%s", userID, contentID, fromAlbumID, toAlbumID)
		
		// Validate inputs
		if fromAlbumID == toAlbumID {
			log.Printf("MoveFavorite: source and destination albums are the same")
			errorResponse(rw, fmt.Errorf("source and destination albums cannot be the same"), 400)
			return
		}
		
		// Validate album IDs format
		fromAlbumObjectID, err := primitive.ObjectIDFromHex(fromAlbumID)
		if err != nil {
			log.Printf("MoveFavorite: invalid fromAlbumID format: %v", err)
			errorResponse(rw, fmt.Errorf("invalid source album ID"), 400)
			return
		}
		
		toAlbumObjectID, err := primitive.ObjectIDFromHex(toAlbumID)
		if err != nil {
			log.Printf("MoveFavorite: invalid toAlbumID format: %v", err)
			errorResponse(rw, fmt.Errorf("invalid destination album ID"), 400)
			return
		}
		
		albumsCollection := configs.GetCollection(configs.DB, "albums")
		favoritesCollection := configs.GetCollection(configs.DB, "favorites")
		

		var fromAlbum models.Album
		err = albumsCollection.FindOne(ctx, bson.M{
			"_id":    fromAlbumObjectID,
			"userID": userID,
		}).Decode(&fromAlbum)
		
		if err == mongo.ErrNoDocuments {
			log.Printf("MoveFavorite: source album not found or doesn't belong to user")
			errorResponse(rw, fmt.Errorf("source album not found"), 404)
			return
		}
		if err != nil {
			log.Printf("MoveFavorite: error finding source album: %v", err)
			errorResponse(rw, err, 500)
			return
		}
		
		var toAlbum models.Album
		err = albumsCollection.FindOne(ctx, bson.M{
			"_id":    toAlbumObjectID,
			"userID": userID,
		}).Decode(&toAlbum)
		
		if err == mongo.ErrNoDocuments {
			log.Printf("MoveFavorite: destination album not found or doesn't belong to user")
			errorResponse(rw, fmt.Errorf("destination album not found"), 404)
			return
		}
		if err != nil {
			log.Printf("MoveFavorite: error finding destination album: %v", err)
			errorResponse(rw, err, 500)
			return
		}
		
		// 2. Check if favorite exists in source album
		var favorite models.Favorite
		err = favoritesCollection.FindOne(ctx, bson.M{
			"userID":    userID,
			"contentID": contentID,
			"albumID":   fromAlbumID,
		}).Decode(&favorite)
		
		if err == mongo.ErrNoDocuments {
			log.Printf("MoveFavorite: favorite not found in source album")
			errorResponse(rw, fmt.Errorf("content not found in source album"), 404)
			return
		}
		if err != nil {
			log.Printf("MoveFavorite: error finding favorite: %v", err)
			errorResponse(rw, err, 500)
			return
		}
		
		var existingInDest models.Favorite
		err = favoritesCollection.FindOne(ctx, bson.M{
			"userID":    userID,
			"contentID": contentID,
			"albumID":   toAlbumID,
		}).Decode(&existingInDest)
		
		if err == nil {
			// Already exists in destination - just delete from source
			log.Printf("MoveFavorite: content already exists in destination album, removing from source")
			
			_, err = favoritesCollection.DeleteOne(ctx, bson.M{
				"_id": favorite.ID,
			})
			if err != nil {
				log.Printf("MoveFavorite: error deleting from source: %v", err)
				errorResponse(rw, err, 500)
				return
			}
			
	
			now := time.Now()
			_, _ = albumsCollection.UpdateOne(ctx, bson.M{"_id": fromAlbumObjectID}, bson.M{"$set": bson.M{"dateModified": now}})
			_, _ = albumsCollection.UpdateOne(ctx, bson.M{"_id": toAlbumObjectID}, bson.M{"$set": bson.M{"dateModified": now}})
			
			successResponse(rw, "Content already existed in destination album, removed from source")
			return
		}
		

		updateResult, err := favoritesCollection.UpdateOne(ctx,
			bson.M{"_id": favorite.ID},
			bson.M{"$set": bson.M{
				"albumID":   toAlbumID,
				"dateAdded": time.Now(),
			}},
		)
		
		if err != nil {
			log.Printf("MoveFavorite: error updating favorite: %v", err)
			errorResponse(rw, err, 500)
			return
		}
		
		if updateResult.ModifiedCount == 0 {
			log.Printf("MoveFavorite: no documents were modified")
			errorResponse(rw, fmt.Errorf("failed to move favorite"), 500)
			return
		}
		
		// 5. Update both albums' dateModified
		now := time.Now()
		_, err = albumsCollection.UpdateOne(ctx,
			bson.M{"_id": fromAlbumObjectID},
			bson.M{"$set": bson.M{"dateModified": now}},
		)
		if err != nil {
			log.Printf("MoveFavorite: warning - could not update source album dateModified: %v", err)
		}
		
		_, err = albumsCollection.UpdateOne(ctx,
			bson.M{"_id": toAlbumObjectID},
			bson.M{"$set": bson.M{"dateModified": now}},
		)
		if err != nil {
			log.Printf("MoveFavorite: warning - could not update destination album dateModified: %v", err)
		}
		
		log.Printf("MoveFavorite: successfully moved content from '%s' to '%s'", fromAlbum.Title, toAlbum.Title)
		
		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(map[string]interface{}{
			"status":  200,
			"message": fmt.Sprintf("Content moved from '%s' to '%s' successfully", fromAlbum.Title, toAlbum.Title),
			"data": map[string]interface{}{
				"fromAlbum": fromAlbum.Title,
				"toAlbum":   toAlbum.Title,
				"contentID": contentID,
			},
		})
	}
}

func GetUserFavorites() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		
		vars := mux.Vars(r)
		userID := vars["UserID"]
		
		log.Printf("GetUserFavorites: userID=%s", userID)
		
		albumsCollection := configs.GetCollection(configs.DB, "albums")
		favoritesCollection := configs.GetCollection(configs.DB, "favorites")
		contentsCollection := configs.GetCollection(configs.DB, "contents")
		

		cursor, err := albumsCollection.Find(ctx, bson.M{"userID": userID})
		if err != nil {
			log.Printf("GetUserFavorites: error finding albums: %v", err)
			errorResponse(rw, err, 500)
			return
		}
		defer cursor.Close(ctx)
		
		var albums []models.Album
		if err = cursor.All(ctx, &albums); err != nil {
			log.Printf("GetUserFavorites: error decoding albums: %v", err)
			errorResponse(rw, err, 500)
			return
		}
		
		log.Printf("GetUserFavorites: found %d albums", len(albums))
		
		// If no albums, return empty structure
		if len(albums) == 0 {
			rw.Header().Set("Content-Type", "application/json")
			rw.WriteHeader(http.StatusOK)
			json.NewEncoder(rw).Encode(map[string]interface{}{
				"status":  200,
				"message": "success",
				"data": map[string]interface{}{
					"userID": userID,
					"albums": []interface{}{},
				},
			})
			return
		}
		
		// 2. Build response with albums and their favorites
		type FavoriteWithContent struct {
			ID        primitive.ObjectID `json:"id"`
			ContentID string             `json:"contentID"`
			AlbumID   string             `json:"albumID"`
			DateAdded time.Time          `json:"dateAdded"`
			Content   *models.Content    `json:"content"`
		}
		
		type AlbumWithFavorites struct {
			ID           primitive.ObjectID     `json:"id"`
			Title        string                 `json:"title"`
			Description  string                 `json:"description,omitempty"`
			DateCreated  time.Time              `json:"dateCreated"`
			DateModified time.Time              `json:"dateModified"`
			Favorites    []FavoriteWithContent  `json:"favorites"`
		}
		
		var albumsWithFavorites []AlbumWithFavorites
		totalOrphansSkipped := 0
		
		for _, album := range albums {
			albumID := album.ID.Hex()
			
			// Get all favorites in this album
			favCursor, err := favoritesCollection.Find(ctx, bson.M{
				"userID":  userID,
				"albumID": albumID,
			})
			if err != nil {
				log.Printf("GetUserFavorites: error finding favorites for album %s: %v", albumID, err)
				continue
			}
			
			var favorites []models.Favorite
			if err = favCursor.All(ctx, &favorites); err != nil {
				log.Printf("GetUserFavorites: error decoding favorites: %v", err)
				favCursor.Close(ctx)
				continue
			}
			favCursor.Close(ctx)
			
			log.Printf("GetUserFavorites: album '%s' has %d favorites (before filtering)", album.Title, len(favorites))
			
			// 3. Populate content data for each favorite - ONLY INCLUDE IF CONTENT EXISTS
			var favoritesWithContent []FavoriteWithContent
			orphansInThisAlbum := 0
			
			for _, fav := range favorites {
				// Fetch full content data
				contentObjectID, err := primitive.ObjectIDFromHex(fav.ContentID)
				if err != nil {
					log.Printf("GetUserFavorites: invalid contentID format: %s - skipping", fav.ContentID)
					orphansInThisAlbum++
					continue // Skip this favorite
				}
				
				var content models.Content
				err = contentsCollection.FindOne(ctx, bson.M{"_id": contentObjectID}).Decode(&content)
				
				if err != nil {
					if err == mongo.ErrNoDocuments {
						log.Printf("GetUserFavorites: content %s not found (deleted/orphaned) - skipping", fav.ContentID)
					} else {
						log.Printf("GetUserFavorites: error fetching content %s: %v - skipping", fav.ContentID, err)
					}
					orphansInThisAlbum++
					continue // Skip this favorite - DON'T include in response
				}
				
				// Content exists - include it
				favWithContent := FavoriteWithContent{
					ID:        fav.ID,
					ContentID: fav.ContentID,
					AlbumID:   fav.AlbumID,
					DateAdded: fav.DateAdded,
					Content:   &content,
				}
				
				favoritesWithContent = append(favoritesWithContent, favWithContent)
			}
			
			if orphansInThisAlbum > 0 {
				log.Printf("GetUserFavorites: skipped %d orphaned favorites in album '%s'", orphansInThisAlbum, album.Title)
				totalOrphansSkipped += orphansInThisAlbum
			}
			
			log.Printf("GetUserFavorites: album '%s' has %d valid favorites (after filtering)", album.Title, len(favoritesWithContent))
			
			// 4. Build album with favorites
			albumWithFavs := AlbumWithFavorites{
				ID:           album.ID,
				Title:        album.Title,
				Description:  album.Description,
				DateCreated:  album.DateCreated,
				DateModified: album.DateModified,
				Favorites:    favoritesWithContent,
			}
			
			albumsWithFavorites = append(albumsWithFavorites, albumWithFavs)
		}
		
		if totalOrphansSkipped > 0 {
			log.Printf("GetUserFavorites: TOTAL orphaned favorites skipped: %d", totalOrphansSkipped)
		}
		
		// Return response
		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(map[string]interface{}{
			"status":  200,
			"message": "success",
			"data": map[string]interface{}{
				"userID": userID,
				"albums": albumsWithFavorites,
			},
		})
	}
}

func GetUserAlbums() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		
		vars := mux.Vars(r)
		userID := vars["UserID"]
		
		log.Printf("GetUserAlbums: userID=%s", userID)
		
		albumsCollection := configs.GetCollection(configs.DB, "albums")
		favoritesCollection := configs.GetCollection(configs.DB, "favorites")
		
		cursor, err := albumsCollection.Find(ctx, bson.M{"userID": userID})
		if err != nil {
			log.Printf("GetUserAlbums: error finding albums: %v", err)
			errorResponse(rw, err, 500)
			return
		}
		defer cursor.Close(ctx)
		
		var albums []models.Album
		if err = cursor.All(ctx, &albums); err != nil {
			log.Printf("GetUserAlbums: error decoding albums: %v", err)
			errorResponse(rw, err, 500)
			return
		}
		
		log.Printf("GetUserAlbums: found %d albums", len(albums))
		
	
		type AlbumWithCount struct {
			ID            primitive.ObjectID `json:"id"`
			Title         string             `json:"title"`
			Description   string             `json:"description,omitempty"`
			DateCreated   time.Time          `json:"dateCreated"`
			DateModified  time.Time          `json:"dateModified"`
			FavoritesCount int               `json:"favoritesCount"`
		}
		
		var albumsWithCount []AlbumWithCount
		
		for _, album := range albums {
			albumID := album.ID.Hex()
			
			// Count favorites in this album
			count, err := favoritesCollection.CountDocuments(ctx, bson.M{
				"userID":  userID,
				"albumID": albumID,
			})
			if err != nil {
				log.Printf("GetUserAlbums: error counting favorites for album %s: %v", albumID, err)
				count = 0 
			}
			
			albumWithCount := AlbumWithCount{
				ID:             album.ID,
				Title:          album.Title,
				Description:    album.Description,
				DateCreated:    album.DateCreated,
				DateModified:   album.DateModified,
				FavoritesCount: int(count),
			}
			
			albumsWithCount = append(albumsWithCount, albumWithCount)
		}
		
		// Return response
		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(map[string]interface{}{
			"status":  200,
			"message": "success",
			"data": map[string]interface{}{
				"userID": userID,
				"albums": albumsWithCount,
			},
		})
	}
}
