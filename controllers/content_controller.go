package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
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

	"github.com/disintegration/imaging"
	"github.com/gabriel-vasile/mimetype"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	contentCollection     *mongo.Collection = configs.GetCollection(configs.DB, "content")
	profilepicsCollection *mongo.Collection = configs.GetCollection(configs.DB, "profile_pics")
	videosCollection      *mongo.Collection = configs.GetCollection(configs.DB, "videos")
	usersCollection       *mongo.Collection = configs.GetCollection(configs.DB, "users")
	followsCollection     *mongo.Collection = configs.GetCollection(configs.DB, "follows")
)

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
		err = usersCollection.FindOne(ctx, bson.M{"_id": oID}).Decode(&userObj)
		if err != nil {
			errorResponse(rw, fmt.Errorf("couldn't find/decode user"), 200)
			return
		}

		//location := configs.EnvMediaDir() + userID + "/pics/profile/" + fileName

		filterByFilename := bson.M{"filename": fileName, "userid": userID}
		filterByUserID := bson.M{"_id": oID}
		if whatWillChange == DELETE {
			delete := bson.M{"$set": bson.M{"isdeleted": true, "iscurrent": false}}
			err := profilepicsCollection.FindOneAndUpdate(ctx, filterByFilename, delete).Err()
			if err != nil {
				fmt.Println("1")
				errorResponse(rw, err, 500)
				return
			}
			sortByDateCreated := options.Find()
			sortByDateCreated.SetSort(bson.D{{Key: "datecreated", Value: -1}})
			filterByNotDeleted := bson.M{"userid": userID, "isdeleted": false}

			var pics []models.ProfilePic
			cur, err := profilepicsCollection.Find(ctx, filterByNotDeleted, sortByDateCreated)
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

				_, err = profilepicsCollection.UpdateOne(ctx, filterByID, makeCurrentTrue)
				if err != nil {
					fmt.Println("4")
					errorResponse(rw, err, 500)
					return
				}
				setUserPic := bson.M{"$set": bson.M{"profile_pic": pics[0].Location}}
				_, err = usersCollection.UpdateOne(ctx, filterByUserID, setUserPic)
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

			_, err := profilepicsCollection.UpdateOne(ctx, filterByCurrent, makeCurrentFalse)
			if err != nil {
				fmt.Println("11111")
				errorResponse(rw, err, 500)
				return
			}
			_, err = profilepicsCollection.UpdateOne(ctx, filterByFilename, makeCurrentTrue)
			if err != nil {
				fmt.Println("22222", fileName)
				errorResponse(rw, err, 500)
				return
			}
			pic := models.NewProfilePic{}
			err = profilepicsCollection.FindOne(ctx, filterByFilename).Decode(&pic)
			if err != nil {
				fmt.Println("could not load new pic")
				errorResponse(rw, err, 500)
				return
			}
			setUserPic := bson.M{"$set": bson.M{"profile_pic": pic.Location}}
			_, err = usersCollection.UpdateOne(ctx, filterByUserID, setUserPic)
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
		mongoSingleResult := contentCollection.FindOneAndUpdate(ctx, filterByFilename, update)

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
		mongoSingleResult := contentCollection.FindOneAndUpdate(ctx, filterByFilename, update)

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
		mongoSingleResult := contentCollection.FindOneAndUpdate(ctx, filterByFilename, update)

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

func PostProfilePic() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		vars := mux.Vars(r)
		userID := vars["UserID"]
		iscurrent, _ := strconv.ParseBool(vars["IsCurrent"])
		isdeleted, _ := strconv.ParseBool(vars["IsDeleted"])
		extension := ""
		newUuid := uuid.New()

		newPostPic := models.NewProfilePic{
			UserID:      userID,
			Location:    configs.EnvMediaDir() + userID + "/pics/profile/",
			DateCreated: time.Now(),
			IsCurrent:   iscurrent,
			IsDeleted:   isdeleted,
		}

		// Ensure directory exists
		if err := os.MkdirAll(newPostPic.Location, 0777); err != nil {
			fmt.Println("MkdirAll error:", err)
		}
		if err := os.Chmod(newPostPic.Location, 0666); err != nil {
			fmt.Println("Chmod error:", err)
		}

		// Parse form
		r.ParseMultipartForm(10 * MB)
		r.Body = http.MaxBytesReader(rw, r.Body, 10*MB)

		// Retrieve file from form
		file, _, err := r.FormFile("file")
		if err != nil {
			errorResponse(rw, err, http.StatusInternalServerError)
			return
		}
		defer file.Close()

		// --- CHANGED: Read the entire file for detection ---
		fileBytes, err := io.ReadAll(file)
		if err != nil {
			fmt.Println("failed to read file data:", err)
			errorResponse(rw, err, http.StatusInternalServerError)
			return
		}

		// Detect MIME using mimetype
		detectedMIME := mimetype.Detect(fileBytes)
		mimeType := detectedMIME.String()
		fmt.Println("Detected MIME:", mimeType)

		// Switch on the detected MIME
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

		if _, err := file.Seek(0, 0); err != nil {
			fmt.Println("Seek error:", err)
			errorResponse(rw, err, http.StatusInternalServerError)
			return
		}

		outPath := newPostPic.Location + strings.Replace(newUuid.String(), "-", "", -1) + "." + extension
		f, err := os.OpenFile(outPath, os.O_WRONLY|os.O_CREATE, 0666)
		if err != nil {
			errorResponse(rw, err, http.StatusInternalServerError)
			return
		}
		defer f.Close()

		if _, err := io.Copy(f, file); err != nil {
			errorResponse(rw, err, http.StatusInternalServerError)
			return
		}

		newPostPic.Location = outPath

		result, err := profilepicsCollection.InsertOne(ctx, newPostPic)
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
	folderPath := configs.EnvMediaDir() + userID + "/profile_pics/"
	if _, err := os.Stat(folderPath); os.IsNotExist(err) {
		err := os.MkdirAll(folderPath, 0755)
		if err != nil {
			return "", err
		}
	}
	filename += "." + strings.Split(fileHeader.Filename, ".")[1]
	fullpath := filepath.Join(folderPath, filename)
	dst, err := os.Create(fullpath)
	if err != nil {
		return "", err
	}
	defer dst.Close()
	if _, err := io.Copy(dst, file); err != nil {
		return "", err
	}
	return fullpath, nil
}

func PostProfilePicBase64() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		vars := mux.Vars(r)
		userID := vars["UserID"]
		oid, err := primitive.ObjectIDFromHex(userID)
		if err != nil {
			errorResponse(rw, fmt.Errorf("invalid userID"), 200)
			return
		}
		iscurrent, _ := strconv.ParseBool(vars["IsCurrent"])
		r.ParseMultipartForm(10 * MB)
		r.Body = http.MaxBytesReader(rw, r.Body, 10*MB)
		file, fileHeader, err := r.FormFile("file")
		if err != nil {
			errorResponse(rw, err, 500)
			return
		}
		defer file.Close()
		profileID := primitive.NewObjectID()
		location, err := uploadProfilePic(userID, file, fileHeader, profileID.Hex())
		if err != nil {
			errorResponse(rw, err, 200)
			return
		}
		newPostPic := models.NewProfilePic{
			ID:          profileID,
			UserID:      userID,
			Location:    location,
			Filename:    profileID.Hex(),
			DateCreated: time.Now(),
			IsCurrent:   iscurrent,
			IsDeleted:   false,
		}
		result, err := profilepicsCollection.InsertOne(ctx, newPostPic)
		if err != nil {
			errorResponse(rw, err, 500)
			return
		}
		if iscurrent {
			filterByCurrent := bson.M{"userid": userID, "iscurrent": true}
			profilepicsCollection.UpdateOne(ctx, filterByCurrent, bson.M{"$set": bson.M{"iscurrent": false}})
			usersCollection.UpdateOne(ctx, bson.M{"_id": oid}, bson.M{"$set": bson.M{"profile_pic": location}})
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

		newUuid := uuid.New()
		newPostPic := models.Content{
			UserID:       userID,
			Poster:       userID,
			Title:        title,
			Description:  description,
			Location:     configs.EnvMediaDir() + userID + "/pics/",
			DateCreated:  time.Now(),
			Show:         show,
			IsPayPerView: ispayperview,
			IsDeleted:    isdeleted,
			PPVPrice:     price,
			Type:         TYPE_PIC,
			Posting:      "",
			Visibility:   visibility,
		}
		newPostPic.Tags = strings.Split(tags, ",")
		for i, s := range newPostPic.Tags {
			newPostPic.Tags[i] = strings.TrimSpace(s)
		}

		if err := os.MkdirAll(newPostPic.Location, 0777); err != nil {
			fmt.Println(err)
		}
		if err := os.Chmod(newPostPic.Location, 0666); err != nil {
			fmt.Println(err)
		}

		r.ParseMultipartForm(10 * MB)
		r.Body = http.MaxBytesReader(rw, r.Body, 10*MB)

		file, _, err := r.FormFile("file")
		if err != nil {
			fmt.Println("Error reading form file:", err)
			http.Error(rw, "Missing file", http.StatusBadRequest)
			return
		}
		defer file.Close()

		// Read the entire upload into memory for detection
		fileBytes, err := io.ReadAll(file)
		if err != nil {
			fmt.Println("Failed to read file data:", err)
			http.Error(rw, "Failed to read file data", http.StatusInternalServerError)
			return
		}

		// Detect MIME type
		detectedMIME := mimetype.Detect(fileBytes)
		mimeType := detectedMIME.String()
		fmt.Println("Detected MIME:", mimeType)

		// Set extension based on MIME
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

		// Reset file pointer so we can copy to disk
		if _, err := file.Seek(0, 0); err != nil {
			fmt.Println("Seek error:", err)
			http.Error(rw, "Internal error", http.StatusInternalServerError)
			return
		}

		// Create the original file
		originalFileName := strings.ReplaceAll(newUuid.String(), "-", "") + "." + extension
		originalPath := newPostPic.Location + originalFileName

		f, err := os.OpenFile(originalPath, os.O_WRONLY|os.O_CREATE, 0666)
		if err != nil {
			fmt.Println("Error creating original file:", err)
			http.Error(rw, "Failed to create file", http.StatusInternalServerError)
			return
		}
		defer f.Close()

		// Save the original file
		if _, err := io.Copy(f, file); err != nil {
			fmt.Println("Error copying to original file:", err)
			http.Error(rw, "Failed to save file", http.StatusInternalServerError)
			return
		}

		// Now create the thumbnail
		thumbFileName := strings.ReplaceAll(newUuid.String(), "-", "") + "thumb." + extension
		thumbPath := newPostPic.Location + thumbFileName

		if mimeType == "image/heic" || mimeType == "image/heif" {
			// === HEIC/HEIF branch: skip resizing, just copy original to thumb
			originalIn, err := os.Open(originalPath)
			if err != nil {
				fmt.Println("Error opening original for thumb copy:", err)
				return
			}
			defer originalIn.Close()

			thumbOut, err := os.Create(thumbPath)
			if err != nil {
				fmt.Println("Error creating thumb file:", err)
				return
			}
			defer thumbOut.Close()

			if _, err := io.Copy(thumbOut, originalIn); err != nil {
				fmt.Println("Error copying thumb file:", err)
				return
			}
		} else {
			// === Non-HEIC branch: decode & resize using imaging
			src, err := imaging.Open(originalPath)
			if err != nil {
				fmt.Println("Error opening file for imaging:", err)
				// You might choose to skip or return here
			} else {
				thumb := imaging.Resize(src, 585, 0, imaging.Linear)
				if err := imaging.Save(thumb, thumbPath); err != nil {
					fmt.Println("Error saving thumbnail:", err)
				}
			}
		}

		// Update location for the original
		newPostPic.Location = originalPath

		// Insert to DB
		result, err := contentCollection.InsertOne(ctx, newPostPic)
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

		// Read JSON from formValue("data")
		jsonData := r.FormValue("data")
		if jsonData == "" {
			errorResponse(rw, fmt.Errorf("missing data"), 200)
			return
		}

		var contentBody models.ContentBody
		if err := json.Unmarshal([]byte(jsonData), &contentBody); err != nil {
			errorResponse(rw, err, 200)
			return
		}

		title := contentBody.Title
		description := contentBody.Description

		if visibility != VISIBILITY_FOLLOWERS {
			visibility = VISIBILITY_EVERYONE
		}

		newUuid := uuid.New()

		newPostPic := models.Content{
			UserID:       userID,
			Poster:       userID,
			Title:        title,
			Description:  description,
			Location:     configs.EnvMediaDir() + userID + "/pics/",
			DateCreated:  time.Now(),
			Show:         show,
			IsPayPerView: ispayperview,
			IsDeleted:    isdeleted,
			PPVPrice:     price,
			Type:         TYPE_PIC,
			Posting:      "",
			Visibility:   visibility,
		}
		newPostPic.Tags = strings.Split(tags, ",")
		for i, s := range newPostPic.Tags {
			newPostPic.Tags[i] = strings.TrimSpace(s)
		}

		// Ensure directory exists
		if err := os.MkdirAll(newPostPic.Location, 0777); err != nil {
			fmt.Println(err)
		}
		if err := os.Chmod(newPostPic.Location, 0666); err != nil {
			fmt.Println(err)
		}

		// Parse form and limit body
		r.ParseMultipartForm(10 * MB)
		r.Body = http.MaxBytesReader(rw, r.Body, 10*MB)

		// Retrieve file
		file, _, err := r.FormFile("file")
		if err != nil {
			fmt.Println(err)
		}
		defer file.Close()

		// Read entire file for MIME detection
		fileBytes, err := io.ReadAll(file)
		if err != nil {
			fmt.Println("failed to read file data:", err)
			http.Error(rw, "Failed to read file data", http.StatusInternalServerError)
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

		if _, err := file.Seek(0, 0); err != nil {
			fmt.Println(err)
			return
		}

		// Construct original path
		originalName := strings.Replace(newUuid.String(), "-", "", -1) + "." + extension
		originalPath := newPostPic.Location + originalName

		// Save original
		f, err := os.OpenFile(originalPath, os.O_WRONLY|os.O_CREATE, 0666)
		if err != nil {
			fmt.Println(err)
		}
		defer f.Close()
		io.Copy(f, file)

		// Construct thumbnail path
		thumbName := strings.Replace(newUuid.String(), "-", "", -1) + "thumb." + extension
		thumbPath := newPostPic.Location + thumbName

		// If HEIC/HEIF => skip resizing, just copy the original
		if mimeType == "image/heic" || mimeType == "image/heif" {

			// Reopen original to copy
			origIn, err := os.Open(originalPath)
			if err != nil {
				fmt.Println("Error opening original for thumb copy:", err)
				return
			}
			defer origIn.Close()

			thumbOut, err := os.Create(thumbPath)
			if err != nil {
				fmt.Println("Error creating thumb file:", err)
				return
			}
			defer thumbOut.Close()

			if _, err := io.Copy(thumbOut, origIn); err != nil {
				fmt.Println("Error copying thumb:", err)
				return
			}

		} else {
			// Otherwise (PNG, JPEG, WEBP) => decode & resize
			srcPRT, err := imaging.Open(originalPath)
			if err != nil {
				fmt.Println("Error opening file for imaging:", err)
			} else {
				srcPRT = imaging.Resize(srcPRT, 585, 0, imaging.Linear)
				if err := imaging.Save(srcPRT, thumbPath); err != nil {
					fmt.Println("Error saving thumbnail:", err)
				}
			}
		}

		// Update location to original
		newPostPic.Location = originalPath

		// Insert into DB
		result, err := contentCollection.InsertOne(ctx, newPostPic)
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

func PostVideo() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Minute)
		defer cancel()
		vars := mux.Vars(r)
		userID := vars["UserID"]
		title := vars["Title"]
		description := vars["Description"]
		tags := vars["Tags"]
		show, _ := strconv.ParseBool(vars["Show"])
		ispayperview, _ := strconv.ParseBool(vars["IsPayPerView"])
		isdeleted, _ := strconv.ParseBool(vars["IsDeleted"])
		extension := ""
		thumbextension := ""
		ffmpegSource := ""
		ffmpegTarget := ""
		newUuid := uuid.New()
		ppvprice := vars["PPVPrice"]
		price, err := strconv.ParseFloat(ppvprice, 64)
		if err != nil {
			fmt.Println("invalid price in PPVPrice")
			price = 0
		}

		newPostVid := models.NewPostVideo{
			UserID:       userID,
			Title:        title,
			Description:  description,
			Location:     configs.EnvMediaDir() + userID + "/videos/",
			DateCreated:  time.Now(),
			Show:         show,
			IsPayPerView: ispayperview,
			IsDeleted:    isdeleted,
			PPVPrice:     price,
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
		err = os.Chmod(configs.EnvMediaDir()+userID, 0666)
		if err != nil {
			fmt.Println(err)
		}

		err = os.Chmod(configs.EnvMediaDir()+userID+"/videos", 0666)
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
		file, _, err := r.FormFile("video")
		if err != nil {
			fmt.Println(err)
		}
		thumb, _, err := r.FormFile("thumb")
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

		thumbHeader := make([]byte, 512)
		if _, err := thumb.Read(thumbHeader); err != nil {
			return
		}
		if _, err := thumb.Seek(0, 0); err != nil {
			return
		}

		mime := http.DetectContentType(fileHeader)
		mimeThumb := http.DetectContentType(thumbHeader)

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
		default:
			http.Error(rw, "This file type for video is not allowed.", http.StatusBadRequest)
			return
		}
		//deleting uploaded video after transcoding is finished
		//defer CleanUp(newPostVid.Location + strings.Replace(newUuid.String(), "-", "", -1) + "." + extension)

		//transcoding after all files are closed
		ffmpegSource = newPostVid.Location + strings.Replace(newUuid.String(), "-", "", -1) + "." + extension
		ffmpegTarget = newPostVid.Location + strings.Replace(newUuid.String(), "-", "", -1)
		fmt.Println(ffmpegSource, ffmpegTarget)
		//start transcoding process at the end of this method
		/* command := []string{
			"./script/create-vod-hls.sh",
			ffmpegSource,
			ffmpegTarget,
		}
		defer Execute("./script/create-vod-hls.sh", command) */
		//end transcoding instructions
		switch mimeThumb {
		case "image/png":
			thumbextension = "png"
		case "image/jpeg":
			thumbextension = "jpeg"
		case "image/webp":
			thumbextension = "webp"

		default:
			http.Error(rw, "This file type is not allowed for thumbnails", http.StatusBadRequest)
			return
		}

		defer file.Close()
		defer thumb.Close()

		f, err := os.OpenFile(newPostVid.Location+strings.Replace(newUuid.String(), "-", "", -1)+"."+extension, os.O_WRONLY|os.O_CREATE, 0666)
		if err != nil {
			fmt.Println("Can't create a file for video at....")
			fmt.Println("----" + newPostVid.Location + strings.Replace(newUuid.String(), "-", "", -1) + "." + extension + "----")
			fmt.Println(err)
		}
		io.Copy(f, file)
		defer f.Close()

		srcPRT, _, err := image.Decode(thumb)
		if err != nil {
			fmt.Println(err)
		}
		srcPRT = imaging.Resize(srcPRT, 585, 0, imaging.Linear)
		err = imaging.Save(srcPRT, newPostVid.Location+strings.Replace(newUuid.String(), "-", "", -1)+"thumb."+thumbextension)
		if err != nil {
			fmt.Println(err)
		}

		newPostVid.Location = newPostVid.Location + strings.Replace(newUuid.String(), "-", "", -1) + "/"
		result, err := videosCollection.InsertOne(ctx, newPostVid)
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
		result, err := contentCollection.InsertOne(ctx, newTextContent)
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
		result, err := contentCollection.InsertOne(ctx, newTextContent)
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
			Location:     configs.EnvMediaDir() + userID + "/videos/",
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
		err = os.Chmod(configs.EnvMediaDir()+userID, 0666)
		if err != nil {
			fmt.Println(err)
		}

		err = os.Chmod(configs.EnvMediaDir()+userID+"/videos", 0666)
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
		result, err := contentCollection.InsertOne(ctx, newPostVid)
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

		jsonData := r.FormValue("data")
		if jsonData == "" {
			errorResponse(rw, fmt.Errorf("missing data"), 200)
			return
		}

		var contentBody models.ContentBody
		if err := json.Unmarshal([]byte(jsonData), &contentBody); err != nil {
			errorResponse(rw, err, 200)
			return
		}

		title := contentBody.Title
		description := contentBody.Description

		newPostVid := models.Content{
			UserID:       userID,
			Poster:       userID,
			Title:        title,
			Description:  description,
			Location:     configs.EnvMediaDir() + userID + "/videos/",
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
		err = os.Chmod(configs.EnvMediaDir()+userID, 0666)
		if err != nil {
			fmt.Println(err)
		}

		err = os.Chmod(configs.EnvMediaDir()+userID+"/videos", 0666)
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
		result, err := contentCollection.InsertOne(ctx, newPostVid)
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
		newStream := models.Content{
			UserID:       userID,
			Poster:       userID,
			Title:        title,
			Description:  description,
			Location:     configs.EnvMediaDir() + userID + "/videos/",
			DateCreated:  time.Now(),
			Show:         show,
			IsPayPerView: ispayperview,
			IsDeleted:    isdeleted,
			PPVPrice:     price,
			Type:         TYPE_STREAM,
			Posting:      "",
			Visibility:   visibility,
		}
		newStream.Tags = strings.Split(tags, ",")
		for i, s := range newStream.Tags {
			newStream.Tags[i] = strings.Trim(s, " ")
		}
		newStream.Location = newStream.Location + strings.Replace(newUuid.String(), "-", "", -1) + "/"
		result, err := contentCollection.InsertOne(ctx, newStream)
		if err != nil {
			rw.WriteHeader(http.StatusConflict)
			errresponse := responses.ContentResponse{Status: http.StatusConflict, Message: "error"}
			json.NewEncoder(rw).Encode(errresponse)
			return
		}

		go func() {
			time.Sleep(10 * time.Second)
			contentID := result.InsertedID.(primitive.ObjectID).Hex()
			sendLiveStartedNotification(userID, contentID)
		}()

		rw.WriteHeader(http.StatusCreated)
		response := responses.ContentResponse{Status: http.StatusCreated, Message: "success", Data: map[string]interface{}{"data": result}}
		json.NewEncoder(rw).Encode(response)
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
		contentBody := models.ContentBody{}

		err = json.NewDecoder(r.Body).Decode(&contentBody)
		if err != nil {
			errorResponse(rw, fmt.Errorf("bad request"), 200)
			return
		}

		title := contentBody.Title
		description := contentBody.Description

		newStream := models.Content{
			UserID:       userID,
			Poster:       userID,
			Title:        title,
			Description:  description,
			Location:     configs.EnvMediaDir() + userID + "/videos/",
			DateCreated:  time.Now(),
			Show:         show,
			IsPayPerView: ispayperview,
			IsDeleted:    isdeleted,
			PPVPrice:     price,
			Type:         TYPE_STREAM,
			Posting:      "",
			Visibility:   visibility,
		}
		newStream.Tags = strings.Split(tags, ",")
		for i, s := range newStream.Tags {
			newStream.Tags[i] = strings.Trim(s, " ")
		}
		newStream.Location = newStream.Location + strings.Replace(newUuid.String(), "-", "", -1) + "/"
		result, err := contentCollection.InsertOne(ctx, newStream)
		if err != nil {
			rw.WriteHeader(http.StatusConflict)
			errresponse := responses.ContentResponse{Status: http.StatusConflict, Message: "error"}
			json.NewEncoder(rw).Encode(errresponse)
			return
		}

		go func() {
			time.Sleep(10 * time.Second)
			contentID := result.InsertedID.(primitive.ObjectID).Hex()
			sendLiveStartedNotification(userID, contentID)
		}()

		rw.WriteHeader(http.StatusCreated)
		response := responses.ContentResponse{Status: http.StatusCreated, Message: "success", Data: map[string]interface{}{"data": result}}
		json.NewEncoder(rw).Encode(response)
	}
}

func SetInitialVisibility() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		cur, err := contentCollection.Find(ctx, bson.M{})
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
			res, err := contentCollection.UpdateOne(ctx, bson.M{"_id": v.Id}, bson.M{"$set": bson.M{"visibility": VISIBILITY_EVERYONE}})
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
		res, err := contentCollection.UpdateMany(ctx, filter, update)
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

		result, err := contentCollection.UpdateOne(ctx, filter, update)
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
