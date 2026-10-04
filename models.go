package main

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

const (
	RoleStudent = "student"
	RoleVendor  = "vendor"
	RoleAdmin   = "admin"

	StatusPendingPayment = "pending_payment"
	StatusPaid           = "paid"
	StatusPreparing      = "preparing"
	StatusReady          = "ready"
	StatusCompleted      = "completed"
	StatusExpired        = "expired"
)

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

type User struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `json:"name"`
	Email     string    `gorm:"uniqueIndex;size:191" json:"email"`
	Password  string    `json:"-"`
	Role      string    `gorm:"size:20;index" json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// Balance = total pendapatan kantin dari pesanan yang sudah dibayar (Rupiah).
type VendorProfile struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	UserID      uint   `gorm:"uniqueIndex" json:"user_id"`
	StoreName   string `json:"store_name"`
	Description string `json:"description"`
	Balance     int64  `json:"balance"`
}

type Menu struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	VendorID    uint   `gorm:"index" json:"vendor_id"`
	Name        string `json:"name"`
	Price       int64  `json:"price"`
	Stock       int    `json:"stock"`
	IsAvailable bool   `json:"is_available"`
}

type Order struct {
	ID              uint        `gorm:"primaryKey" json:"id"`
	StudentID       uint        `gorm:"index" json:"student_id"`
	VendorID        uint        `gorm:"index" json:"vendor_id"`
	Subtotal        int64       `json:"subtotal"`
	AdminFee        int64       `json:"admin_fee"`
	Total           int64       `json:"total"`
	Status          string      `gorm:"size:30;index" json:"status"`
	PickupTime      *time.Time  `json:"pickup_time"`
	ExternalID      string      `gorm:"uniqueIndex;size:100" json:"external_id"`
	XenditInvoiceID string      `json:"xendit_invoice_id"`
	PaymentURL      string      `json:"payment_url"`
	Items           []OrderItem `gorm:"foreignKey:OrderID" json:"items"`
	CreatedAt       time.Time   `json:"created_at"`
}

type OrderItem struct {
	ID      uint   `gorm:"primaryKey" json:"id"`
	OrderID uint   `gorm:"index" json:"order_id"`
	MenuID  uint   `json:"menu_id"`
	Name    string `json:"name"`
	Price   int64  `json:"price"`
	Qty     int    `json:"qty"`
}
