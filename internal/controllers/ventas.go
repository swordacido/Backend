package controllers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"huasteca-backend/internal/database"
	"huasteca-backend/internal/models"
)

type VentasController struct{}

func NewVentasController() *VentasController {
	return &VentasController{}
}

func (vc *VentasController) GetVentas(c *gin.Context) {
	userID, _ := c.Get("user_id")
	roleVal, _ := c.Get("role")
	role := roleVal.(models.Role)

	ventasCollection := database.GetCollection("ventas")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	var cursor *mongo.Cursor
	var err error

	switch role {
	case models.RoleADMIN:
		cursor, err = ventasCollection.Find(ctx, bson.M{})
	case models.RoleVENDEDOR:
		cursor, err = ventasCollection.Find(ctx, bson.M{"vendedor_id": userID})
	default:
		cursor, err = ventasCollection.Find(ctx, bson.M{"comprador_id": userID})
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching sales"})
		return
	}
	defer cursor.Close(ctx)

	var ventas []models.Venta
	if err := cursor.All(ctx, &ventas); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error decoding sales"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ventas": ventas})
}

func (vc *VentasController) CreateVenta(c *gin.Context) {
	userIDVal, _ := c.Get("user_id")
	userID := userIDVal.(primitive.ObjectID)

	type VentaInput struct {
		ProductoID string `json:"producto_id" binding:"required"`
		Cantidad   int    `json:"cantidad" binding:"required,gt=0"`
	}

	var input VentaInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	productoID, err := primitive.ObjectIDFromHex(input.ProductoID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
		return
	}

	productsCollection := database.GetCollection("products")
	var product models.Product
	err = productsCollection.FindOne(c.Request.Context(), bson.M{"_id": productoID}).Decode(&product)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}

	if product.Stock < input.Cantidad {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Not enough stock"})
		return
	}

	total := product.Precio * float64(input.Cantidad)

	ventasCollection := database.GetCollection("ventas")
	venta := models.Venta{
		ID:             primitive.NewObjectID(),
		ProductoID:    productoID,
		ProductoNombre: product.Nombre,
		Cantidad:     input.Cantidad,
		PrecioUnitario: product.Precio,
		Total:       total,
		CompradorID: userID,
		VendedorID:  product.VendedorID,
		Fecha:       time.Now(),
	}

	_, err = ventasCollection.InsertOne(c.Request.Context(), venta)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create sale"})
		return
	}

	productsCollection.UpdateOne(c.Request.Context(), bson.M{"_id": productoID}, bson.M{"$set": bson.M{"stock": product.Stock - input.Cantidad, "updated_at": time.Now()}})

	c.JSON(http.StatusCreated, gin.H{"message": "Purchase completed", "venta": venta})
}