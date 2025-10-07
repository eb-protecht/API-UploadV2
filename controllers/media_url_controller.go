package controllers

import (
	"encoding/json"
	"net/http"
	"upload-service/utils"

	"github.com/gorilla/mux"
)

// ConvertPathToURL converts a file path to a web URL
// @Summary Convert file path to URL
// @Description Converts a local file path to a web-accessible URL
// @Tags media
// @Accept json
// @Produce json
// @Param path query string true "File path to convert"
// @Success 200 {object} map[string]interface{}
// @Router /media/path-to-url [get]
func ConvertPathToURL() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		filePath := r.URL.Query().Get("path")

		if filePath == "" {
			http.Error(rw, "path parameter is required", http.StatusBadRequest)
			return
		}

		url := utils.FilePathToURL(filePath)

		response := map[string]interface{}{
			"status":  http.StatusOK,
			"message": "success",
			"data": map[string]string{
				"original_path": filePath,
				"url":           url,
			},
		}

		rw.Header().Set("Content-Type", "application/json")
		json.NewEncoder(rw).Encode(response)
	}
}

// ConvertURLToPath converts a web URL to a file path
// @Summary Convert URL to file path
// @Description Converts a web-accessible URL to a local file path
// @Tags media
// @Accept json
// @Produce json
// @Param url query string true "URL to convert"
// @Success 200 {object} map[string]interface{}
// @Router /media/url-to-path [get]
func ConvertURLToPath() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		url := r.URL.Query().Get("url")

		if url == "" {
			http.Error(rw, "url parameter is required", http.StatusBadRequest)
			return
		}

		filePath := utils.URLToFilePath(url)

		response := map[string]interface{}{
			"status":  http.StatusOK,
			"message": "success",
			"data": map[string]string{
				"original_url": url,
				"path":         filePath,
			},
		}

		rw.Header().Set("Content-Type", "application/json")
		json.NewEncoder(rw).Encode(response)
	}
}

// GetMediaInfo returns media information with proper URLs
// @Summary Get media information
// @Description Returns media file information with web-accessible URLs
// @Tags media
// @Accept json
// @Produce json
// @Param userID path string true "User ID"
// @Param fileType path string true "File type (pics, videos, profile_pics, etc.)"
// @Param filename path string true "Filename"
// @Success 200 {object} map[string]interface{}
// @Router /media/{userID}/{fileType}/{filename}/info [get]
func GetMediaInfo() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		userID := vars["userID"]
		fileType := vars["fileType"]
		filename := vars["filename"]

		mediaURL := utils.GetMediaURL(userID, fileType, filename)
		filePath := "/app/media/" + userID + "/" + fileType + "/" + filename

		response := map[string]interface{}{
			"status":  http.StatusOK,
			"message": "success",
			"data": map[string]interface{}{
				"user_id":   userID,
				"file_type": fileType,
				"filename":  filename,
				"url":       mediaURL,
				"path":      filePath,
			},
		}

		rw.Header().Set("Content-Type", "application/json")
		json.NewEncoder(rw).Encode(response)
	}
}
