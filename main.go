package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var db *gorm.DB

// Gambar contoh menu untuk data demo, diakses di /assets/food/*.svg
//
//go:embed assets
var assetFiles embed.FS

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
	db.Create(&User{Name: "Admin", Email: email, Password: hashPassword(pass), Role: RoleAdmin})
	log.Println("admin awal dibuat:", email)
}

func main() {
	_ = godotenv.Load()
	if os.Getenv("JWT_SECRET") == "" {
		log.Fatal("JWT_SECRET wajib diisi")
	}
	db = connectDB()
	if err := db.AutoMigrate(&User{}, &VendorProfile{}, &Menu{}, &MenuAddon{}, &Payment{}, &Order{}, &OrderItem{}, &Withdrawal{}); err != nil {
		log.Fatal(err)
	}
	seedAdmin()
	if os.Getenv("SEED_DEMO") == "true" {
		seedDemo()
	}
	if mockMode() {
		log.Println("MODE MOCK: pembayaran QRIS disimulasikan lewat POST /api/dev/payments/:id/pay")
	}
	go expireStalePayments()

	r := gin.Default()
	r.MaxMultipartMemory = 4 << 20
	api := r.Group("/api")
	api.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true, "payment_mode": paymentMode(), "admin_fee": adminFee()})
	})

	// publik
	api.POST("/register", register)
	api.POST("/login", login)
	api.POST("/vendors/register", vendorRegister)
	api.GET("/vendors", listVendors)
	api.GET("/vendors/:id", getVendor)
	api.GET("/menus", listMenus)
	api.GET("/home", home)
	api.POST("/webhooks/xendit/qris", xenditQRWebhook)
	api.POST("/webhooks/xendit/invoice", xenditInvoiceWebhook)

	// perlu login
	auth := api.Group("", authRequired())
	auth.GET("/me", me)
	auth.GET("/orders/:id", getOrder)

	student := auth.Group("", roleRequired(RoleStudent))
	student.POST("/checkout", checkout)
	student.POST("/orders", createOrder)
	student.GET("/orders", myOrders)
	student.GET("/payments/:id", getPayment)
	student.GET("/payments/:id/qr.png", paymentQR)
	if mockMode() || xenditTestMode() {
		student.POST("/dev/payments/:id/simulate", simulatePayment)
	}
	if mockMode() {
		student.POST("/dev/payments/:id/pay", devPayPayment)
		student.POST("/dev/pay/:id", devPay)
	}

	vendor := auth.Group("/vendor", roleRequired(RoleVendor))
	vendor.GET("/profile", vendorGetProfile)
	vendor.PUT("/profile", vendorUpdateProfile)
	vendor.POST("/upload", vendorUpload)
	vendor.GET("/menus", vendorListMenus)
	vendor.POST("/menus", vendorCreateMenu)
	vendor.PUT("/menus/:id", vendorUpdateMenu)
	vendor.DELETE("/menus/:id", vendorDeleteMenu)
	vendor.GET("/orders", vendorOrders)
	vendor.PATCH("/orders/:id/status", vendorUpdateOrderStatus)
	vendor.GET("/withdrawals", vendorWithdrawals)
	vendor.POST("/withdrawals", vendorRequestWithdrawal)

	admin := auth.Group("/admin", roleRequired(RoleAdmin))
	admin.POST("/vendors", adminCreateVendor)
	admin.GET("/vendors", adminListVendors)
	admin.PATCH("/vendors/:id/status", adminSetVendorStatus)
	admin.GET("/withdrawals", adminListWithdrawals)
	admin.PATCH("/withdrawals/:id", adminProcessWithdrawal)

	// Foto menu/kantin hasil unggahan + gambar contoh data demo.
	r.Static("/uploads", uploadDir())
	assets, _ := fs.Sub(assetFiles, "assets")
	r.StaticFS("/assets", http.FS(assets))

	log.Fatal(r.Run(":" + getenv("PORT", "8080")))
}
