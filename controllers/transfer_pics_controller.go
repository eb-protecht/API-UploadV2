package controllers

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/jpeg"
	"image/png"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
	"upload-service/models"

	"go.mongodb.org/mongo-driver/bson"
)

func TransferProfilePics() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
		defer cancel()
		cur, err := profilepicsCollection.Find(ctx, bson.M{"location": bson.M{"$exists": false}})
		if err != nil {
			errorResponse(w, err, 200)
			return
		}
		basePath := "/mnt/storage/hls/vod/"
		profilepics := []models.ProfilePic{}
		if err := cur.All(ctx, &profilepics); err != nil {
			errorResponse(w, err, 200)
			return
		}
		for _, profilePic := range profilepics {
			folderPath := basePath + profilePic.UserID + "/profile_pics/"
			// Decode the base64 string
			data, err := base64.StdEncoding.DecodeString(profilePic.Base64)
			if err != nil {
				log.Fatal("Error decoding base64 data", err)
			}
			reader := bytes.NewReader(data)
			img, format, err := image.Decode(reader)
			if err != nil {
				log.Fatal("Error decoding image", err)
			}
			var extension string
			switch format {
			case "jpeg":
				extension = ".jpg"
			case "png":
				extension = ".png"
			default:
				log.Fatalf("Unsupported or unknown image format: %s", format)
			}
			outputFileName := profilePic.ID.Hex() + extension
			fullPath := filepath.Join(folderPath, outputFileName)

			if _, err := os.Stat(folderPath); os.IsNotExist(err) {
				err := os.MkdirAll(folderPath, 0755)
				if err != nil {
					log.Fatal("Error creating directory", err)
				}
			}
			file, err := os.Create(fullPath)
			if err != nil {
				log.Fatal("Error creating file", err)
			}
			switch format {
			case "jpeg":
				err = jpeg.Encode(file, img, nil)
			case "png":
				err = png.Encode(file, img)
			default:
				log.Fatalf("Unsupported image type for saving: %s", format)
			}

			if err != nil {
				log.Fatal("Error saving image", err)
			}
			file.Close()
			profilepicsCollection.UpdateOne(ctx, bson.M{"_id": profilePic.ID}, bson.M{"$set": bson.M{"location": fullPath}})
		}
		successResponse(w, "Transfered")
	}
}
