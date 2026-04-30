package controllers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"huasteca-backend/internal/config"
	"huasteca-backend/internal/database"
	"huasteca-backend/internal/middleware"
	"huasteca-backend/internal/models"
)

type AuthController struct {
	cfg *config.Config
}

func NewAuthController(cfg *config.Config) *AuthController {
	return &AuthController{cfg: cfg}
}

type GoogleAuthRequest struct {
	AccessToken string `json:"access_token" binding:"required"`
}

func validateGoogleAccessToken(accessToken string, clientID string) (map[string]interface{}, error) {
	// Use Google's tokeninfo endpoint to validate the access token
	url := "https://oauth2.googleapis.com/tokeninfo?access_token=" + accessToken
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("invalid token: status %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	// Verify audience matches our client ID
	if aud, ok := result["aud"].(string); ok && aud != clientID {
		return nil, fmt.Errorf("audience mismatch")
	}

	return result, nil
}

func (ac *AuthController) GoogleAuth(c *gin.Context) {
	var req GoogleAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	payload, err := validateGoogleAccessToken(req.AccessToken, ac.cfg.GetGoogleClientID())
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid Google token"})
		return
	}

	email, _ := payload["email"].(string)
	name, _ := payload["name"].(string)
	pictureURL := ""
	if pic, ok := payload["picture"].(string); ok {
		pictureURL = pic
	}
	sub, _ := payload["sub"].(string)

	usersCollection := database.GetCollection("users")

	var existingUser models.User
	err = usersCollection.FindOne(c.Request.Context(), bson.M{"email": email}).Decode(&existingUser)

	if err == nil {
		log.Printf("[LOGIN] User logged in: email=%s, role=%s", email, existingUser.Role)
		token, err := generateJWT(existingUser.ID.Hex(), string(existingUser.Role), ac.cfg.GetJWTKey())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"token": token, "user": existingUser})
		return
	}

	if err != mongo.ErrNoDocuments {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	newUser := models.User{
		ID:         primitive.NewObjectID(),
		Email:      email,
		Name:       name,
		PictureURL: pictureURL,
		Role:       models.RoleCLIENTE,
		GoogleID:   sub,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	_, err = usersCollection.InsertOne(c.Request.Context(), newUser)
	if err != nil {
		log.Printf("[ERROR] Failed to create user: email=%s, error=%v", email, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}
	log.Printf("[REGISTER] New user registered: email=%s, role=CLIENTE (default), google_id=%s", email, sub)

	token, err := generateJWT(newUser.ID.Hex(), string(newUser.Role), ac.cfg.GetJWTKey())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": token, "user": newUser})
}

func generateJWT(userID string, role string, jwtKey []byte) (string, error) {
	claims := middleware.Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtKey)
}

func (ac *AuthController) GetCurrentUser(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	usersCollection := database.GetCollection("users")

	var user models.User
	err := usersCollection.FindOne(c.Request.Context(), bson.M{"_id": userID}).Decode(&user)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"user": user})
}

func (ac *AuthController) UpdateProfile(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var req struct {
		Name       string `json:"name"`
		PictureURL string `json:"picture_url"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	usersCollection := database.GetCollection("users")

	update := bson.M{
		"$set": bson.M{
			"name":        req.Name,
			"picture_url":  req.PictureURL,
			"updated_at":  time.Now(),
		},
	}

	_, err := usersCollection.UpdateOne(c.Request.Context(), bson.M{"_id": userID}, update)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update profile"})
		return
	}

	var updatedUser models.User
	usersCollection.FindOne(c.Request.Context(), bson.M{"_id": userID}).Decode(&updatedUser)

	c.JSON(http.StatusOK, gin.H{"user": updatedUser})
}

func (ac *AuthController) GetUsers(c *gin.Context) {
	usersCollection := database.GetCollection("users")

	cursor, err := usersCollection.Find(c.Request.Context(), bson.M{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch users"})
		return
	}
	defer cursor.Close(c.Request.Context())

	var users []models.User
	if err := cursor.All(c.Request.Context(), &users); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode users"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"users": users})
}

func (ac *AuthController) UploadProfileImage(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	file, header, err := c.Request.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Image file required"})
		return
	}
	defer file.Close()

	if header.Size > 2*1024*1024 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File size must be less than 2MB"})
		return
	}

	contentType := header.Header.Get("Content-Type")
	if contentType != "image/jpeg" && contentType != "image/png" && contentType != "image/gif" && contentType != "image/webp" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Only image files are allowed"})
		return
	}

	ext := filepath.Ext(header.Filename)
	filename := fmt.Sprintf("profile_%d_%s%s", time.Now().Unix(), userID.(primitive.ObjectID).Hex(), ext)
	uploadPath := filepath.Join("./public/uploads", filename)

	if err := os.MkdirAll("./public/uploads", 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create upload directory"})
		return
	}

	out, err := os.Create(uploadPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create file"})
		return
	}
	defer out.Close()

	if _, err := io.Copy(out, file); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}

	relativePath := fmt.Sprintf("/uploads/%s", filename)

	usersCollection := database.GetCollection("users")
	update := bson.M{
		"$set": bson.M{
			"picture_url": relativePath,
			"updated_at":  time.Now(),
		},
	}

	_, err = usersCollection.UpdateOne(c.Request.Context(), bson.M{"_id": userID}, update)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update profile picture"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"url": relativePath, "message": "Profile picture updated"})
}