package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"upload-service/configs"
	"upload-service/models"
	"upload-service/responses"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/disintegration/imaging"
	"github.com/gabriel-vasile/mimetype"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func getContentCollection() *mongo.Collection {
	return configs.GetCollection(configs.DB, "content")
}

func getProfilePicsCollection() *mongo.Collection {
	return configs.GetCollection(configs.DB, "profile_pics")
}

func getVideosCollection() *mongo.Collection {
	return configs.GetCollection(configs.DB, "videos")
}

func getUsersCollection() *mongo.Collection {
	return configs.GetCollection(configs.DB, "users")
}

func getFollowsCollection() *mongo.Collection {
	return configs.GetCollection(configs.DB, "follows")
}

//var validate = validator.New()

// 10MB
const (
	MB                   = 1 << 20
	VISIBILITY_EVERYONE  = "everyone"
	VISIBILITY_FOLLOWERS = "followers"
)

// 10GB
func Execute(script string, command []string) (bool, error) {
	cmd := &exec.Cmd{
		Path:   script,
		Args:   command,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}

	err := cmd.Start()
	if err != nil {
		fmt.Println(err)
		return false, err
	}

	/* err = cmd.Wait()
	if err != nil {
		return false, err
	} */

	return true, nil
}

func CleanUp(path string) (bool, error) {
	e := os.Remove(path)
	if e != nil {
		return false, e

	}
	return true, nil
}

func errorResponse(rw http.ResponseWriter, err error, code int) {
	rw.WriteHeader(code)
	response := responses.ContentResponse{Status: code, Message: "error", Data: map[string]interface{}{"data": err.Error()}}
	json.NewEncoder(rw).Encode(response)
}

func successResponse(rw http.ResponseWriter, result interface{}) {
	rw.WriteHeader(http.StatusCreated)
	response := responses.ContentResponse{Status: http.StatusCreated, Message: "success", Data: map[string]interface{}{"data": result}}
	json.NewEncoder(rw).Encode(response)
}

const (
	DELETE       = "delete"
	MAKE_CURRENT = "current"
	TYPE_PIC     = "pic"
	TYPE_VIDEO   = "video"
	TYPE_TEXT    = "text"
	TYPE_STREAM  = "live"
)

func UpdateOnProfilePic() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		vars := mux.Vars(r)
		userID := vars["UserID"]
		fileName := vars["Filename"]
		whatWillChange := vars["ToBeChanged"]
		userObj := models.User{}
		oID, err := primitive.ObjectIDFromHex(userID)
		if err != nil {
			errorResponse(rw, fmt.Errorf("invalid object id"), 200)
			return
		}
		err = getUsersCollection().FindOne(ctx, bson.M{"_id": oID}).Decode(&userObj)
		if err != nil {
			errorResponse(rw, fmt.Errorf("couldn't find/decode user"), 200)
			return
		}

		//location := configs.EnvMediaDir() + userID + "/pics/profile/" + fileName

		filterByFilename := bson.M{"filename": fileName, "userid": userID}
		filterByUserID := bson.M{"_id": oID}
		if whatWillChange == DELETE {
			delete := bson.M{"$set": bson.M{"isdeleted": true, "iscurrent": false}}
			err := getProfilePicsCollection().FindOneAndUpdate(ctx, filterByFilename, delete).Err()
			if err != nil {
				fmt.Println("1")
				errorResponse(rw, err, 500)
				return
			}
			sortByDateCreated := options.Find()
			sortByDateCreated.SetSort(bson.D{{Key: "datecreated", Value: -1}})
			filterByNotDeleted := bson.M{"userid": userID, "isdeleted": false}

			var pics []models.ProfilePic
			cur, err := getProfilePicsCollection().Find(ctx, filterByNotDeleted, sortByDateCreated)
			if err != nil {
				fmt.Println("2")
				errorResponse(rw, err, 500)
				return
			}
			if err := cur.All(ctx, &pics); err != nil {
				if err != nil {
					fmt.Println("3")
					errorResponse(rw, err, 500)
					return
				}
			}
			var hasCurrent bool
			for _, v := range pics {
				if v.IsCurrent {
					hasCurrent = true
				}
			}
			if !hasCurrent {
				filterByID := bson.M{"_id": pics[0].ID}
				makeCurrentTrue := bson.M{"$set": bson.M{"iscurrent": true}}

				_, err = getProfilePicsCollection().UpdateOne(ctx, filterByID, makeCurrentTrue)
				if err != nil {
					fmt.Println("4")
					errorResponse(rw, err, 500)
					return
				}
				setUserPic := bson.M{"$set": bson.M{"profile_pic": pics[0].Location}}
				_, err = getUsersCollection().UpdateOne(ctx, filterByUserID, setUserPic)
				if err != nil {
					fmt.Println("could not set new profile pic")
					errorResponse(rw, err, 500)
					return
				}
			}
		} else if whatWillChange == MAKE_CURRENT {
			filterByCurrent := bson.M{"userid": userID, "iscurrent": true}
			makeCurrentFalse := bson.M{"$set": bson.M{"iscurrent": false}}
			makeCurrentTrue := bson.M{"$set": bson.M{"iscurrent": true}}

			_, err := getProfilePicsCollection().UpdateOne(ctx, filterByCurrent, makeCurrentFalse)
			if err != nil {
				fmt.Println("11111")
				errorResponse(rw, err, 500)
				return
			}
			_, err = getProfilePicsCollection().UpdateOne(ctx, filterByFilename, makeCurrentTrue)
			if err != nil {
				fmt.Println("22222", fileName)
				errorResponse(rw, err, 500)
				return
			}
			pic := models.NewProfilePic{}
			err = getProfilePicsCollection().FindOne(ctx, filterByFilename).Decode(&pic)
			if err != nil {
				fmt.Println("could not load new pic")
				errorResponse(rw, err, 500)
				return
			}
			setUserPic := bson.M{"$set": bson.M{"profile_pic": pic.Location}}
			_, err = getUsersCollection().UpdateOne(ctx, filterByUserID, setUserPic)
			if err != nil {
				fmt.Println("could not set new profile pic")
				errorResponse(rw, err, 500)
				return
			}
		} else {
			err := fmt.Errorf("malformed request")
			errorResponse(rw, err, 400)
			return
		}
		successResponse(rw, "OK")
	}
}

func EditContent() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		vars := mux.Vars(r)
		oID := vars["ContentID"]
		title := vars["Title"]
		description := vars["Description"]
		tags := vars["Tags"]
		show, _ := strconv.ParseBool(vars["Show"])
		ispayperview, _ := strconv.ParseBool(vars["IsPayPerView"])
		isdeleted, _ := strconv.ParseBool(vars["IsDeleted"])
		theType := vars["Type"]
		posting := vars["Posting"]
		ppvprice := vars["PPVPrice"]
		price, err := strconv.ParseFloat(ppvprice, 64)
		if err != nil {
			fmt.Println("invalid price in PPVPrice")
			price = 0
		}
		contentID, err := primitive.ObjectIDFromHex(oID)
		if err != nil {
			errorResponse(rw, err, 200)
			return
		}
		theTags := strings.Split(tags, ",")
		filterByFilename := bson.M{"_id": contentID}
		update := bson.M{}
		if theType == TYPE_PIC || theType == TYPE_VIDEO {
			update = bson.M{"$set": bson.M{
				"isdeleted":    isdeleted,
				"show":         show,
				"ispayperview": ispayperview,
				"ppvprice":     price,
				"title":        title,
				"description":  description,
				"tags":         theTags,
			}}
		} else if theType == TYPE_TEXT {
			update = bson.M{"$set": bson.M{
				"isdeleted":    isdeleted,
				"posting":      posting,
				"show":         show,
				"ispayperview": ispayperview,
				"ppvprice":     price,
				"title":        title,
				"description":  description,
				"tags":         theTags,
			}}
		}
		mongoSingleResult := getContentCollection().FindOneAndUpdate(ctx, filterByFilename, update)

		var content = models.Content{}
		err = mongoSingleResult.Decode(&content)
		if err != nil {
			errorResponse(rw, err, 500)
			return
		}
		successResponse(rw, "OK")
		//go insertInREDISGetContentByUserID(content.UserID)
	}
}

func EditContentWithBody() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		vars := mux.Vars(r)
		oID := vars["ContentID"]
		tags := vars["Tags"]
		show, _ := strconv.ParseBool(vars["Show"])
		ispayperview, _ := strconv.ParseBool(vars["IsPayPerView"])
		isdeleted, _ := strconv.ParseBool(vars["IsDeleted"])
		theType := vars["Type"]
		ppvprice := vars["PPVPrice"]
		price, err := strconv.ParseFloat(ppvprice, 64)
		if err != nil {
			fmt.Println("invalid price in PPVPrice")
			price = 0
		}
		contentID, err := primitive.ObjectIDFromHex(oID)
		if err != nil {
			errorResponse(rw, err, 200)
			return
		}
		contentBody := models.ContentBody{}

		err = json.NewDecoder(r.Body).Decode(&contentBody)
		if err != nil {
			errorResponse(rw, fmt.Errorf("bad request"), 200)
			return
		}

		title := contentBody.Title
		description := contentBody.Description
		posting := contentBody.Posting

		theTags := strings.Split(tags, ",")
		filterByFilename := bson.M{"_id": contentID}
		update := bson.M{}
		if theType == TYPE_PIC || theType == TYPE_VIDEO {
			update = bson.M{"$set": bson.M{
				"isdeleted":    isdeleted,
				"show":         show,
				"ispayperview": ispayperview,
				"ppvprice":     price,
				"title":        title,
				"description":  description,
				"tags":         theTags,
			}}
		} else if theType == TYPE_TEXT {
			update = bson.M{"$set": bson.M{
				"isdeleted":    isdeleted,
				"posting":      posting,
				"show":         show,
				"ispayperview": ispayperview,
				"ppvprice":     price,
				"title":        title,
				"description":  description,
				"tags":         theTags,
			}}
		}
		mongoSingleResult := getContentCollection().FindOneAndUpdate(ctx, filterByFilename, update)

		var content = models.Content{}
		err = mongoSingleResult.Decode(&content)
		if err != nil {
			errorResponse(rw, err, 500)
			return
		}
		successResponse(rw, "OK")
		//go insertInREDISGetContentByUserID(content.UserID)
	}
}
func EditContentWithBodyV2() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		vars := mux.Vars(r)
		oID := vars["ContentID"]
		tags := vars["Tags"]
		show, _ := strconv.ParseBool(vars["Show"])
		ispayperview, _ := strconv.ParseBool(vars["IsPayPerView"])
		isdeleted, _ := strconv.ParseBool(vars["IsDeleted"])
		theType := vars["Type"]
		ppvprice := vars["PPVPrice"]
		visibility := vars["Visibility"]
		price, err := strconv.ParseFloat(ppvprice, 64)
		if err != nil {
			fmt.Println("invalid price in PPVPrice")
			price = 0
		}
		contentID, err := primitive.ObjectIDFromHex(oID)
		if err != nil {
			errorResponse(rw, err, 200)
			return
		}
		contentBody := models.ContentBody{}

		err = json.NewDecoder(r.Body).Decode(&contentBody)
		if err != nil {
			errorResponse(rw, fmt.Errorf("bad request"), 200)
			return
		}

		title := contentBody.Title
		description := contentBody.Description
		posting := contentBody.Posting
		if visibility != VISIBILITY_FOLLOWERS {
			visibility = VISIBILITY_EVERYONE
		}
		theTags := strings.Split(tags, ",")
		filterByFilename := bson.M{"_id": contentID}
		update := bson.M{}
		if theType == TYPE_PIC || theType == TYPE_VIDEO {
			update = bson.M{"$set": bson.M{
				"isdeleted":    isdeleted,
				"show":         show,
				"ispayperview": ispayperview,
				"ppvprice":     price,
				"title":        title,
				"description":  description,
				"tags":         theTags,
				"visibility":   visibility,
			}}
		} else if theType == TYPE_TEXT {
			update = bson.M{"$set": bson.M{
				"isdeleted":    isdeleted,
				"posting":      posting,
				"show":         show,
				"ispayperview": ispayperview,
				"ppvprice":     price,
				"title":        title,
				"description":  description,
				"tags":         theTags,
				"visibility":   visibility,
			}}
		}
		mongoSingleResult := getContentCollection().FindOneAndUpdate(ctx, filterByFilename, update)

		var content = models.Content{}
		err = mongoSingleResult.Decode(&content)
		if err != nil {
			errorResponse(rw, err, 500)
			return
		}
		successResponse(rw, "OK")
		//go insertInREDISGetContentByUserID(content.UserID)
	}
}

func deleteFromS3(bucketName, S3RawKey string) error {
	if S3RawKey == "" {
		return nil
	}

	ctx := context.Background()
	client := configs.GetS3Client()

	_, err := client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(S3RawKey),
	})

	if err != nil {
		fmt.Println("Error deleting from S3:", err)
		return err
	}

	fmt.Println("Deleted from S3:", S3RawKey)
	return nil
}

func PostProfilePic() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		vars := mux.Vars(r)
		userID := vars["UserID"]
		iscurrent, _ := strconv.ParseBool(vars["IsCurrent"])
		isdeleted, _ := strconv.ParseBool(vars["IsDeleted"])

		// Generate unique ID
		imageID := strings.Replace(uuid.New().String(), "-", "", -1)

		// Parse form
		r.ParseMultipartForm(10 * MB)
		r.Body = http.MaxBytesReader(rw, r.Body, 10*MB)

		// Retrieve file from form
		file, _, err := r.FormFile("file")
		if err != nil {
			errorResponse(rw, err, http.StatusBadRequest)
			return
		}
		defer file.Close()

		// Read the entire file for detection
		fileBytes, err := io.ReadAll(file)
		if err != nil {
			fmt.Println("failed to read file data:", err)
			errorResponse(rw, err, http.StatusInternalServerError)
			return
		}

		// Detect MIME
		detectedMIME := mimetype.Detect(fileBytes)
		mimeType := detectedMIME.String()
		fmt.Println("Detected MIME:", mimeType)

		extension := ""
		switch mimeType {
		case "image/png":
			extension = "png"
		case "image/jpeg":
			extension = "jpeg"
		case "image/webp":
			extension = "webp"
		case "image/heic", "image/heif":
			extension = "heic"
		default:
			http.Error(rw, "This file type is not allowed for images", http.StatusBadRequest)
			return
		}

		// If setting as current, delete old current profile pic from S3
		if iscurrent {
			var oldPic models.NewProfilePic
			err := getProfilePicsCollection().FindOne(ctx, bson.M{
				"userid":    userID,
				"iscurrent": true,
			}).Decode(&oldPic)

			if err == nil && oldPic.S3RawKey != "" {
				// Delete old file from S3
				deleteFromS3(configs.EnvPicturesBucket(), oldPic.S3RawKey)

				// Mark as not current in DB
				getProfilePicsCollection().UpdateOne(ctx,
					bson.M{"_id": oldPic.ID},
					bson.M{"$set": bson.M{"iscurrent": false}},
				)
			}
		}

		uploader := configs.GetS3Uploader()
		S3RawKey := fmt.Sprintf("%s/profile/%s.%s", userID, imageID, extension)

		fmt.Printf("Uploading profile pic to S3: s3://%s/%s\n", configs.EnvPicturesBucket(), S3RawKey)

		_, err = uploader.Upload(ctx, &s3.PutObjectInput{
			Bucket:      aws.String(configs.EnvPicturesBucket()),
			Key:         aws.String(S3RawKey),
			Body:        bytes.NewReader(fileBytes),
			ContentType: aws.String(mimeType),
		})
		if err != nil {
			fmt.Println("Error uploading to S3:", err)
			errorResponse(rw, fmt.Errorf("error uploading profile pic"), 500)
			return
		}
		fmt.Println("Profile pic uploaded to S3")

		profilePicURL := configs.EnvPicturesCDNURL() + "/" + S3RawKey

		newPostPic := models.NewProfilePic{
			UserID:      userID,
			Location:    profilePicURL,
			S3RawKey:    S3RawKey,
			DateCreated: time.Now(),
			IsCurrent:   iscurrent,
			IsDeleted:   isdeleted,
		}

		result, err := getProfilePicsCollection().InsertOne(ctx, newPostPic)
		if err != nil {
			errorResponse(rw, err, http.StatusInternalServerError)
			return
		}

		rw.WriteHeader(http.StatusCreated)
		response := responses.ContentResponse{
			Status:  http.StatusCreated,
			Message: "success",
			Data:    map[string]interface{}{"data": result},
		}
		json.NewEncoder(rw).Encode(response)
	}
}

func uploadProfilePic(userID string, file multipart.File, fileHeader *multipart.FileHeader, filename string) (string, error) {
	ctx := context.Background()

	// Read file
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}

	// Detect extension
	detectedMIME := mimetype.Detect(fileBytes)
	mimeType := detectedMIME.String()

	extension := ""
	switch mimeType {
	case "image/png":
		extension = "png"
	case "image/jpeg":
		extension = "jpeg"
	case "image/webp":
		extension = "webp"
	case "image/heic", "image/heif":
		extension = "heic"
	default:
		extension = strings.Split(fileHeader.Filename, ".")[1]
	}

	// Upload to S3
	uploader := configs.GetS3Uploader()
	s3Key := fmt.Sprintf("%s/profile/%s.%s", userID, filename, extension)

	_, err = uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(configs.EnvPicturesBucket()),
		Key:         aws.String(s3Key),
		Body:        bytes.NewReader(fileBytes),
		ContentType: aws.String(mimeType),
	})
	if err != nil {
		return "", err
	}

	return configs.EnvPicturesCDNURL() + "/" + s3Key, nil
}

func PostProfilePicBase64() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		vars := mux.Vars(r)
		userID := vars["UserID"]
		oid, err := primitive.ObjectIDFromHex(userID)
		if err != nil {
			errorResponse(rw, fmt.Errorf("invalid userID"), 400)
			return
		}

		iscurrent, _ := strconv.ParseBool(vars["IsCurrent"])

		r.ParseMultipartForm(10 * MB)
		r.Body = http.MaxBytesReader(rw, r.Body, 10*MB)

		file, fileHeader, err := r.FormFile("file")
		if err != nil {
			errorResponse(rw, err, 400)
			return
		}
		defer file.Close()

		profileID := primitive.NewObjectID()

		// uploadProfilePic now returns CDN URL and uploads to S3
		location, err := uploadProfilePic(userID, file, fileHeader, profileID.Hex())
		if err != nil {
			errorResponse(rw, err, 500)
			return
		}

		fmt.Print("LOADING WHEN LOADED", location)

		fmt.Println("iscurrent", iscurrent)
		// If setting as current, delete old current profile pic from S3
		if iscurrent {
			//var oldPic models.NewProfilePic
			// err := getProfilePicsCollection().FindOne(ctx, bson.M{
			// 	"userid":    userID,
			// 	"iscurrent": true,
			// }).Decode(&oldPic)
			// TODO AFTER YOU SET THE DELETE POLICY IN ACCOUNT
			// if err == nil && oldPic.S3RawKey != "" {
			// 	deleteFromS3(configs.EnvPicturesBucket(), oldPic.S3RawKey)
			// }

			// Mark old pics as not current
			getProfilePicsCollection().UpdateMany(ctx,
				bson.M{"userid": userID, "iscurrent": true},
				bson.M{"$set": bson.M{"iscurrent": false}},
			)

			// Update user's profile_pic field
			getUsersCollection().UpdateOne(ctx,
				bson.M{"_id": oid},
				bson.M{"$set": bson.M{"profile_pic": location}},
			)
		}

		newPostPic := models.NewProfilePic{
			ID:          profileID,
			UserID:      userID,
			Location:    location,
			S3RawKey:    fmt.Sprintf("%s/profile/%s", userID, profileID.Hex()), // Assuming uploadProfilePic uses this pattern
			Filename:    profileID.Hex(),
			DateCreated: time.Now(),
			IsCurrent:   iscurrent,
			IsDeleted:   false,
		}

		result, err := getProfilePicsCollection().InsertOne(ctx, newPostPic)
		if err != nil {
			errorResponse(rw, err, 500)
			return
		}

		successResponse(rw, result)
	}
}

func PostPic() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		vars := mux.Vars(r)
		userID := vars["UserID"]
		title := vars["Title"]
		description := vars["Description"]
		tags := vars["Tags"]
		visibility := vars["Visibility"]
		show, _ := strconv.ParseBool(vars["Show"])
		ispayperview, _ := strconv.ParseBool(vars["IsPayPerView"])
		isdeleted, _ := strconv.ParseBool(vars["IsDeleted"])
		ppvprice := vars["PPVPrice"]
		price, err := strconv.ParseFloat(ppvprice, 64)
		if err != nil {
			fmt.Println("invalid price in PPVPrice")
			price = 0
		}

		if visibility != VISIBILITY_FOLLOWERS {
			visibility = VISIBILITY_EVERYONE
		}

		// Generate unique ID
		imageID := strings.Replace(uuid.New().String(), "-", "", -1)

		r.ParseMultipartForm(10 * MB)
		r.Body = http.MaxBytesReader(rw, r.Body, 10*MB)

		file, _, err := r.FormFile("file")
		if err != nil {
			fmt.Println("Error reading form file:", err)
			errorResponse(rw, err, 400)
			return
		}
		defer file.Close()

		// Read the entire file for MIME detection
		fileBytes, err := io.ReadAll(file)
		if err != nil {
			fmt.Println("Failed to read file data:", err)
			errorResponse(rw, fmt.Errorf("failed to read file"), 500)
			return
		}

		// Detect MIME type
		detectedMIME := mimetype.Detect(fileBytes)
		mimeType := detectedMIME.String()
		fmt.Println("Detected MIME:", mimeType)

		extension := ""
		switch mimeType {
		case "image/png":
			extension = "png"
		case "image/jpeg":
			extension = "jpeg"
		case "image/webp":
			extension = "webp"
		case "image/heic", "image/heif":
			extension = "heic"
		default:
			http.Error(rw, "This file type is not allowed for images", http.StatusBadRequest)
			return
		}

		// Upload original to S3
		uploader := configs.GetS3Uploader()
		s3OriginalKey := fmt.Sprintf("%s/%s.%s", userID, imageID, extension)

		fmt.Printf("Uploading original image to S3: s3://%s/%s\n", configs.EnvPicturesBucket(), s3OriginalKey)

		_, err = uploader.Upload(ctx, &s3.PutObjectInput{
			Bucket:      aws.String(configs.EnvPicturesBucket()),
			Key:         aws.String(s3OriginalKey),
			Body:        bytes.NewReader(fileBytes),
			ContentType: aws.String(mimeType),
		})
		if err != nil {
			fmt.Println("Error uploading original to S3:", err)
			errorResponse(rw, fmt.Errorf("error uploading image"), 500)
			return
		}
		fmt.Println("Original image uploaded to S3")

		// Generate and upload thumbnail
		var s3ThumbnailKey string
		if mimeType == "image/heic" || mimeType == "image/heif" {
			// For HEIC, use original as thumbnail
			s3ThumbnailKey = s3OriginalKey
		} else {
			// Decode and resize
			img, err := imaging.Decode(bytes.NewReader(fileBytes))
			if err != nil {
				fmt.Println("Error decoding image:", err)
				s3ThumbnailKey = s3OriginalKey // Fallback to original
			} else {
				thumbnail := imaging.Resize(img, 585, 0, imaging.Linear)

				var thumbBuf bytes.Buffer
				switch extension {
				case "jpeg":
					imaging.Encode(&thumbBuf, thumbnail, imaging.JPEG)
				case "png":
					imaging.Encode(&thumbBuf, thumbnail, imaging.PNG)
				case "webp":
					imaging.Encode(&thumbBuf, thumbnail, imaging.JPEG)
				default:
					imaging.Encode(&thumbBuf, thumbnail, imaging.JPEG)
				}

				s3ThumbnailKey = fmt.Sprintf("%s/%s_thumb.%s", userID, imageID, extension)

				fmt.Printf("Uploading thumbnail to S3: s3://%s/%s\n", configs.EnvPicturesBucket(), s3ThumbnailKey)

				_, err = uploader.Upload(ctx, &s3.PutObjectInput{
					Bucket:      aws.String(configs.EnvPicturesBucket()),
					Key:         aws.String(s3ThumbnailKey),
					Body:        &thumbBuf,
					ContentType: aws.String(mimeType),
				})
				if err != nil {
					fmt.Println("Error uploading thumbnail:", err)
					s3ThumbnailKey = s3OriginalKey // Fallback
				} else {
					fmt.Println("Thumbnail uploaded to S3")
				}
			}
		}

		// Build CDN URLs
		cdnURL := configs.EnvPicturesCDNURL()
		originalURL := cdnURL + "/" + s3OriginalKey
		thumbnailURL := cdnURL + "/" + s3ThumbnailKey

		newPostPic := models.Content{
			UserID:       userID,
			Poster:       userID,
			Title:        title,
			Description:  description,
			Location:     originalURL,
			Posting:      thumbnailURL,
			S3RawKey:     s3OriginalKey,
			ThumbnailKey: s3ThumbnailKey,
			DateCreated:  time.Now(),
			Show:         show,
			IsPayPerView: ispayperview,
			IsDeleted:    isdeleted,
			PPVPrice:     price,
			Type:         TYPE_PIC,
			Visibility:   visibility,
		}

		newPostPic.Tags = strings.Split(tags, ",")
		for i, s := range newPostPic.Tags {
			newPostPic.Tags[i] = strings.TrimSpace(s)
		}

		// Insert to DB
		result, err := getContentCollection().InsertOne(ctx, newPostPic)
		fmt.Println(result)
		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			response := responses.ContentResponse{
				Status:  http.StatusInternalServerError,
				Message: "error",
				Data:    map[string]interface{}{"data": err.Error()},
			}
			json.NewEncoder(rw).Encode(response)
			return
		}

		// Return success
		rw.WriteHeader(http.StatusCreated)
		response := responses.ContentResponse{
			Status:  http.StatusCreated,
			Message: "success",
			Data:    map[string]interface{}{"data": result},
		}
		json.NewEncoder(rw).Encode(response)
	}
}

func PostPicWithBody() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		vars := mux.Vars(r)
		userID := vars["UserID"]
		tags := vars["Tags"]
		visibility := vars["Visibility"]
		show, _ := strconv.ParseBool(vars["Show"])
		ispayperview, _ := strconv.ParseBool(vars["IsPayPerView"])
		isdeleted, _ := strconv.ParseBool(vars["IsDeleted"])
		ppvprice := vars["PPVPrice"]
		price, err := strconv.ParseFloat(ppvprice, 64)
		if err != nil {
			fmt.Println("invalid price in PPVPrice")
			price = 0
		}

		if visibility != VISIBILITY_FOLLOWERS {
			visibility = VISIBILITY_EVERYONE
		}

		// Read JSON from formValue("data")
		jsonData := r.FormValue("data")
		if jsonData == "" {
			errorResponse(rw, fmt.Errorf("missing data"), 400)
			return
		}

		var contentBody models.ContentBody
		if err := json.Unmarshal([]byte(jsonData), &contentBody); err != nil {
			errorResponse(rw, err, 400)
			return
		}

		title := contentBody.Title
		description := contentBody.Description

		// Generate unique ID
		imageID := strings.Replace(uuid.New().String(), "-", "", -1)

		// Parse multipart form
		r.ParseMultipartForm(10 * MB)
		r.Body = http.MaxBytesReader(rw, r.Body, 10*MB)

		// Retrieve file
		file, _, err := r.FormFile("file")
		if err != nil {
			fmt.Println(err)
			errorResponse(rw, err, 400)
			return
		}
		defer file.Close()

		// Read entire file for MIME detection
		fileBytes, err := io.ReadAll(file)
		if err != nil {
			fmt.Println("failed to read file data:", err)
			errorResponse(rw, fmt.Errorf("failed to read file"), 500)
			return
		}

		detectedMIME := mimetype.Detect(fileBytes)
		mimeType := detectedMIME.String()
		fmt.Println("Detected MIME:", mimeType)

		extension := ""
		switch mimeType {
		case "image/png":
			extension = "png"
		case "image/jpeg":
			extension = "jpeg"
		case "image/webp":
			extension = "webp"
		case "image/heic":
			extension = "heic"
		case "image/heif":
			extension = "heic"
		default:
			http.Error(rw, "This file type is not allowed for images", http.StatusBadRequest)
			return
		}

		uploader := configs.GetS3Uploader()
		s3OriginalKey := fmt.Sprintf("%s/%s.%s", userID, imageID, extension)

		fmt.Printf("Uploading original image to S3: s3://%s/%s\n", configs.EnvPicturesBucket(), s3OriginalKey)

		_, err = uploader.Upload(ctx, &s3.PutObjectInput{
			Bucket:      aws.String(configs.EnvPicturesBucket()),
			Key:         aws.String(s3OriginalKey),
			Body:        bytes.NewReader(fileBytes),
			ContentType: aws.String(mimeType),
		})
		if err != nil {
			fmt.Println("Error uploading original to S3:", err)
			errorResponse(rw, fmt.Errorf("error uploading image"), 500)
			return
		}
		fmt.Println("Original image uploaded to S3")

		var s3ThumbnailKey string
		if mimeType == "image/heic" || mimeType == "image/heif" {
			s3ThumbnailKey = s3OriginalKey
		} else {

			img, err := imaging.Decode(bytes.NewReader(fileBytes))
			if err != nil {
				fmt.Println("Error decoding image:", err)
				s3ThumbnailKey = s3OriginalKey
			} else {
				thumbnail := imaging.Resize(img, 585, 0, imaging.Linear)

				var thumbBuf bytes.Buffer
				switch extension {
				case "jpeg":
					imaging.Encode(&thumbBuf, thumbnail, imaging.JPEG)
				case "png":
					imaging.Encode(&thumbBuf, thumbnail, imaging.PNG)
				case "webp":
					imaging.Encode(&thumbBuf, thumbnail, imaging.JPEG)
				default:
					imaging.Encode(&thumbBuf, thumbnail, imaging.JPEG)
				}

				s3ThumbnailKey = fmt.Sprintf("%s/%s_thumb.%s", userID, imageID, extension)

				fmt.Printf("Uploading thumbnail to S3: s3://%s/%s\n", configs.EnvPicturesBucket(), s3ThumbnailKey)

				_, err = uploader.Upload(ctx, &s3.PutObjectInput{
					Bucket:      aws.String(configs.EnvPicturesBucket()),
					Key:         aws.String(s3ThumbnailKey),
					Body:        &thumbBuf,
					ContentType: aws.String(mimeType),
				})
				if err != nil {
					fmt.Println("Error uploading thumbnail:", err)
					s3ThumbnailKey = s3OriginalKey
				} else {
					fmt.Println("Thumbnail uploaded to S3")
				}
			}
		}

		cdnURL := configs.EnvPicturesCDNURL()
		originalURL := cdnURL + "/" + s3OriginalKey
		thumbnailURL := cdnURL + "/" + s3ThumbnailKey

		newPostPic := models.Content{
			UserID:       userID,
			Poster:       userID,
			Title:        title,
			Description:  description,
			Location:     originalURL,
			Posting:      thumbnailURL,
			S3RawKey:     s3OriginalKey,
			ThumbnailKey: s3ThumbnailKey,
			DateCreated:  time.Now(),
			Show:         show,
			IsPayPerView: ispayperview,
			IsDeleted:    isdeleted,
			PPVPrice:     price,
			Type:         TYPE_PIC,
			Visibility:   visibility,
		}

		newPostPic.Tags = strings.Split(tags, ",")
		for i, s := range newPostPic.Tags {
			newPostPic.Tags[i] = strings.TrimSpace(s)
		}

		fmt.Print(">>>", newPostPic)
		// Insert into DB
		result, err := getContentCollection().InsertOne(ctx, newPostPic)
		fmt.Println(result)
		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			response := responses.ContentResponse{
				Status:  http.StatusInternalServerError,
				Message: "error",
				Data:    map[string]interface{}{"data": err.Error()},
			}
			json.NewEncoder(rw).Encode(response)
			return
		}

		// Success
		rw.WriteHeader(http.StatusCreated)
		response := responses.ContentResponse{
			Status:  http.StatusCreated,
			Message: "success",
			Data:    map[string]interface{}{"data": result},
		}
		json.NewEncoder(rw).Encode(response)
	}
}

// func PostVideo() http.HandlerFunc {
// 	return func(rw http.ResponseWriter, r *http.Request) {

// 		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Minute)
// 		defer cancel()
// 		vars := mux.Vars(r)
// 		userID := vars["UserID"]
// 		title := vars["Title"]
// 		description := vars["Description"]
// 		tags := vars["Tags"]
// 		show, _ := strconv.ParseBool(vars["Show"])
// 		ispayperview, _ := strconv.ParseBool(vars["IsPayPerView"])
// 		isdeleted, _ := strconv.ParseBool(vars["IsDeleted"])
// 		extension := ""
// 		thumbextension := ""
// 		ffmpegSource := ""
// 		ffmpegTarget := ""
// 		newUuid := uuid.New()
// 		ppvprice := vars["PPVPrice"]
// 		price, err := strconv.ParseFloat(ppvprice, 64)
// 		if err != nil {
// 			fmt.Println("invalid price in PPVPrice")
// 			price = 0
// 		}

// 		newPostVid := models.NewPostVideo{
// 			UserID:       userID,
// 			Title:        title,
// 			Description:  description,
// 			Location:     configs.EnvMediaDir() + "/" + userID + "/videos/",
// 			DateCreated:  time.Now(),
// 			Show:         show,
// 			IsPayPerView: ispayperview,
// 			IsDeleted:    isdeleted,
// 			PPVPrice:     price,
// 		}
// 		newPostVid.Tags = strings.Split(tags, ",")
// 		for i, s := range newPostVid.Tags {
// 			newPostVid.Tags[i] = strings.Trim(s, " ")
// 		}
// 		fmt.Println("Creating folder ..." + newPostVid.Location)
// 		var res = os.MkdirAll(newPostVid.Location, 0777)
// 		if res != nil {
// 			fmt.Println(res)
// 		}
// 		err = os.Chmod(configs.EnvMediaDir()+"/"+userID, 0666)
// 		if err != nil {
// 			fmt.Println(err)
// 		}

// 		err = os.Chmod(configs.EnvMediaDir()+"/"+userID+"/videos", 0666)
// 		if err != nil {
// 			fmt.Println(err)
// 		}
// 		var res1 = os.MkdirAll(newPostVid.Location+strings.Replace(newUuid.String(), "-", "", -1), 0777)
// 		fmt.Println("Creating folder ..." + newPostVid.Location + strings.Replace(newUuid.String(), "-", "", -1))
// 		if res1 != nil {
// 			fmt.Println(res1)
// 		}
// 		err = os.Chmod(newPostVid.Location+strings.Replace(newUuid.String(), "-", "", -1), 0666)
// 		if err != nil {
// 			fmt.Println(err)
// 		}

// 		r.ParseMultipartForm(1024 * 20 * MB)
// 		r.Body = http.MaxBytesReader(rw, r.Body, 1024*20*MB)
// 		file, _, err := r.FormFile("video")
// 		if err != nil {
// 			fmt.Println(err)
// 		}
// 		thumb, _, err := r.FormFile("thumb")
// 		if err != nil {
// 			fmt.Println(err)
// 		}

// 		fileHeader := make([]byte, 512)
// 		if _, err := file.Read(fileHeader); err != nil {
// 			return
// 		}
// 		if _, err := file.Seek(0, 0); err != nil {
// 			return
// 		}

// 		thumbHeader := make([]byte, 512)
// 		if _, err := thumb.Read(thumbHeader); err != nil {
// 			return
// 		}
// 		if _, err := thumb.Seek(0, 0); err != nil {
// 			return
// 		}

// 		mime := http.DetectContentType(fileHeader)
// 		mimeThumb := http.DetectContentType(thumbHeader)

// 		fmt.Println(mime)
// 		switch mime {
// 		case "video/mp4":
// 			extension = "mp4"
// 		case "video/mkv":
// 			extension = "mkv"
// 		case "video/avi":
// 			extension = "avi"
// 		case "video/3gp":
// 			extension = "3gp"
// 		case "video/mov":
// 			extension = "mov"
// 		default:
// 			http.Error(rw, "This file type for video is not allowed.", http.StatusBadRequest)
// 			return
// 		}
// 		//deleting uploaded video after transcoding is finished
// 		//defer CleanUp(newPostVid.Location + strings.Replace(newUuid.String(), "-", "", -1) + "." + extension)

// 		//transcoding after all files are closed
// 		ffmpegSource = newPostVid.Location + strings.Replace(newUuid.String(), "-", "", -1) + "." + extension
// 		ffmpegTarget = newPostVid.Location + strings.Replace(newUuid.String(), "-", "", -1)
// 		fmt.Println(ffmpegSource, ffmpegTarget)
// 		//start transcoding process at the end of this method
// 		/* command := []string{
// 			"./script/create-vod-hls.sh",
// 			ffmpegSource,
// 			ffmpegTarget,
// 		}
// 		defer Execute("./script/create-vod-hls.sh", command) */
// 		//end transcoding instructions
// 		switch mimeThumb {
// 		case "image/png":
// 			thumbextension = "png"
// 		case "image/jpeg":
// 			thumbextension = "jpeg"
// 		case "image/webp":
// 			thumbextension = "webp"

// 		default:
// 			http.Error(rw, "This file type is not allowed for thumbnails", http.StatusBadRequest)
// 			return
// 		}

// 		defer file.Close()
// 		defer thumb.Close()

// 		f, err := os.OpenFile(newPostVid.Location+strings.Replace(newUuid.String(), "-", "", -1)+"."+extension, os.O_WRONLY|os.O_CREATE, 0666)
// 		if err != nil {
// 			fmt.Println("Can't create a file for video at....")
// 			fmt.Println("----" + newPostVid.Location + strings.Replace(newUuid.String(), "-", "", -1) + "." + extension + "----")
// 			fmt.Println(err)
// 		}
// 		io.Copy(f, file)
// 		defer f.Close()

// 		srcPRT, _, err := image.Decode(thumb)
// 		if err != nil {
// 			fmt.Println(err)
// 		}
// 		srcPRT = imaging.Resize(srcPRT, 585, 0, imaging.Linear)
// 		err = imaging.Save(srcPRT, newPostVid.Location+strings.Replace(newUuid.String(), "-", "", -1)+"thumb."+thumbextension)
// 		if err != nil {
// 			fmt.Println(err)
// 		}

// 		newPostVid.Location = newPostVid.Location + strings.Replace(newUuid.String(), "-", "", -1) + "/"
// 		result, err := getVideosCollection().InsertOne(ctx, newPostVid)
// 		fmt.Println(result)
// 		if err != nil {
// 			rw.WriteHeader(http.StatusInternalServerError)
// 			response := responses.ContentResponse{Status: http.StatusInternalServerError, Message: "error", Data: map[string]interface{}{"data": err.Error()}}
// 			json.NewEncoder(rw).Encode(response)
// 			return
// 		}
// 		rw.WriteHeader(http.StatusCreated)
// 		response := responses.ContentResponse{Status: http.StatusCreated, Message: "success", Data: map[string]interface{}{"data": result}}
// 		json.NewEncoder(rw).Encode(response)

// 	}
// }

func PostVideo() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Minute)
		defer cancel()

		fmt.Print("CHECKPOINT 1")

		// Parse parameters
		vars := mux.Vars(r)
		userID := vars["UserID"]
		title := vars["Title"]
		description := vars["Description"]
		tags := vars["Tags"]
		show, _ := strconv.ParseBool(vars["Show"])
		ispayperview, _ := strconv.ParseBool(vars["IsPayPerView"])
		isdeleted, _ := strconv.ParseBool(vars["IsDeleted"])
		ppvprice := vars["PPVPrice"]
		price, err := strconv.ParseFloat(ppvprice, 64)
		if err != nil {
			fmt.Println("Invalid price in PPVPrice")
			price = 0
		}

		// Generate unique video ID
		newUuid := uuid.New()
		videoID := strings.Replace(newUuid.String(), "-", "", -1)

		// Parse multipart form
		err = r.ParseMultipartForm(1024 * 20 * MB)
		if err != nil {
			http.Error(rw, "Error parsing form", http.StatusBadRequest)
			return
		}
		r.Body = http.MaxBytesReader(rw, r.Body, 1024*20*MB)

		// Get video file
		file, _, err := r.FormFile("video")
		if err != nil {
			fmt.Println("Error getting video file:", err)
			http.Error(rw, "Error reading video file", http.StatusBadRequest)
			return
		}
		defer file.Close()

		// Validate video file type
		fileHeader := make([]byte, 512)
		if _, err := file.Read(fileHeader); err != nil {
			http.Error(rw, "Error reading file", http.StatusBadRequest)
			return
		}
		if _, err := file.Seek(0, 0); err != nil {
			http.Error(rw, "Error reading file", http.StatusBadRequest)
			return
		}

		mime := http.DetectContentType(fileHeader)
		extension := ""

		fmt.Println("Video MIME type:", mime)
		switch mime {
		case "video/mp4":
			extension = "mp4"
		case "video/quicktime":
			extension = "mov"
		case "video/x-msvideo":
			extension = "avi"
		case "video/x-matroska":
			extension = "mkv"
		case "video/3gp":
			extension = "3gp"
		default:
			http.Error(rw, "This file type for video is not allowed: "+mime, http.StatusBadRequest)
			return
		}

		uploader := configs.GetS3Uploader()

		fmt.Print("UPLOADER>>", uploader)

		// Upload raw video to S3
		s3VideoKey := fmt.Sprintf("%s/%s.%s", userID, videoID, extension)
		fmt.Printf("Uploading video to S3: s3://%s/%s\n", configs.EnvRawBucket(), s3VideoKey)

		_, err = uploader.Upload(ctx, &s3.PutObjectInput{
			Bucket:      aws.String(configs.EnvRawBucket()),
			Key:         aws.String(s3VideoKey),
			Body:        file,
			ContentType: aws.String(mime),
		})
		if err != nil {
			fmt.Println("Error uploading video to S3:", err)
			http.Error(rw, "Error uploading video to S3", http.StatusInternalServerError)
			return
		}
		fmt.Println("✅ Video uploaded to S3 successfully")

		/**
		  insert a new metadata for video in the database
		  HLSURL will be updated by the ec2 processor server in aws  along with status and thumbnail path
		*/
		newPostVid := models.NewPostVideo{
			VideoID:      videoID,
			UserID:       userID,
			Title:        title,
			Description:  description,
			S3RawKey:     s3VideoKey,
			ThumbnailKey: "",
			HLSURL:       "",
			Status:       "processing",
			DateCreated:  time.Now(),
			Show:         show,
			IsPayPerView: ispayperview,
			IsDeleted:    isdeleted,
			PPVPrice:     price,
		}

		// Parse tags
		newPostVid.Tags = strings.Split(tags, ",")
		for i, s := range newPostVid.Tags {
			newPostVid.Tags[i] = strings.TrimSpace(s)
		}

		fmt.Println("TEST MONGO BD INSERT")

		result, err := getVideosCollection().InsertOne(ctx, newPostVid)
		fmt.Println("MongoDB insert result:", result)
		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			response := responses.ContentResponse{
				Status:  http.StatusInternalServerError,
				Message: "error",
				Data:    map[string]interface{}{"data": err.Error()},
			}
			json.NewEncoder(rw).Encode(response)
			return
		}

		fmt.Println("TEST MONGO BD INSERT sucessss")

		// Return success
		rw.WriteHeader(http.StatusCreated)
		response := responses.ContentResponse{
			Status:  http.StatusCreated,
			Message: "success",
			Data: map[string]interface{}{
				"video_id": videoID,
				"status":   "processing",
				"message":  "Video uploaded successfully and is being processed",
			},
		}
		json.NewEncoder(rw).Encode(response)
	}
}

func PostText() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Minute)
		defer cancel()
		vars := mux.Vars(r)
		userID := vars["UserID"]
		title := vars["Title"]
		description := vars["Description"]
		tags := vars["Tags"]
		visibility := vars["Visibility"]
		show, _ := strconv.ParseBool(vars["Show"])
		ispayperview, _ := strconv.ParseBool(vars["IsPayPerView"])
		isdeleted, _ := strconv.ParseBool(vars["IsDeleted"])
		ppvprice := vars["PPVPrice"]
		postingEncoded := vars["Posting"]
		postingDecoded, err := url.QueryUnescape(postingEncoded)
		price, err := strconv.ParseFloat(ppvprice, 64)
		if err != nil {
			fmt.Println("invalid price in PPVPrice")
			price = 0
		}
		if visibility != VISIBILITY_FOLLOWERS {
			visibility = VISIBILITY_EVERYONE
		}
		newTextContent := models.Content{
			UserID:       userID,
			Poster:       userID,
			Type:         TYPE_TEXT,
			Title:        title,
			Description:  description,
			DateCreated:  time.Now(),
			Show:         show,
			IsPayPerView: ispayperview,
			IsDeleted:    isdeleted,
			PPVPrice:     price,
			Posting:      postingDecoded,
			Visibility:   visibility,
		}
		newTextContent.Tags = strings.Split(tags, ",")
		for i, s := range newTextContent.Tags {
			newTextContent.Tags[i] = strings.Trim(s, " ")
		}
		result, err := getContentCollection().InsertOne(ctx, newTextContent)
		if err != nil {
			fmt.Println(err)
			errorResponse(rw, err, 200)
			return
		}
		successResponse(rw, result.InsertedID)
		//go insertInREDISGetContentByUserID(userID)
	}
}

func PostTextWithBody() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Minute)
		defer cancel()
		vars := mux.Vars(r)
		userID := vars["UserID"]
		tags := vars["Tags"]
		visibility := vars["Visibility"]
		show, _ := strconv.ParseBool(vars["Show"])
		ispayperview, _ := strconv.ParseBool(vars["IsPayPerView"])
		isdeleted, _ := strconv.ParseBool(vars["IsDeleted"])
		ppvprice := vars["PPVPrice"]
		price, err := strconv.ParseFloat(ppvprice, 64)
		if err != nil {
			fmt.Println("invalid price in PPVPrice")
			price = 0
		}
		if visibility != VISIBILITY_FOLLOWERS {
			visibility = VISIBILITY_EVERYONE
		}
		contentBody := models.ContentBody{}

		err = json.NewDecoder(r.Body).Decode(&contentBody)
		if err != nil {
			errorResponse(rw, fmt.Errorf("bad request"), 200)
			return
		}

		title := contentBody.Title
		description := contentBody.Description
		postingDecoded := contentBody.Posting

		newTextContent := models.Content{
			UserID:       userID,
			Poster:       userID,
			Type:         TYPE_TEXT,
			Title:        title,
			Description:  description,
			DateCreated:  time.Now(),
			Show:         show,
			IsPayPerView: ispayperview,
			IsDeleted:    isdeleted,
			PPVPrice:     price,
			Posting:      postingDecoded,
			Visibility:   visibility,
		}
		newTextContent.Tags = strings.Split(tags, ",")
		for i, s := range newTextContent.Tags {
			newTextContent.Tags[i] = strings.Trim(s, " ")
		}
		result, err := getContentCollection().InsertOne(ctx, newTextContent)
		if err != nil {
			fmt.Println(err)
			errorResponse(rw, err, 200)
			return
		}
		successResponse(rw, result.InsertedID)
		//go insertInREDISGetContentByUserID(userID)
	}
}

func PostVideoNT() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Minute)
		defer cancel()
		vars := mux.Vars(r)
		userID := vars["UserID"]
		title := vars["Title"]
		visibility := vars["Visibility"]
		description := vars["Description"]
		tags := vars["Tags"]
		show, _ := strconv.ParseBool(vars["Show"])
		ispayperview, _ := strconv.ParseBool(vars["IsPayPerView"])
		isdeleted, _ := strconv.ParseBool(vars["IsDeleted"])
		extension := ""
		ffmpegSource := ""
		ffmpegTarget := ""
		newUuid := uuid.New()
		ppvprice := vars["PPVPrice"]
		price, err := strconv.ParseFloat(ppvprice, 64)
		if err != nil {
			fmt.Println("invalid price in PPVPrice")
			price = 0
		}
		if visibility != VISIBILITY_FOLLOWERS {
			visibility = VISIBILITY_EVERYONE
		}
		newPostVid := models.Content{
			UserID:       userID,
			Poster:       userID,
			Title:        title,
			Description:  description,
			Location:     configs.EnvMediaDir() + "/" + userID + "/videos/",
			DateCreated:  time.Now(),
			Show:         show,
			IsPayPerView: ispayperview,
			IsDeleted:    isdeleted,
			PPVPrice:     price,
			Type:         TYPE_VIDEO,
			Posting:      "",
			Visibility:   visibility,
			Transcoding:  TRANSCODING_PENDING,
		}
		newPostVid.Tags = strings.Split(tags, ",")
		for i, s := range newPostVid.Tags {
			newPostVid.Tags[i] = strings.Trim(s, " ")
		}
		fmt.Println("Creating folder ..." + newPostVid.Location)
		var res = os.MkdirAll(newPostVid.Location, 0777)
		if res != nil {
			fmt.Println(res)
		}
		err = os.Chmod(configs.EnvMediaDir()+"/"+userID, 0666)
		if err != nil {
			fmt.Println(err)
		}

		err = os.Chmod(configs.EnvMediaDir()+"/"+userID+"/videos", 0666)
		if err != nil {
			fmt.Println(err)
		}
		var res1 = os.MkdirAll(newPostVid.Location+strings.Replace(newUuid.String(), "-", "", -1), 0777)
		fmt.Println("Creating folder ..." + newPostVid.Location + strings.Replace(newUuid.String(), "-", "", -1))
		if res1 != nil {
			fmt.Println(res1)
		}
		err = os.Chmod(newPostVid.Location+strings.Replace(newUuid.String(), "-", "", -1), 0666)
		if err != nil {
			fmt.Println(err)
		}

		r.ParseMultipartForm(1024 * 20 * MB)
		r.Body = http.MaxBytesReader(rw, r.Body, 1024*20*MB)
		file, fheader, err := r.FormFile("video")
		if err != nil {
			fmt.Println(err)
		}

		fileHeader := make([]byte, 512)
		if _, err := file.Read(fileHeader); err != nil {
			return
		}
		if _, err := file.Seek(0, 0); err != nil {
			return
		}

		mime := http.DetectContentType(fileHeader)

		fmt.Println(mime)
		switch mime {
		case "video/mp4":
			extension = "mp4"
		case "video/mkv":
			extension = "mkv"
		case "video/avi":
			extension = "avi"
		case "video/3gp":
			extension = "3gp"
		case "video/mov":
			extension = "mov"
		case "video/hevc", "video/h265":
			extension = "hevc"
		case "application/octet-stream":
			// MIME sniffer couldn’t decide – trust the filename
			ext := strings.ToLower(filepath.Ext(fheader.Filename)) // “.mp4”, “.h265”, …
			switch ext {
			case ".mp4", ".mkv", ".avi", ".3gp", ".mov",
				".hevc", ".h265", ".265":
				extension = ext[1:] // strip the leading “.”
			default:
				http.Error(rw, "This file type for video is not allowed.", http.StatusBadRequest)
				return
			}

		default:
			http.Error(rw, "This file type for video is not allowed.", http.StatusBadRequest)
			return
		}

		ffmpegSource = newPostVid.Location + strings.Replace(newUuid.String(), "-", "", -1) + "." + extension
		ffmpegTarget = newPostVid.Location + strings.Replace(newUuid.String(), "-", "", -1)
		fmt.Println(ffmpegSource, ffmpegTarget)

		defer file.Close()

		f, err := os.OpenFile(configs.INITMEDIADIR()+userID+"-"+strings.Replace(newUuid.String(), "-", "", -1)+"."+extension, os.O_WRONLY|os.O_CREATE, 0666)
		if err != nil {
			fmt.Println("Can't create a file for video at....")
			fmt.Println("----" + newPostVid.Location + strings.Replace(newUuid.String(), "-", "", -1) + "." + extension + "----")
			fmt.Println(err)
		}
		io.Copy(f, file)
		defer f.Close()

		newPostVid.Location = newPostVid.Location + strings.Replace(newUuid.String(), "-", "", -1) + "/"
		result, err := getContentCollection().InsertOne(ctx, newPostVid)
		fmt.Println(result)
		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			response := responses.ContentResponse{Status: http.StatusInternalServerError, Message: "error", Data: map[string]interface{}{"data": err.Error()}}
			json.NewEncoder(rw).Encode(response)
			return
		}
		rw.WriteHeader(http.StatusCreated)
		response := responses.ContentResponse{Status: http.StatusCreated, Message: "success", Data: map[string]interface{}{"data": result}}
		json.NewEncoder(rw).Encode(response)
		//go insertInREDISGetContentByUserID(userID)

	}
}

func PostVideoNTWithBody() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Minute)
		defer cancel()

		vars := mux.Vars(r)
		userID := vars["UserID"]
		visibility := vars["Visibility"]
		tags := vars["Tags"]
		show, _ := strconv.ParseBool(vars["Show"])
		ispayperview, _ := strconv.ParseBool(vars["IsPayPerView"])
		isdeleted, _ := strconv.ParseBool(vars["IsDeleted"])
		ppvprice := vars["PPVPrice"]
		price, err := strconv.ParseFloat(ppvprice, 64)
		if err != nil {
			price = 0
		}

		if visibility != VISIBILITY_FOLLOWERS {
			visibility = VISIBILITY_EVERYONE
		}

		jsonData := r.FormValue("data")
		if jsonData == "" {
			errorResponse(rw, fmt.Errorf("missing data"), 400)
			return
		}

		var contentBody models.ContentBody
		if err := json.Unmarshal([]byte(jsonData), &contentBody); err != nil {
			errorResponse(rw, err, 400)
			return
		}

		title := contentBody.Title
		description := contentBody.Description

		// Generate unique video ID
		videoID := strings.Replace(uuid.New().String(), "-", "", -1)

		// Parse multipart form
		r.ParseMultipartForm(1024 * 20 * MB)
		r.Body = http.MaxBytesReader(rw, r.Body, 1024*20*MB)

		file, fheader, err := r.FormFile("video")
		if err != nil {
			errorResponse(rw, fmt.Errorf("error reading video file"), 400)
			return
		}
		defer file.Close()

		fileHeader := make([]byte, 512)
		file.Read(fileHeader)
		file.Seek(0, 0)

		mime := http.DetectContentType(fileHeader)
		extension := ""

		switch mime {
		case "video/mp4":
			extension = "mp4"
		case "video/quicktime":
			extension = "mov"
		case "video/x-msvideo":
			extension = "avi"
		case "video/x-matroska":
			extension = "mkv"
		case "video/3gpp":
			extension = "3gp"
		case "video/hevc", "video/h265":
			extension = "hevc"
		case "application/octet-stream":
			ext := strings.ToLower(filepath.Ext(fheader.Filename))
			switch ext {
			case ".mp4", ".mkv", ".avi", ".3gp", ".mov", ".hevc", ".h265", ".265":
				extension = ext[1:]
			default:
				http.Error(rw, "Invalid video file type", http.StatusBadRequest)
				return
			}
		default:
			http.Error(rw, "Invalid video file type", http.StatusBadRequest)
			return
		}

		uploader := configs.GetS3Uploader()
		s3VideoKey := fmt.Sprintf("%s/%s.%s", userID, videoID, extension)

		fmt.Printf("Uploading video to S3: s3://%s/%s\n", configs.EnvRawBucket(), s3VideoKey)

		_, err = uploader.Upload(ctx, &s3.PutObjectInput{
			Bucket:      aws.String(configs.EnvRawBucket()),
			Key:         aws.String(s3VideoKey),
			Body:        file,
			ContentType: aws.String(mime),
		})
		if err != nil {
			fmt.Println("Error uploading to S3:", err)
			errorResponse(rw, fmt.Errorf("error uploading video"), 500)
			return
		}
		fmt.Println("Video uploaded to S3")

		newPostVid := models.Content{
			VideoID:      videoID,
			UserID:       userID,
			Poster:       userID,
			Title:        title,
			Description:  description,
			S3RawKey:     s3VideoKey,
			ThumbnailKey: "",
			HLSURL:       "",
			DateCreated:  time.Now(),
			Show:         show,
			IsPayPerView: ispayperview,
			IsDeleted:    isdeleted,
			PPVPrice:     price,
			Type:         TYPE_VIDEO,
			Posting:      "",
			Visibility:   visibility,
			Transcoding:  TRANSCODING_PENDING,
		}

		newPostVid.Tags = strings.Split(tags, ",")
		for i, s := range newPostVid.Tags {
			newPostVid.Tags[i] = strings.TrimSpace(s)
		}

		result, err := getContentCollection().InsertOne(ctx, newPostVid)
		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			response := responses.ContentResponse{
				Status:  http.StatusInternalServerError,
				Message: "error",
				Data:    map[string]interface{}{"data": err.Error()},
			}
			json.NewEncoder(rw).Encode(response)
			return
		}

		fmt.Print(result)

		rw.WriteHeader(http.StatusCreated)
		response := responses.ContentResponse{
			Status:  http.StatusCreated,
			Message: "success",
			Data: map[string]interface{}{
				"video_id": videoID,
				"status":   "processing",
			},
		}
		json.NewEncoder(rw).Encode(response)
	}
}

func StartStream() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Minute)
		defer cancel()
		vars := mux.Vars(r)
		userID := vars["UserID"]
		title := vars["Title"]
		visibility := vars["Visibility"]
		description := vars["Description"]
		tags := vars["Tags"]
		show, _ := strconv.ParseBool(vars["Show"])
		ispayperview, _ := strconv.ParseBool(vars["IsPayPerView"])
		isdeleted, _ := strconv.ParseBool(vars["IsDeleted"])

		ppvprice := vars["PPVPrice"]
		price, err := strconv.ParseFloat(ppvprice, 64)
		if err != nil {
			fmt.Println("invalid price in PPVPrice")
			price = 0
		}

		if visibility != VISIBILITY_FOLLOWERS {
			visibility = VISIBILITY_EVERYONE
		}

		// Generate unique stream key
		streamKey := strings.Replace(uuid.New().String(), "-", "", -1)

		// Streaming server IP (replace with your actual IP or use env variable)
		streamingServerIP := "13.50.17.68" // TODO: Move to configs.EnvStreamingServer()

		newStream := models.Content{
			UserID:       userID,
			Poster:       userID,
			Title:        title,
			Description:  description,
			Location:     configs.EnvMediaDir() + "/" + userID + "/videos/",
			DateCreated:  time.Now(),
			Show:         show,
			IsPayPerView: ispayperview,
			IsDeleted:    isdeleted,
			PPVPrice:     price,
			Type:         TYPE_STREAM,
			Posting:      "",
			Visibility:   visibility,

			// Live streaming fields
			StreamKey:   streamKey,
			RTMPUrl:     fmt.Sprintf("rtmp://%s/live/%s", streamingServerIP, streamKey),
			HLSURL:      fmt.Sprintf("http://%s/hls/%s/index.m3u8", streamingServerIP, streamKey),
			IsLive:      false, // Will be set to true when streaming actually starts
			ViewerCount: 0,
		}

		newStream.Tags = strings.Split(tags, ",")
		for i, s := range newStream.Tags {
			newStream.Tags[i] = strings.Trim(s, " ")
		}

		result, err := getContentCollection().InsertOne(ctx, newStream)
		if err != nil {
			rw.WriteHeader(http.StatusConflict)
			errresponse := responses.ContentResponse{Status: http.StatusConflict, Message: "error"}
			json.NewEncoder(rw).Encode(errresponse)
			return
		}

		go func() {
			// THE NGINX PUBLISHER WILL SEND NOTIFICATION NOT THIS GUY @CHECK
			// time.Sleep(30 * time.Second)
			// contentID := result.InsertedID.(primitive.ObjectID).Hex()
			// sendLiveStartedNotification(userID, contentID)
		}()

		rw.WriteHeader(http.StatusCreated)
		response := responses.ContentResponse{
			Status:  http.StatusCreated,
			Message: "success",
			Data: map[string]interface{}{
				"content_id": result.InsertedID,
				"stream_key": streamKey,
				"rtmp_url":   newStream.RTMPUrl,
				"hls_url":    newStream.HLSURL,
			},
		}
		json.NewEncoder(rw).Encode(response)
	}
}

// KNOW BASED ON on_publish event from rtmp nginx when live stream started
func HandleStreamPublish() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(rw, "invalid form", http.StatusBadRequest)
			return
		}

		streamKey := r.FormValue("name")

		if streamKey == "" {
			http.Error(rw, "missing stream key", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		var stream models.Content
		err := getContentCollection().FindOne(ctx, bson.M{
			"stream_key": streamKey,
			"type":       TYPE_STREAM,
		}).Decode(&stream)

		if err != nil {
			http.Error(rw, "unauthorized", http.StatusNotFound)
			return
		}

		now := time.Now()
		getContentCollection().UpdateOne(
			ctx,
			bson.M{"_id": stream.Id},
			bson.M{"$set": bson.M{
				"is_live":        true,
				"stream_started": now,
			}},
		)

		// Send notifications
		go func() {
			contentID := stream.Id.Hex()
			hlsURL := fmt.Sprintf("http://13.50.17.68/hls/%s/index.m3u8", streamKey)

			fmt.Printf("Checking HLS availability: %s\n", hlsURL)

			if waitForHLSViaHTTP(hlsURL, 30*time.Second) {
				fmt.Printf(" HLS ready! Sending notifications for %s\n", streamKey)
				sendLiveStartedNotification(stream.UserID, contentID)
			} else {
				fmt.Printf(" HLS timeout, sending notifications anywayyy for %s\n", streamKey)
				sendLiveStartedNotification(stream.UserID, contentID)
			}
		}()

		fmt.Println(stream.UserID + " IS NOW LIVEE ")

		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(map[string]string{
			"user_id":    stream.UserID,
			"stream_key": streamKey,
		})
	}
}

func HandleStreamPublishDone() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(rw, "invalid form", http.StatusBadRequest)
			return
		}

		streamKey := r.FormValue("name")

		if streamKey == "" {
			http.Error(rw, "missing stream key", http.StatusBadRequest)
			return
		}

		fmt.Printf("Stream ENDED: %s\n", streamKey)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		var stream models.Content
		err := getContentCollection().FindOne(ctx, bson.M{
			"stream_key": streamKey,
			"type":       TYPE_STREAM,
		}).Decode(&stream)

		if err != nil {
			fmt.Printf("Stream not found: %s\n", streamKey)
			rw.WriteHeader(http.StatusOK)
			return
		}

		// Mark stream as ended
		now := time.Now()

		// Build the recording URL
		recordingURL := fmt.Sprintf("%s/streams/%s/%s/playlist.m3u8",
			configs.EnvCDNURL(), stream.UserID, streamKey)

		thumbnailURL := fmt.Sprintf("%s/streams/%s/%s/thumbnail.jpg",
			configs.EnvCDNURL(), stream.UserID, streamKey)

		update := bson.M{
			"$set": bson.M{
				"is_live":       false,
				"stream_ended":  now,
				"hls_url":       recordingURL,
				"thumbnail_key": thumbnailURL,
				"posting":       recordingURL,
				"has_recording": true,
				"status":        "ready",
				"transcoding":   TRANSCODING_DONE,
			},
		}

		_, err = getContentCollection().UpdateOne(
			ctx,
			bson.M{"_id": stream.Id},
			update,
		)

		if err != nil {
			fmt.Println("Error updating stream:", err)
		}

		fmt.Printf("Stream finalized. Recording at: %s\n", recordingURL)

		rw.WriteHeader(http.StatusOK)
	}
}

func waitForHLSViaHTTP(hlsURL string, maxWait time.Duration) bool {
	client := &http.Client{
		Timeout: 3 * time.Second,
	}

	deadline := time.Now().Add(maxWait)

	// Exponential backoff: start fast, then slow down
	waitDuration := 300 * time.Millisecond // Start at 300ms
	maxBackoff := 2 * time.Second          // Max 2 seconds between checks
	minSegments := 2

	attemptCount := 0

	for time.Now().Before(deadline) {
		attemptCount++

		resp, err := client.Get(hlsURL)

		if err == nil && resp.StatusCode == http.StatusOK {
			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()

			if err == nil {
				playlist := string(body)
				segmentCount := strings.Count(playlist, ".ts")

				if segmentCount >= minSegments {
					fmt.Printf(" HLS ready after %.1fs (%d attempts, %d segments)\n",
						time.Since(deadline.Add(-maxWait)).Seconds(),
						attemptCount,
						segmentCount)
					return true
				}

				fmt.Printf(" Attempt %d: %d/%d segments (waiting %v)\n",
					attemptCount, segmentCount, minSegments, waitDuration)
			}
		} else if resp != nil {
			resp.Body.Close()
		}

		// Wait before next attempt
		time.Sleep(waitDuration)

		// Exponential backoff
		waitDuration = time.Duration(float64(waitDuration) * 1.5)
		if waitDuration > maxBackoff {
			waitDuration = maxBackoff
		}
	}

	fmt.Printf("HLS timeout after %.1fs (%d attempts)\n",
		maxWait.Seconds(), attemptCount)
	return false
}

func GetStreamUserID() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		streamKey := r.URL.Query().Get("stream_key")
		if streamKey == "" {
			http.Error(rw, "stream_key required", http.StatusBadRequest)
			return
		}

		var stream models.Content
		err := getContentCollection().FindOne(ctx, bson.M{
			"stream_key": streamKey,
			"type":       TYPE_STREAM,
		}).Decode(&stream)

		if err != nil {
			http.Error(rw, "stream not found", http.StatusNotFound)
			return
		}

		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(map[string]string{
			"user_id":    stream.UserID,
			"stream_key": streamKey,
		})
	}
}

func EndView() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		vars := mux.Vars(r)
		media := vars["MediaID"]
		viewerid := vars["ViewerID"]

		ctx := context.Background()

		// Remove from Redis
		redisKey := fmt.Sprintf("stream:%s:viewer:%s", media, viewerid)
		configs.GetRedisClient().Del(ctx, redisKey)

		// Update viewer count
		go updateLiveViewerCount(media)

		successResponse(w, map[string]interface{}{"message": "left stream"})
	}
}

func StartView() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		vars := mux.Vars(r)
		media := vars["MediaID"]
		viewerid := vars["ViewerID"]

		ctx := context.Background()

		// Check if it's a live stream
		objID, err := primitive.ObjectIDFromHex(media)
		if err != nil {
			errorResponse(w, err, 400)
			return
		}

		var content models.Content
		err = getContentCollection().FindOne(ctx, bson.M{"_id": objID}).Decode(&content)
		if err != nil {
			errorResponse(w, err, 404)
			return
		}

		fmt.Println("content", content)

		// Only track if it's a live stream
		if content.Type == TYPE_STREAM && content.IsLive {
			// Store viewer in Redis with 30s expiry
			redisKey := fmt.Sprintf("stream:%s:viewer:%s", media, viewerid)
			configs.GetRedisClient().Set(ctx, redisKey, time.Now().Unix(), 30*time.Second)

			// Update viewer count
			go updateLiveViewerCount(media)

			fmt.Println("JOINEDDDDDDDDDDD")

			successResponse(w, map[string]interface{}{
				"message":      "viewing live stream",
				"viewer_count": content.ViewerCount,
			})
		} else {
			errorResponse(w, fmt.Errorf("not a live stream"), 400)
		}
	}
}

func updateLiveViewerCount(contentID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	objID, err := primitive.ObjectIDFromHex(contentID)
	if err != nil {
		return
	}

	// Count viewers in Redis
	pattern := fmt.Sprintf("stream:%s:viewer:*", contentID)
	keys, err := configs.GetRedisClient().Keys(ctx, pattern).Result()
	if err != nil {
		fmt.Println("Error counting viewers:", err)
		return
	}

	viewerCount := len(keys)

	// Update MongoDB
	_, err = getContentCollection().UpdateOne(
		ctx,
		bson.M{"_id": objID},
		bson.M{"$set": bson.M{"viewer_count": viewerCount}},
	)
	if err != nil {
		fmt.Println("Error updating viewer count:", err)
	}
}

func ViewHeartbeat() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		media := vars["MediaID"]
		viewerid := vars["ViewerID"]

		ctx := context.Background()
		redisKey := fmt.Sprintf("stream:%s:viewer:%s", media, viewerid)

		// Refresh expiry to 30 fucking seconds
		err := configs.GetRedisClient().Expire(ctx, redisKey, 30*time.Second).Err()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

func StartStreamWithBody() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Minute)
		defer cancel()
		vars := mux.Vars(r)
		userID := vars["UserID"]
		visibility := vars["Visibility"]
		tags := vars["Tags"]
		show, _ := strconv.ParseBool(vars["Show"])
		ispayperview, _ := strconv.ParseBool(vars["IsPayPerView"])
		isdeleted, _ := strconv.ParseBool(vars["IsDeleted"])

		ppvprice := vars["PPVPrice"]
		price, err := strconv.ParseFloat(ppvprice, 64)
		if err != nil {
			fmt.Println("invalid price in PPVPrice")
			price = 0
		}

		if visibility != VISIBILITY_FOLLOWERS {
			visibility = VISIBILITY_EVERYONE
		}

		contentBody := models.ContentBody{}
		err = json.NewDecoder(r.Body).Decode(&contentBody)
		if err != nil {
			errorResponse(rw, fmt.Errorf("bad request"), 200)
			return
		}

		title := contentBody.Title
		description := contentBody.Description

		// Generate unique stream key
		streamKey := strings.Replace(uuid.New().String(), "-", "", -1)

		// Streaming server IP
		streamingServerIP := "13.50.17.68" // TODO: Move to config

		newStream := models.Content{
			UserID:       userID,
			Poster:       userID,
			Title:        title,
			Description:  description,
			Location:     configs.EnvMediaDir() + "/" + userID + "/videos/",
			DateCreated:  time.Now(),
			Show:         show,
			IsPayPerView: ispayperview,
			IsDeleted:    isdeleted,
			PPVPrice:     price,
			Type:         TYPE_STREAM,
			Posting:      "",
			Visibility:   visibility,

			// Live streaming fields
			StreamKey:   streamKey,
			RTMPUrl:     fmt.Sprintf("rtmp://%s/live/%s", streamingServerIP, streamKey),
			HLSURL:      fmt.Sprintf("http://%s/hls/%s.m3u8", streamingServerIP, streamKey),
			IsLive:      false,
			ViewerCount: 0,
		}

		newStream.Tags = strings.Split(tags, ",")
		for i, s := range newStream.Tags {
			newStream.Tags[i] = strings.Trim(s, " ")
		}

		result, err := getContentCollection().InsertOne(ctx, newStream)
		if err != nil {
			rw.WriteHeader(http.StatusConflict)
			errresponse := responses.ContentResponse{Status: http.StatusConflict, Message: "error"}
			json.NewEncoder(rw).Encode(errresponse)
			return
		}

		go func() {
			// time.Sleep(10 * time.Second)
			// contentID := result.InsertedID.(primitive.ObjectID).Hex()
			// sendLiveStartedNotification(userID, contentID)
		}()

		rw.WriteHeader(http.StatusCreated)
		response := responses.ContentResponse{
			Status:  http.StatusCreated,
			Message: "success",
			Data: map[string]interface{}{
				"content_id": result.InsertedID,
				"stream_key": streamKey,
				"rtmp_url":   newStream.RTMPUrl,
				"hls_url":    newStream.HLSURL,
			},
		}
		json.NewEncoder(rw).Encode(response)
	}
}

func SetInitialVisibility() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		cur, err := getContentCollection().Find(ctx, bson.M{})
		if err != nil {
			errorResponse(rw, err, 200)
			return
		}
		content := []models.Content{}
		if err = cur.All(ctx, &content); err != nil {
			errorResponse(rw, err, 200)
			return
		}
		for k, v := range content {
			res, err := getContentCollection().UpdateOne(ctx, bson.M{"_id": v.Id}, bson.M{"$set": bson.M{"visibility": VISIBILITY_EVERYONE}})
			if err != nil {
				fmt.Println("COULDN'T UPDATE INDEX %v", k)
				continue
			}
			fmt.Println("index: %v has been modified: %v", k, res.ModifiedCount)
		}
		successResponse(rw, "OK")
	}
}

func SetTranscodingStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		filter := bson.M{"type": "video"}
		update := bson.M{"$set": bson.M{"transcoding": TRANSCODING_DONE}}
		res, err := getContentCollection().UpdateMany(ctx, filter, update)
		if err != nil {
			errorResponse(w, err, 500)
			return
		}
		successResponse(w, res)
	}
}

func DeleteContent() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		vars := mux.Vars(r)
		contentID := vars["ContentID"]

		oID, err := primitive.ObjectIDFromHex(contentID)
		if err != nil {
			errorResponse(rw, fmt.Errorf("invalid content ID"), 400)
			return
		}

		// Soft delete by setting isdeleted to true
		filter := bson.M{"_id": oID}
		update := bson.M{"$set": bson.M{"isdeleted": true}}

		result, err := getContentCollection().UpdateOne(ctx, filter, update)
		if err != nil {
			errorResponse(rw, fmt.Errorf("failed to delete content"), 500)
			return
		}

		if result.ModifiedCount == 0 {
			errorResponse(rw, fmt.Errorf("content not found"), 404)
			return
		}

		successResponse(rw, "Content deleted successfully")
	}
}

// Define a struct for cleaner code
type UploadedFile struct {
	Filename string `json:"filename"`
	S3Key    string `json:"s3_key"`
	Size     int64  `json:"size"`
}

type FailedFile struct {
	Filename string `json:"filename"`
	Error    string `json:"error"`
}

func UploadMultipleFiles() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
		defer cancel()

		// Parse multipart form
		if err := r.ParseMultipartForm(8024 * 100 * MB); err != nil {
			errorResponse(rw, fmt.Errorf("error parsing form: %v", err), 400)
			return
		}

		userID := r.FormValue("user_id")
		videoID := r.FormValue("video_id")

		if userID == "" || videoID == "" {
			errorResponse(rw, fmt.Errorf("missing user_id or video_id"), 400)
			return
		}

		fmt.Printf("User ID: %s\n", userID)
		fmt.Printf("Video ID: %s\n", videoID)
		fmt.Println("=================================")

		// Get all files
		files := r.MultipartForm.File["files"]

		if len(files) == 0 {
			errorResponse(rw, fmt.Errorf("no files uploaded"), 400)
			return
		}

		fmt.Printf("Total files received: %d\n", len(files))
		fmt.Println("=================================")

		// Get S3 uploader
		uploader := configs.GetS3Uploader()

		var uploadedFiles []UploadedFile
		var failedFiles []FailedFile
		var playlistKey string

		// Upload each file to S3
		for i, fileHeader := range files {
			fmt.Printf("Processing file %d/%d: %s\n", i+1, len(files), fileHeader.Filename)

			// Open the file
			file, err := fileHeader.Open()
			if err != nil {
				fmt.Printf("  ERROR: Failed to open file: %v\n", err)
				failedFiles = append(failedFiles, FailedFile{
					Filename: fileHeader.Filename,
					Error:    err.Error(),
				})
				continue
			}

			// Determine content type
			contentType := getContentType(fileHeader.Filename)

			// S3 key: videoId/filename
			s3Key := fmt.Sprintf("%s/%s", videoID, fileHeader.Filename)

			// Check if this is the playlist file
			if strings.HasSuffix(fileHeader.Filename, ".m3u8") {
				playlistKey = s3Key
			}

			fmt.Printf("  Uploading to S3: s3://%s/%s\n", configs.EnvProcessedBucket(), s3Key)

			// Upload to S3
			_, err = uploader.Upload(ctx, &s3.PutObjectInput{
				Bucket:      aws.String(configs.EnvProcessedBucket()),
				Key:         aws.String(s3Key),
				Body:        file,
				ContentType: aws.String(contentType),
			})

			file.Close()

			if err != nil {
				fmt.Printf("  ERROR: Failed to upload to S3: %v\n", err)
				failedFiles = append(failedFiles, FailedFile{
					Filename: fileHeader.Filename,
					Error:    err.Error(),
				})
				continue
			}

			fmt.Printf("  SUCCESS: Uploaded %s (%d bytes)\n", fileHeader.Filename, fileHeader.Size)

			uploadedFiles = append(uploadedFiles, UploadedFile{
				Filename: fileHeader.Filename,
				S3Key:    s3Key,
				Size:     fileHeader.Size,
			})

			fmt.Println("---------------------------------")
		}

		fmt.Printf("\n=================================\n")
		fmt.Printf("Upload Summary:\n")
		fmt.Printf("  Successful: %d\n", len(uploadedFiles))
		fmt.Printf("  Failed: %d\n", len(failedFiles))
		fmt.Printf("=================================\n")

		// Update Content collection with HLS URL
		if playlistKey != "" {
			// Use your CDN URL instead of S3 direct URL
			hlsURL := fmt.Sprintf("https://syn-video-cdn.b-cdn.net/%s", playlistKey)

			// Update by video_id only (since it's unique)
			filter := bson.M{"video_id": videoID}
			update := bson.M{
				"$set": bson.M{
					"hls_url":     hlsURL,
					"posting":     hlsURL,
					"transcoding": "done",
				},
			}

			result, err := getContentCollection().UpdateOne(ctx, filter, update)
			if err != nil {
				fmt.Printf("ERROR: Failed to update content collection: %v\n", err)
			} else if result.MatchedCount == 0 {
				fmt.Printf("WARNING: No content found with video_id=%s\n", videoID)
			} else {
				fmt.Printf("SUCCESS: Updated content collection with HLS URL: %s\n", hlsURL)
			}
		} else {
			fmt.Println("WARNING: No .m3u8 file found, skipping database update")
		}

		// Return response
		rw.WriteHeader(http.StatusOK)
		response := map[string]interface{}{
			"status":         "success",
			"message":        fmt.Sprintf("Uploaded %d/%d files successfully", len(uploadedFiles), len(files)),
			"video_id":       videoID,
			"user_id":        userID,
			"hls_url":        fmt.Sprintf("https://syn-video-cdn.b-cdn.net/%s", playlistKey),
			"uploaded_files": uploadedFiles,
			"failed_files":   failedFiles,
		}
		json.NewEncoder(rw).Encode(response)
	}
}

func getContentType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))

	switch ext {
	case ".m3u8":
		return "application/x-mpegURL"
	case ".ts":
		return "video/MP2T"
	case ".mp4":
		return "video/mp4"
	default:
		return "application/octet-stream"
	}
}
