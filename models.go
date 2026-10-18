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

	// Status pendaftaran kantin
	VendorPending  = "pending"
	VendorApproved = "approved"
	VendorRejected = "rejected"

	// Status pembayaran (satu pembayaran QRIS bisa mencakup beberapa kantin)
	PaymentPending = "pending"
	PaymentPaid    = "paid"
	PaymentExpired = "expired"

	// Status pencairan dana kantin
	WithdrawRequested = "requested"
	WithdrawProcessed = "processed"
	WithdrawRejected  = "rejected"

	OrderTakeaway = "takeaway"
	OrderDineIn   = "dine_in"
)

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Kode ambil pendek yang mudah disebut ke kasir (tanpa huruf/angka yang mirip).
func pickupCode() string {
	const chars = "ACDEFGHJKMNPRTUVWXY34679"
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = chars[int(b[i])%len(chars)]
	}
	return string(b)
}

type User struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `json:"name"`
	Email     string    `gorm:"uniqueIndex;size:191" json:"email"`
	Password  string    `json:"-"`
	Role      string    `gorm:"size:20;index" json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// Balance = pendapatan kantin yang belum dicairkan (Rupiah).
type VendorProfile struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	UserID          uint      `gorm:"uniqueIndex" json:"user_id"`
	StoreName       string    `json:"store_name"`
	Description     string    `json:"description"`
	ImageURL        string    `json:"image_url"`
	Location        string    `json:"location"`
	Phone           string    `json:"phone,omitempty"`
	IsOpen          bool      `gorm:"default:true" json:"is_open"`
	Status          string    `gorm:"size:20;index;default:approved" json:"status"`
	Balance         int64     `json:"balance"`
	BankName        string    `json:"bank_name,omitempty"`
	BankAccountNo   string    `json:"bank_account_no,omitempty"`
	BankAccountName string    `json:"bank_account_name,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

type Menu struct {
	ID          uint        `gorm:"primaryKey" json:"id"`
	VendorID    uint        `gorm:"index" json:"vendor_id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Category    string      `gorm:"size:30;index" json:"category"`
	ImageURL    string      `json:"image_url"`
	Price       int64       `json:"price"`
	Stock       int         `json:"stock"`
	IsAvailable bool        `json:"is_available"`
	Addons      []MenuAddon `gorm:"foreignKey:MenuID;constraint:OnDelete:CASCADE" json:"addons"`
	// Hanya dibaca (hasil join), bukan kolom tabel menus.
	StoreName  string `gorm:"->;-:migration" json:"store_name,omitempty"`
	VendorOpen *bool  `gorm:"->;-:migration" json:"vendor_open,omitempty"`
	Sold       *int64 `gorm:"->;-:migration" json:"sold,omitempty"` // jumlah porsi terjual (GET /home)
}

// Tambahan berbayar per menu, mis. telur, bawang goreng, keju.
type MenuAddon struct {
	ID     uint   `gorm:"primaryKey" json:"id"`
	MenuID uint   `gorm:"index" json:"menu_id"`
	Name   string `json:"name"`
	Price  int64  `json:"price"`
}

// Satu pembayaran QRIS untuk satu atau beberapa pesanan (beda kantin).
type Payment struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	StudentID  uint       `gorm:"index" json:"student_id"`
	ExternalID string     `gorm:"uniqueIndex;size:100" json:"external_id"`
	Method     string     `gorm:"size:20" json:"method"`
	Amount     int64      `json:"amount"`
	Status     string     `gorm:"size:20;index" json:"status"`
	XenditID   string     `json:"xendit_id"`
	QRString   string     `json:"qr_string"`
	ExpiresAt  time.Time  `json:"expires_at"`
	PaidAt     *time.Time `json:"paid_at"`
	Orders     []Order    `gorm:"foreignKey:PaymentID" json:"orders,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

type Order struct {
	ID              uint        `gorm:"primaryKey" json:"id"`
	StudentID       uint        `gorm:"index" json:"student_id"`
	VendorID        uint        `gorm:"index" json:"vendor_id"`
	PaymentID       uint        `gorm:"index" json:"payment_id"`
	StoreName       string      `json:"store_name"`
	CustomerName    string      `json:"customer_name"`
	PickupCode      string      `gorm:"size:10" json:"pickup_code"`
	OrderType       string      `gorm:"size:20;default:takeaway" json:"order_type"`
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
	ID         uint   `gorm:"primaryKey" json:"id"`
	OrderID    uint   `gorm:"index" json:"order_id"`
	MenuID     uint   `json:"menu_id"`
	Name       string `json:"name"`
	ImageURL   string `json:"image_url"`
	Price      int64  `json:"price"`
	AddonPrice int64  `json:"addon_price"` // total harga tambahan per porsi
	Addons     string `json:"addons"`      // nama tambahan, dipisah koma
	Note       string `json:"note"`
	Qty        int    `json:"qty"`
}

// Permintaan pencairan saldo kantin ke rekening bank.
type Withdrawal struct {
	ID              uint       `gorm:"primaryKey" json:"id"`
	VendorID        uint       `gorm:"index" json:"vendor_id"`
	StoreName       string     `json:"store_name"`
	Amount          int64      `json:"amount"`
	BankName        string     `json:"bank_name"`
	BankAccountNo   string     `json:"bank_account_no"`
	BankAccountName string     `json:"bank_account_name"`
	Status          string     `gorm:"size:20;index" json:"status"`
	AdminNote       string     `json:"admin_note"`
	ProcessedAt     *time.Time `json:"processed_at"`
	CreatedAt       time.Time  `json:"created_at"`
}
