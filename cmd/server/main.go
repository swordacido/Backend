package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"huasteca-backend/internal/config"
	"huasteca-backend/internal/controllers"
	"huasteca-backend/internal/database"
	"huasteca-backend/internal/middleware"
	"huasteca-backend/internal/models"
)

func main() {
	cfg := config.LoadConfig()

	if err := database.Connect(cfg.GetMongoURI()); err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := database.Disconnect(ctx); err != nil {
			log.Printf("Error disconnecting from MongoDB: %v", err)
		}
	}()

	if err := database.CreateIndexes(); err != nil {
		log.Printf("Warning: Failed to create indexes: %v", err)
	}

	r := gin.Default()

	corsConfig := cors.DefaultConfig()
	corsConfig.AllowOrigins = cfg.GetCORSOrigins()
	corsConfig.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	corsConfig.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization"}
	corsConfig.AllowCredentials = true
	corsConfig.MaxAge = 12 * 60 * 60
	r.Use(cors.New(corsConfig))

	r.Static("/uploads", "./public/uploads")

	// Serve Angular production build
	r.Static("/assets", "./public/assets")
	r.StaticFile("/favicon.ico", "./public/favicon.ico")
	r.NoRoute(func(c *gin.Context) {
		c.File("./public/index.html")
	})

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	authController := controllers.NewAuthController(cfg)
	productController := controllers.NewProductController()
	ventasController := controllers.NewVentasController()

	api := r.Group("/api/v1")
	{
		auth := api.Group("/auth")
		{
			auth.POST("/google", authController.GoogleAuth)
		}

		products := api.Group("/products")
		{
			products.GET("", productController.GetProducts)
			products.GET("/:id", productController.GetProduct)
		}

		authenticated := api.Group("")
		authenticated.Use(middleware.AuthMiddleware(cfg.JWT_SECRET))
		{
			authenticated.GET("/me", authController.GetCurrentUser)

			authenticatedProducts := authenticated.Group("/products")
			{
				authenticatedProducts.POST("", productController.CreateProduct)
				authenticatedProducts.PUT("/:id", productController.UpdateProduct)
				authenticatedProducts.DELETE("/:id", productController.DeleteProduct)
				authenticatedProducts.POST("/upload", productController.UploadImage)
			}

			ventas := authenticated.Group("/ventas")
			{
				ventas.POST("", ventasController.CreateVenta)
			}

			admin := authenticated.Group("/admin")
			admin.Use(middleware.RBACMiddleware(models.RoleADMIN))
			{
				admin.GET("/ventas", ventasController.GetVentas)
			}
		}
	}

	srv := &http.Server{
		Addr:    cfg.GetServerAddress(),
		Handler: r,
	}

	go func() {
		log.Printf("Server starting on %s", cfg.GetServerAddress())
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}