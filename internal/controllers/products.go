package controllers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"huasteca-backend/internal/database"
	"huasteca-backend/internal/models"
)

type ProductController struct{}

func NewProductController() *ProductController {
	return &ProductController{}
}

func (pc *ProductController) GetProducts(c *gin.Context) {
	productsCollection := database.GetCollection("products")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	cursor, err := productsCollection.Find(ctx, bson.M{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch products"})
		return
	}
	defer cursor.Close(ctx)

	var products []models.Product
	if err := cursor.All(ctx, &products); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode products"})
		return
	}

	// Convert relative URLs to absolute URLs
	baseURL := "http://" + c.Request.Host
	for i := range products {
		if products[i].ImagenURL != "" && products[i].ImagenURL[0] == '/' {
			products[i].ImagenURL = baseURL + products[i].ImagenURL
		}
	}

	c.JSON(http.StatusOK, gin.H{"products": products})
}

func (pc *ProductController) GetProduct(c *gin.Context) {
	idParam := c.Param("id")
	productID, err := primitive.ObjectIDFromHex(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
		return
	}

	productsCollection := database.GetCollection("products")

	var product models.Product
	err = productsCollection.FindOne(c.Request.Context(), bson.M{"_id": productID}).Decode(&product)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}

	// Convert relative URL to absolute URL
	baseURL := "http://" + c.Request.Host
	if product.ImagenURL != "" && product.ImagenURL[0] == '/' {
		product.ImagenURL = baseURL + product.ImagenURL
	}

	c.JSON(http.StatusOK, gin.H{"product": product})
}

func (pc *ProductController) CreateProduct(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	type ProductInput struct {
		Nombre      string  `json:"nombre" binding:"required"`
		Descripcion string  `json:"descripcion"`
		Precio      float64 `json:"precio" binding:"required,gt=0"`
		Stock       int     `json:"stock" binding:"gte=0"`
		ImagenURL   string  `json:"imagen_url"`
	}

	var input ProductInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	productsCollection := database.GetCollection("products")

	newProduct := models.Product{
		ID:          primitive.NewObjectID(),
		Nombre:      input.Nombre,
		Descripcion: input.Descripcion,
		Precio:      input.Precio,
		Stock:       input.Stock,
		ImagenURL:   input.ImagenURL,
		VendedorID:  userID.(primitive.ObjectID),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	result, err := productsCollection.InsertOne(c.Request.Context(), newProduct)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create product"})
		return
	}

	newProduct.ID = result.InsertedID.(primitive.ObjectID)
	c.JSON(http.StatusCreated, gin.H{"product": newProduct})
}

func (pc *ProductController) UpdateProduct(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	idParam := c.Param("id")
	productID, err := primitive.ObjectIDFromHex(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
		return
	}

	type ProductInput struct {
		Nombre      string  `json:"nombre"`
		Descripcion string  `json:"descripcion"`
		Precio      float64 `json:"precio"`
		Stock       int     `json:"stock"`
		ImagenURL   string  `json:"imagen_url"`
	}

	var input ProductInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	productsCollection := database.GetCollection("products")

	var existingProduct models.Product
	err = productsCollection.FindOne(c.Request.Context(), bson.M{"_id": productID}).Decode(&existingProduct)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}

	// Check if user is admin or the vendor
	roleVal, _ := c.Get("role")
	userRole, _ := roleVal.(models.Role)
	if userRole != models.RoleADMIN && existingProduct.VendedorID != userID.(primitive.ObjectID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not authorized to update this product"})
		return
	}

	update := bson.M{
		"$set": bson.M{
			"updated_at": time.Now(),
		},
	}

	if input.Nombre != "" {
		update["$set"].(bson.M)["nombre"] = input.Nombre
	}
	if input.Descripcion != "" {
		update["$set"].(bson.M)["descripcion"] = input.Descripcion
	}
	if input.Precio > 0 {
		update["$set"].(bson.M)["precio"] = input.Precio
	}
	if input.Stock >= 0 {
		update["$set"].(bson.M)["stock"] = input.Stock
	}
	if input.ImagenURL != "" {
		update["$set"].(bson.M)["imagen_url"] = input.ImagenURL
	}

	_, err = productsCollection.UpdateOne(c.Request.Context(), bson.M{"_id": productID}, update)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update product"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Product updated"})
}

func (pc *ProductController) DeleteProduct(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	idParam := c.Param("id")
	productID, err := primitive.ObjectIDFromHex(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
		return
	}

	productsCollection := database.GetCollection("products")

	var existingProduct models.Product
	err = productsCollection.FindOne(c.Request.Context(), bson.M{"_id": productID}).Decode(&existingProduct)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}

	// Check if user is admin or the vendor
	roleVal, _ := c.Get("role")
	userRole, _ := roleVal.(models.Role)
	if userRole != models.RoleADMIN && existingProduct.VendedorID != userID.(primitive.ObjectID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not authorized to delete this product"})
		return
	}

	_, err = productsCollection.DeleteOne(c.Request.Context(), bson.M{"_id": productID})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete product"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Product deleted"})
}

func (pc *ProductController) UploadImage(c *gin.Context) {
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
	filename := fmt.Sprintf("%d_%s%s", time.Now().Unix(), userID.(primitive.ObjectID).Hex(), ext)
	uploadPath := filepath.Join(".", "uploads", filename)

	if err := os.MkdirAll("./uploads", 0755); err != nil {
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
	c.JSON(http.StatusOK, gin.H{"url": relativePath})
}
