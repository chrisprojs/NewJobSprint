package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// User struct to map MongoDB data to Go struct with explicit bson tags
type User struct {
	Name             string `json:"name" bson:"name"`
	Email            string `json:"email" bson:"email"`
	PhoneNumber      string `json:"phone_number" bson:"phone_number"`
	City             string `json:"city" bson:"city"`
	JobTitle         string `json:"job_title" bson:"job_title"`
	IdentityDocFront string `json:"identity_doc_front" bson:"identity_doc_front"`
	IdentityDocBack  string `json:"identity_doc_back" bson:"identity_doc_back"`
	SelfieWithDoc    string `json:"selfie_with_doc" bson:"selfie_with_doc"`
	Note             string `json:"notes,omitempty" bson:"notes,omitempty"` // Optional field
}

const MaxUploadSize = 5 * 1024 * 1024

var allowedExtensions = map[string]bool{".jpg": true, ".png": true, ".pdf": true}

// handleFormSubmission processes the form submission and uploads files
func handleFormSubmission(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		// Return error message as JSON
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, `{"message": "Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	// Limit the size of the incoming request to avoid overloading
	r.Body = http.MaxBytesReader(w, r.Body, MaxUploadSize+1024)
	if err := r.ParseMultipartForm(MaxUploadSize); err != nil {
		// Return file size error as JSON
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, `{"message": "File is too large"}`, http.StatusRequestEntityTooLarge)
		return
	}

	user := User{
		Name:        r.FormValue("fullName"),
		Email:       r.FormValue("email"),
		PhoneNumber: r.FormValue("phone"),
		City:        r.FormValue("city"),
		JobTitle:    r.FormValue("jobRole"),
		Note:        r.FormValue("notes"),
	}

	var err error
	user.IdentityDocFront, err = handleFileUpload(r, "idFront")
	if err != nil {
		// Return file upload error as JSON
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, `{"message": "`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	user.IdentityDocBack, err = handleFileUpload(r, "idBack")
	if err != nil {
		// Return file upload error as JSON
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, `{"message": "`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	user.SelfieWithDoc, err = handleFileUpload(r, "selfieWithId")
	if err != nil {
		// Return file upload error as JSON
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, `{"message": "`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	if err := insertUserIntoMongo(user); err != nil {
		// Return database error as JSON
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, `{"message": "Error storing user data"}`, http.StatusInternalServerError)
		return
	}

	// Send success message as JSON
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"message": "Application submitted successfully"})
}

// handleFileUpload reads, validates, and encodes the uploaded file to Base64
func handleFileUpload(r *http.Request, formKey string) (string, error) {
	file, header, err := r.FormFile(formKey)
	if err != nil {
		return "", fmt.Errorf("failed to get file: %w", err)
	}
	defer file.Close()

	if header.Size > MaxUploadSize {
		return "", fmt.Errorf("file size exceeds 5MB limit")
	}

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !allowedExtensions[ext] {
		return "", fmt.Errorf("invalid file type: %s", ext)
	}

	// Read file content into memory and encode it to Base64
	buf := make([]byte, header.Size)
	_, err = file.Read(buf)
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	encoded := base64.StdEncoding.EncodeToString(buf)
	return encoded, nil
}

// insertUserIntoMongo inserts user data into MongoDB
func insertUserIntoMongo(user User) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	clientOptions := options.Client().ApplyURI("mongodb+srv://shamasurrehman509:LqqXCkGoS6WNLXxP@cluster0.2ttxi.mongodb.net/")
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return fmt.Errorf("failed to connect to MongoDB: %w", err)
	}
	log.Println("Db Connect")

	defer client.Disconnect(ctx)

	collection := client.Database("job_sprint").Collection("applications")
	doc := bson.M{
		"name":               user.Name,
		"email":              user.Email,
		"phone_number":       user.PhoneNumber,
		"city":               user.City,
		"job_title":          user.JobTitle,
		"identity_doc_front": user.IdentityDocFront,
		"identity_doc_back":  user.IdentityDocBack,
		"selfie_with_doc":    user.SelfieWithDoc,
	}
	if user.Note != "" {
		doc["notes"] = user.Note
	}

	_, err = collection.InsertOne(ctx, doc)
	if err != nil {
		return fmt.Errorf("failed to insert user: %w", err)
	}
	return nil
}

// handleGetAllUsers retrieves all user data from MongoDB and returns it as JSON
func handleGetAllUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Set a context with timeout for MongoDB operations
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second) // Increased timeout to 10 seconds
	defer cancel()

	// MongoDB connection options
	clientOptions := options.Client().ApplyURI("mongodb+srv://shamasurrehman509:LqqXCkGoS6WNLXxP@cluster0.2ttxi.mongodb.net/")
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to connect to MongoDB: %v", err), http.StatusInternalServerError)
		return
	}
	defer func() {
		if err := client.Disconnect(ctx); err != nil {
			log.Printf("Failed to disconnect from MongoDB: %v", err)
		}
	}()

	// Access the collection
	collection := client.Database("job_sprint").Collection("applications")

	// Retrieve all documents
	cursor, err := collection.Find(ctx, bson.M{})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to retrieve data: %v", err), http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var users []User
	if err = cursor.All(ctx, &users); err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse data: %v", err), http.StatusInternalServerError)
		return
	}

	// Set response headers and write JSON response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(users); err != nil {
		http.Error(w, fmt.Sprintf("Failed to encode data: %v", err), http.StatusInternalServerError)
		return
	}
}

// SetupRoutes sets up HTTP routes and handlers
func SetupRoutes() {
	http.HandleFunc("/submit", handleFormSubmission)
	http.HandleFunc("/users", handleGetAllUsers) // New route for fetching all user data

}
