package controllers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"upload-service/configs"
	"upload-service/models"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/crypto/bcrypt"
)

func PostVideoToPostgres() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		userID := vars["UserID"]
		title := vars["Title"]
		visibility := vars["Visibility"]
		description := vars["Description"]
		tags := vars["Tags"]
		show, _ := strconv.ParseBool(vars["Show"])
		isPayPerView, _ := strconv.ParseBool(vars["IsPayPerView"])
		isDeleted, _ := strconv.ParseBool(vars["IsDeleted"])
		ppvprice := vars["PPVPrice"]

		// Convert price to float64
		price, err := strconv.ParseFloat(ppvprice, 64)
		if err != nil {
			fmt.Println("Invalid price in PPVPrice")
			price = 0
		}

		// Ensure visibility has a valid value
		if visibility != "followers" {
			visibility = "everyone"
		}

		// Generate MongoDB-like ObjectID for PostgreSQL
		newObjectID := primitive.NewObjectID()

		// Create the new content struct
		newPostVid := models.Content{
			Id:           newObjectID, // Use ObjectID in the Go struct
			UserID:       userID,
			Poster:       userID,
			Title:        title,
			Description:  description,
			Location:     configs.EnvMediaDir() + userID + "/videos/",
			DateCreated:  time.Now(),
			Show:         show,
			IsPayPerView: isPayPerView,
			IsDeleted:    isDeleted,
			PPVPrice:     price,
			Tags:         strings.Split(tags, ","),
			Visibility:   visibility,
		}

		// Trim whitespace from tags
		for i, s := range newPostVid.Tags {
			newPostVid.Tags[i] = strings.TrimSpace(s)
		}

		// Create directory for storing the video file
		fmt.Println("Creating folder at location:", newPostVid.Location)
		err = os.MkdirAll(newPostVid.Location, 0777)
		if err != nil {
			fmt.Println("Error creating directory:", err)
			http.Error(rw, "Error creating directory", http.StatusInternalServerError)
			return
		}

		r.ParseMultipartForm(1024 * 20 * 1024 * 1024)               // Parse up to 20 MB
		r.Body = http.MaxBytesReader(rw, r.Body, 1024*20*1024*1024) // Limit body size

		file, fileHeader, err := r.FormFile("video")
		if err != nil {
			fmt.Println("Error retrieving the file:", err)
			http.Error(rw, "Error retrieving file", http.StatusBadRequest)
			return
		}
		defer file.Close()

		// Check the file MIME type
		fileHeaderBuffer := make([]byte, 512)
		if _, err := file.Read(fileHeaderBuffer); err != nil {
			fmt.Println("Error reading file header:", err)
			http.Error(rw, "Error reading file", http.StatusInternalServerError)
			return
		}
		if _, err := file.Seek(0, 0); err != nil {
			fmt.Println("Error resetting file pointer:", err)
			http.Error(rw, "Error resetting file", http.StatusInternalServerError)
			return
		}

		mimeType := http.DetectContentType(fileHeaderBuffer)
		var extension string
		switch mimeType {
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
		case "application/octet-stream":
			// Handle octet-stream: infer extension from the uploaded filename
			extension = filepath.Ext(fileHeader.Filename)
			if extension == "" {
				http.Error(rw, "Unsupported video format and no extension found", http.StatusBadRequest)
				return
			}
			extension = extension[1:] // Remove the dot from the extension
		default:
			http.Error(rw, "Unsupported video format", http.StatusBadRequest)
			return
		}

		// Define the target location for storing the video
		videoFileName := strings.Replace(newObjectID.Hex(), "-", "", -1) + "." + extension
		videoFilePath := newPostVid.Location + videoFileName

		// Create the video file
		outFile, err := os.OpenFile(videoFilePath, os.O_WRONLY|os.O_CREATE, 0666)
		if err != nil {
			fmt.Println("Error creating video file:", err)
			http.Error(rw, "Error saving video file", http.StatusInternalServerError)
			return
		}
		defer outFile.Close()

		// Copy the uploaded video file to the target location
		_, err = io.Copy(outFile, file)
		if err != nil {
			fmt.Println("Error saving video file:", err)
			http.Error(rw, "Error saving video file", http.StatusInternalServerError)
			return
		}

		// Update the content's location field with the full path
		newPostVid.Location = videoFilePath

		// Insert the content record into PostgreSQL
		result := configs.PGDB.Table("promoted_content").Create(&newPostVid)
		if result.Error != nil {
			fmt.Println("Error inserting video content into PostgreSQL:", result.Error)
			http.Error(rw, "Error saving content", http.StatusInternalServerError)
			return
		}

		// Return a success response with the generated ObjectID
		rw.WriteHeader(http.StatusCreated)
		response := map[string]interface{}{
			"status":  http.StatusCreated,
			"message": "success",
			"data":    newPostVid.Id.Hex(),
		}
		json.NewEncoder(rw).Encode(response)
	}
}
func RegisterUser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		newUser := models.User{}

		// Decode the request body into the newUser struct
		err := json.NewDecoder(r.Body).Decode(&newUser)
		if err != nil {
			errorResponse(w, fmt.Errorf("bad request: invalid input"), 400)
			return
		}
		// Check if a user with the same email already exists
		var count int64
		err = configs.PGDB.Table("promotion_users").Where("email = ?", newUser.Email).Count(&count).Error
		if err != nil {
			fmt.Println(err)
			errorResponse(w, fmt.Errorf("error checking if user exists"), 500)
			return
		}

		if count >= 1 {
			errorResponse(w, fmt.Errorf("user with this email already exists"), 400)
			return
		}

		if len(newUser.ProfilePic) < 1 {
			if newUser.Gender == "M" {
				newUser.ProfilePic = configs.MALEAVATAR()
			} else if newUser.Gender == "F" {
				newUser.ProfilePic = configs.FEMALEAVATAR()
			}
		}

		// Hash the user's password before storing
		newUser.Password, err = hashPassword(newUser.Password)
		if err != nil {
			errorResponse(w, fmt.Errorf("couldn't hash password"), 500)
			return
		}

		// Set account status and timestamps
		newUser.Active = false
		newUser.Verified = false
		newUser.CreatedAt = time.Now()
		newUser.UpdatedAt = time.Now()

		newUser.UserID = primitive.NewObjectID().Hex()
		newUser.Categories = nil
		result := configs.PGDB.Table("promotion_users").Create(&newUser)
		if result.Error != nil {
			errorResponse(w, fmt.Errorf("couldn't store new user: %v", result.Error), 500)
			return
		}

		successResponse(w, "ok")
	}
}

func Promote() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)

		// Extract parameters from the route
		contentID := vars["ContentID"]
		startStr := vars["Start"]
		endStr := vars["End"]
		targetAgeStr := vars["TargetAge"]
		targetGender := vars["TargetGender"]
		amountStr := vars["Amount"]
		startDate, err := time.Parse("2006-01-02", startStr)
		if err != nil {
			errorResponse(w, err, 200)
			return
		}

		endDate, err := time.Parse("2006-01-02", endStr)
		if err != nil {
			errorResponse(w, err, 200)
			return
		}
		targetAges := strings.Split(targetAgeStr, ",")
		if len(targetAges) != 2 {
			errorResponse(w, fmt.Errorf("error parsing target ages"), 200)
			return
		}

		startTargetAge, err := strconv.ParseInt(strings.TrimSpace(targetAges[0]), 10, 64)
		if err != nil {
			errorResponse(w, err, 200)
			return
		}

		endTargetAge, err := strconv.ParseInt(strings.TrimSpace(targetAges[1]), 10, 64)
		if err != nil {
			errorResponse(w, err, 200)
			return
		}

		amount, err := strconv.ParseFloat(amountStr, 64)
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid amount: %v", err), http.StatusBadRequest)
			return
		}

		newPromotion := models.Promotions{
			ContentID:      contentID,
			Amount:         amount,
			TargetGender:   targetGender,
			StartDate:      startDate,
			EndDate:        endDate,
			StartTargetAge: startTargetAge,
			EndTargetAge:   endTargetAge,
		}

		result := configs.PGDB.Create(&newPromotion)
		if result.Error != nil {
			errorResponse(w, result.Error, 200)
			return
		}
		successResponse(w, "ok")

	}
}

func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}
