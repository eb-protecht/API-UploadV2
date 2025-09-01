package controllers

import (
	"context"
	"fmt"
	"net/http"
	"time"
	"upload-service/configs"
	"upload-service/models"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

var favoritesCollection *mongo.Collection = configs.GetCollection(configs.DB, "favorites")

func AddContentToFavorites() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		vars := mux.Vars(r)
		userID := vars["UserID"]
		contentID := vars["ContentID"]
		album := vars["AlbumTitle"]
		favorites := models.Favorite{}

		res := favoritesCollection.FindOne(ctx, bson.M{"userid": userID})
		if err := res.Decode(&favorites); err != nil {
			if err != mongo.ErrNoDocuments {
				errorResponse(rw, err, 200)
				return
			}
			favorites.UserID = userID
			favorites.DateCreated = time.Now()
			var albums []models.Album
			albumObj := models.Album{}
			if album != "" || album != " " {
				albumObj.Title = album
			} else {
				albumObj.Title = "favorites"
			}
			albumObj.DateCreated = time.Now()
			albumObj.Content = append(albumObj.Content, contentID)
			albums = append(albums, albumObj)
			favorites.Albums = albums
			_, err := favoritesCollection.InsertOne(ctx, favorites)
			if err != nil {
				errorResponse(rw, err, 200)
				return
			}
			successResponse(rw, "OK")
			return
		}
		var index int
		var albumExists bool
		for i, v := range favorites.Albums {
			if v.Title == album {
				index = i
				albumExists = true
			}
			for _, c := range v.Content {
				if c == contentID {
					successResponse(rw, "content alredy is in favorites in: "+v.Title)
					return
				}
			}
		}
		if albumExists {
			favorites.Albums[index].Content = append(favorites.Albums[index].Content, contentID)
		} else {
			newAlbum := models.Album{}
			newAlbum.Content = append(newAlbum.Content, contentID)
			newAlbum.DateCreated = time.Now()
			if album != "" && album != " " {
				newAlbum.Title = album
			} else {
				newAlbum.Title = "favorites"
			}
			favorites.Albums = append(favorites.Albums, newAlbum)
		}
		updateResult, err := favoritesCollection.UpdateOne(ctx, bson.M{"userid": userID}, bson.M{"$set": bson.M{"albums": favorites.Albums}})
		if err != nil {
			errorResponse(rw, err, 200)
			return
		}
		successResponse(rw, updateResult.ModifiedCount)
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
		res := favoritesCollection.FindOne(ctx, bson.M{"userid": userID})
		if err := res.Decode(&favorites); err != nil {
			if err != mongo.ErrNoDocuments {
				errorResponse(rw, err, 200)
				return
			}
		}
		for k, v := range favorites.Albums {
			for j, content := range v.Content {
				if content == contentID {
					favorites.Albums[k].Content = append(favorites.Albums[k].Content[:j], favorites.Albums[k].Content[j+1:]...)
					break
				}
			}
		}
		updateResult, err := favoritesCollection.UpdateOne(ctx, bson.M{"userid": userID}, bson.M{"$set": bson.M{"albums": favorites.Albums}})
		if err != nil {
			errorResponse(rw, err, 200)
			return
		}
		successResponse(rw, updateResult.ModifiedCount)
	}
}

func CreateNewAlbum() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		vars := mux.Vars(r)
		userID := vars["UserID"]
		album := vars["AlbumTitle"]
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
		updateResult, err := favoritesCollection.UpdateOne(ctx, bson.M{"userid": userID}, bson.M{"$set": bson.M{"albums": favorites.Albums}})
		if err != nil {
			errorResponse(rw, err, 200)
			return
		}
		successResponse(rw, updateResult.ModifiedCount)
	}
}
