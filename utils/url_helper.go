package utils

import (
	"os"
	"strings"
)

// GetBaseURL returns the base URL for the application
// In production, this should come from an environment variable
func GetBaseURL() string {
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		// Default for local development
		baseURL = "http://localhost"
	}
	return baseURL
}

// FilePathToURL converts a local file path to a web-accessible URL
// Example: /app/media/user123/pics/photo.jpg -> http://localhost/media/user123/pics/photo.jpg
func FilePathToURL(filePath string) string {
	baseURL := GetBaseURL()

	// Remove the /app prefix if present
	webPath := strings.TrimPrefix(filePath, "/app")

	// Ensure webPath starts with /
	if !strings.HasPrefix(webPath, "/") {
		webPath = "/" + webPath
	}

	return baseURL + webPath
}

// URLToFilePath converts a web URL back to a file path
// Example: http://localhost/media/user123/pics/photo.jpg -> /app/media/user123/pics/photo.jpg
func URLToFilePath(url string) string {
	baseURL := GetBaseURL()

	// Remove the base URL
	path := strings.TrimPrefix(url, baseURL)

	// Add /app prefix
	return "/app" + path
}

// GetMediaURL returns the full URL for a media file
func GetMediaURL(userID, fileType, filename string) string {
	return GetBaseURL() + "/media/" + userID + "/" + fileType + "/" + filename
}

// GetStreamURL returns the full URL for a stream file
func GetStreamURL(userID, streamID, filename string) string {
	return GetBaseURL() + "/streams/" + userID + "/" + streamID + "/" + filename
}

// GetThumbnailURL returns the URL for a thumbnail
func GetThumbnailURL(originalPath string) string {
	// Assuming thumbnail is in the same directory with "thumb" prefix
	dir := strings.TrimSuffix(originalPath, strings.Split(originalPath, "/")[len(strings.Split(originalPath, "/"))-1])
	filename := strings.Split(originalPath, "/")[len(strings.Split(originalPath, "/"))-1]

	// Add "thumb" prefix to filename
	thumbFilename := "thumb_" + filename

	return FilePathToURL(dir + thumbFilename)
}
