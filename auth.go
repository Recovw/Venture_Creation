package main

import (
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func fail(c *gin.Context, code int, msg string) { c.JSON(code, gin.H{"error": msg}) }

func newToken(u User) (string, error) {
	claims := jwt.MapClaims{"uid": u.ID, "role": u.Role, "exp": time.Now().Add(72 * time.Hour).Unix()}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(os.Getenv("JWT_SECRET")))
}

func authRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token tidak ada"})
			return
		}
		claims := jwt.MapClaims{}
		tok, err := jwt.ParseWithClaims(strings.TrimPrefix(h, "Bearer "), claims,
			func(t *jwt.Token) (any, error) { return []byte(os.Getenv("JWT_SECRET")), nil },
			jwt.WithValidMethods([]string{"HS256"}))
		uid, ok1 := claims["uid"].(float64)
		role, ok2 := claims["role"].(string)
		if err != nil || !tok.Valid || !ok1 || !ok2 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token tidak valid"})
			return
		}
		c.Set("uid", uint(uid))
		c.Set("role", role)
		c.Next()
	}
}

func roleRequired(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetString("role") != role {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "akses ditolak"})
			return
		}
		c.Next()
	}
}

// Registrasi mandiri: role SELALU student (tidak dibaca dari request).
func register(c *gin.Context) {
	var req struct {
		Name     string `json:"name" binding:"required"`
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required,min=5"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err.Error())
		return
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	u := User{Name: req.Name, Email: strings.ToLower(req.Email), Password: string(hash), Role: RoleStudent}
	if err := db.Create(&u).Error; err != nil {
		fail(c, 409, "email sudah terdaftar")
		return
	}
	tok, _ := newToken(u)
	c.JSON(201, gin.H{"user": u, "token": tok})
}

func login(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err.Error())
		return
	}
	var u User
	if err := db.Where("email = ?", strings.ToLower(req.Email)).First(&u).Error; err != nil ||
		bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(req.Password)) != nil {
		fail(c, 401, "email atau password salah")
		return
	}
	tok, _ := newToken(u)
	c.JSON(200, gin.H{"user": u, "token": tok})
}

func me(c *gin.Context) {
	var u User
	if err := db.First(&u, c.GetUint("uid")).Error; err != nil {
		fail(c, 404, "user tidak ditemukan")
		return
	}
	c.JSON(200, u)
}

// Admin membuat akun vendor + profil kantin.
func adminCreateVendor(c *gin.Context) {
	var req struct {
		Name      string `json:"name" binding:"required"`
		Email     string `json:"email" binding:"required,email"`
		Password  string `json:"password" binding:"required,min=8"`
		StoreName string `json:"store_name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err.Error())
		return
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	u := User{Name: req.Name, Email: strings.ToLower(req.Email), Password: string(hash), Role: RoleVendor}
	vp := VendorProfile{StoreName: req.StoreName}
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&u).Error; err != nil {
			return err
		}
		vp.UserID = u.ID
		return tx.Create(&vp).Error
	})
	if err != nil {
		fail(c, 409, "email sudah terdaftar")
		return
	}
	c.JSON(201, gin.H{"user": u, "vendor": vp})
}
