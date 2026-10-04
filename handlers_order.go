package main

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type orderItemReq struct {
	MenuID uint `json:"menu_id" binding:"required"`
	Qty    int  `json:"qty" binding:"required,min=1"`
}

type createOrderReq struct {
	VendorID   uint           `json:"vendor_id" binding:"required"`
	Items      []orderItemReq `json:"items" binding:"required,min=1,dive"`
	PickupTime *time.Time     `json:"pickup_time"`
}

// Satu checkout = satu kantin = satu pembayaran.
func createOrder(c *gin.Context) {
	var req createOrderReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err.Error())
		return
	}
	uid := c.GetUint("uid")
	var student User
	db.First(&student, uid)
	var vendor VendorProfile
	if err := db.First(&vendor, req.VendorID).Error; err != nil {
		fail(c, 404, "kantin tidak ditemukan")
		return
	}

	order := Order{StudentID: uid, VendorID: vendor.ID, Status: StatusPendingPayment,
		ExternalID: "order-" + randHex(8), PickupTime: req.PickupTime}

	err := db.Transaction(func(tx *gorm.DB) error {
		for _, it := range req.Items {
			var m Menu
			if err := tx.Where("id = ? AND vendor_id = ? AND is_available = ?", it.MenuID, vendor.ID, true).First(&m).Error; err != nil {
				return fmt.Errorf("menu %d tidak tersedia di kantin ini", it.MenuID)
			}
			res := tx.Model(&Menu{}).Where("id = ? AND stock >= ?", m.ID, it.Qty).
				UpdateColumn("stock", gorm.Expr("stock - ?", it.Qty))
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				return fmt.Errorf("stok %s tidak cukup", m.Name)
			}
			order.Subtotal += m.Price * int64(it.Qty)
			order.Items = append(order.Items, OrderItem{MenuID: m.ID, Name: m.Name, Price: m.Price, Qty: it.Qty})
		}
		order.AdminFee = adminFee()
		order.Total = order.Subtotal + order.AdminFee
		return tx.Create(&order).Error
	})
	if err != nil {
		fail(c, 400, err.Error())
		return
	}

	id, url, err := createInvoice(order.ExternalID, order.Total, student.Email, "Pesanan kantin "+vendor.StoreName)
	if err != nil {
		expireOrder(order.ExternalID)
		fail(c, 502, "gagal membuat invoice: "+err.Error())
		return
	}
	order.XenditInvoiceID, order.PaymentURL = id, url
	db.Model(&order).Updates(map[string]any{"xendit_invoice_id": id, "payment_url": url})
	mode := "xendit"
	if mockMode() {
		mode = "mock"
	}
	c.JSON(201, gin.H{"order": order, "payment_mode": mode})
}

func myOrders(c *gin.Context) {
	var orders []Order
	db.Preload("Items").Where("student_id = ?", c.GetUint("uid")).Order("id desc").Find(&orders)
	c.JSON(200, orders)
}

// Dipakai aplikasi mobile untuk polling status (mis. tiap 3-5 detik).
func getOrder(c *gin.Context) {
	var o Order
	if err := db.Preload("Items").First(&o, c.Param("id")).Error; err != nil {
		fail(c, 404, "pesanan tidak ditemukan")
		return
	}
	uid, role := c.GetUint("uid"), c.GetString("role")
	allowed := role == RoleAdmin || (role == RoleStudent && o.StudentID == uid)
	if role == RoleVendor {
		var vp VendorProfile
		allowed = db.Where("user_id = ?", uid).First(&vp).Error == nil && vp.ID == o.VendorID
	}
	if !allowed {
		fail(c, 403, "akses ditolak")
		return
	}
	c.JSON(200, o)
}

// Pembayaran sukses: status -> paid, saldo kantin += subtotal. Aman dipanggil berulang.
func markPaid(externalID string) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var o Order
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("external_id = ?", externalID).First(&o).Error; err != nil {
			return err
		}
		if o.Status != StatusPendingPayment {
			return nil
		}
		if err := tx.Model(&o).Update("status", StatusPaid).Error; err != nil {
			return err
		}
		return tx.Model(&VendorProfile{}).Where("id = ?", o.VendorID).
			UpdateColumn("balance", gorm.Expr("balance + ?", o.Subtotal)).Error
	})
}

// Batal/kedaluwarsa: kembalikan stok. Aman dipanggil berulang.
func expireOrder(externalID string) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var o Order
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("external_id = ?", externalID).First(&o).Error; err != nil {
			return err
		}
		if o.Status != StatusPendingPayment {
			return nil
		}
		var items []OrderItem
		tx.Where("order_id = ?", o.ID).Find(&items)
		for _, it := range items {
			tx.Model(&Menu{}).Where("id = ?", it.MenuID).UpdateColumn("stock", gorm.Expr("stock + ?", it.Qty))
		}
		return tx.Model(&o).Update("status", StatusExpired).Error
	})
}

// Hanya aktif di mode mock: simulasi "pembayaran berhasil" tanpa Xendit.
func devPay(c *gin.Context) {
	var o Order
	if err := db.Where("id = ? AND student_id = ?", c.Param("id"), c.GetUint("uid")).First(&o).Error; err != nil {
		fail(c, 404, "pesanan tidak ditemukan")
		return
	}
	if err := markPaid(o.ExternalID); err != nil {
		fail(c, 500, err.Error())
		return
	}
	db.Preload("Items").First(&o, o.ID)
	c.JSON(200, o)
}

func xenditInvoiceWebhook(c *gin.Context) {
	if !callbackTokenOK(c) {
		fail(c, 401, "callback token salah")
		return
	}
	var p struct {
		ExternalID string `json:"external_id"`
		Status     string `json:"status"`
	}
	if err := c.ShouldBindJSON(&p); err != nil {
		fail(c, 400, err.Error())
		return
	}
	var err error
	switch p.Status {
	case "PAID", "SETTLED":
		err = markPaid(p.ExternalID)
	case "EXPIRED":
		err = expireOrder(p.ExternalID)
	}
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
