package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var db *gorm.DB

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func connectDB() *gorm.DB {
	dsn := os.Getenv("DB_DSN")
	g, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("gagal konek database: ", err)
	}
	return g
}

func seedAdmin() {
	email, pass := os.Getenv("ADMIN_EMAIL"), os.Getenv("ADMIN_PASSWORD")
	if email == "" || pass == "" {
		return
	}
	var n int64
	db.Model(&User{}).Where("role = ?", RoleAdmin).Count(&n)
	if n > 0 {
		return
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
	db.Create(&User{Name: "Admin", Email: email, Password: string(hash), Role: RoleAdmin})
	log.Println("admin awal dibuat:", email)
}

func main() {
	_ = godotenv.Load()
	if os.Getenv("JWT_SECRET") == "" {
		log.Fatal("JWT_SECRET wajib diisi")
	}
	db = connectDB()
	if err := db.AutoMigrate(&User{}, &VendorProfile{}, &Menu{}, &Order{}, &OrderItem{}); err != nil {
		log.Fatal(err)
	}
	seedAdmin()
	if mockMode() {
		log.Println("MODE MOCK: pembayaran disimulasikan lewat POST /api/dev/pay/:id")
	}

	r := gin.Default()
	api := r.Group("/api")
	api.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true, "payment_mode": map[bool]string{true: "mock", false: "xendit"}[mockMode()]}) })

	// publik
	api.POST("/register", register)
	api.POST("/login", login)
	api.GET("/vendors", listVendors)
	api.GET("/menus", listMenus)
	api.POST("/webhooks/xendit/invoice", xenditInvoiceWebhook)

	// perlu login
	auth := api.Group("", authRequired())
	auth.GET("/me", me)
	auth.GET("/orders/:id", getOrder)

	student := auth.Group("", roleRequired(RoleStudent))
	student.POST("/orders", createOrder)
	student.GET("/orders", myOrders)
	if mockMode() {
		student.POST("/dev/pay/:id", devPay)
	}

	vendor := auth.Group("/vendor", roleRequired(RoleVendor))
	vendor.GET("/profile", vendorGetProfile)
	vendor.PUT("/profile", vendorUpdateProfile)
	vendor.GET("/menus", vendorListMenus)
	vendor.POST("/menus", vendorCreateMenu)
	vendor.PUT("/menus/:id", vendorUpdateMenu)
	vendor.GET("/orders", vendorOrders)
	vendor.PATCH("/orders/:id/status", vendorUpdateOrderStatus)

	admin := auth.Group("/admin", roleRequired(RoleAdmin))
	admin.POST("/vendors", adminCreateVendor)

	log.Fatal(r.Run(":" + getenv("PORT", "8080")))
}
