package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Role string

const (
	RoleADMIN   Role = "ADMIN"
	RoleVENDEDOR Role = "VENDEDOR"
	RoleCLIENTE Role = "CLIENTE"
)

type User struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Email     string            `bson:"email" json:"email"`
	Name      string            `bson:"name" json:"name"`
	PictureURL string           `bson:"picture_url" json:"picture_url"`
	Role      Role              `bson:"role" json:"role"`
	GoogleID  string            `bson:"google_id" json:"google_id"`
	CreatedAt time.Time        `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time        `bson:"updated_at" json:"updated_at"`
}

type Product struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Nombre      string             `bson:"nombre" json:"nombre"`
	Descripcion string             `bson:"descripcion" json:"descripcion"`
	Precio      float64           `bson:"precio" json:"precio"`
	Stock       int               `bson:"stock" json:"stock"`
	ImagenURL   string             `bson:"imagen_url" json:"imagen_url"`
	VendedorID  primitive.ObjectID `bson:"vendedor_id" json:"vendedor_id"`
	CreatedAt   time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt   time.Time          `bson:"updated_at" json:"updated_at"`
}

type Venta struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	ProductoID   primitive.ObjectID `bson:"producto_id" json:"producto_id"`
	ProductoNombre string           `bson:"producto_nombre" json:"producto_nombre"`
	Cantidad    int                `bson:"cantidad" json:"cantidad"`
	PrecioUnitario float64         `bson:"precio_unitario" json:"precio_unitario"`
	Total       float64           `bson:"total" json:"total"`
	CompradorID primitive.ObjectID `bson:"comprador_id" json:"comprador_id"`
	VendedorID  primitive.ObjectID `bson:"vendedor_id" json:"vendedor_id"`
	Fecha       time.Time          `bson:"fecha" json:"fecha"`
}