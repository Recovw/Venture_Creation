package main

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	qrcode "github.com/skip2/go-qrcode"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type orderItemReq struct {
	MenuID   uint   `json:"menu_id" binding:"required"`
	Qty      int    `json:"qty" binding:"required,min=1"`
	AddonIDs []uint `json:"addon_ids"`
	Note     string `json:"note" binding:"max=200"`
}

// Satu keranjang = satu kantin.
type cartReq struct {
	VendorID   uint           `json:"vendor_id" binding:"required"`
	Items      []orderItemReq `json:"items" binding:"required,min=1,dive"`
	PickupTime *flexTime      `json:"pickup_time"`
	OrderType  string         `json:"order_type"`
}

type checkoutReq struct {
	Carts []cartReq `json:"carts" binding:"required,min=1,max=10,dive"`
}

// POST /checkout: beberapa keranjang (beda kantin) dibayar sekaligus dengan satu QRIS.
func checkout(c *gin.Context) {
	var req checkoutReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err.Error())
		return
	}
	p, err := placeOrders(c.GetUint("uid"), req.Carts)
	if err != nil {
		fail(c, 400, err.Error())
		return
	}
	c.JSON(201, gin.H{"payment": p, "payment_mode": paymentMode()})
}

// POST /orders (versi lama, satu kantin) tetap didukung untuk aplikasi mobile.
func createOrder(c *gin.Context) {
	var req cartReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err.Error())
		return
	}
	p, err := placeOrders(c.GetUint("uid"), []cartReq{req})
	if err != nil {
		fail(c, 400, err.Error())
		return
	}
	c.JSON(201, gin.H{"order": p.Orders[0], "payment": p, "payment_mode": paymentMode()})
}

func paymentMode() string {
	if mockMode() {
		return "mock"
	}
	return "xendit"
}

func placeOrders(uid uint, carts []cartReq) (*Payment, error) {
	var student User
	db.First(&student, uid)
	seen := map[uint]bool{}
	for _, ct := range carts {
		if seen[ct.VendorID] {
			return nil, fmt.Errorf("satu kantin hanya boleh satu keranjang per checkout")
		}
		seen[ct.VendorID] = true
	}

	p := Payment{StudentID: uid, ExternalID: "pay-" + randHex(8), Method: "qris",
		Status: PaymentPending, ExpiresAt: time.Now().Add(paymentTTL()).Truncate(time.Second)}

	err := db.Transaction(func(tx *gorm.DB) error {
		for i, ct := range carts {
			var vendor VendorProfile
			if err := tx.First(&vendor, ct.VendorID).Error; err != nil || vendor.Status != VendorApproved {
				return fmt.Errorf("kantin tidak ditemukan")
			}
			if !vendor.IsOpen {
				return fmt.Errorf("%s sedang tutup", vendor.StoreName)
			}
			orderType := OrderTakeaway
			if ct.OrderType == OrderDineIn {
				orderType = OrderDineIn
			}
			o := Order{StudentID: uid, VendorID: vendor.ID, StoreName: vendor.StoreName, CustomerName: student.Name,
				PickupCode: pickupCode(), OrderType: orderType, Status: StatusPendingPayment,
				ExternalID: "order-" + randHex(8), PickupTime: ct.PickupTime.Time()}
			for _, it := range ct.Items {
				item, err := buildItem(tx, vendor.ID, it)
				if err != nil {
					return err
				}
				o.Subtotal += (item.Price + item.AddonPrice) * int64(item.Qty)
				o.Items = append(o.Items, item)
			}
			// Biaya layanan cukup sekali per checkout, dibebankan ke pesanan pertama.
			if i == 0 {
				o.AdminFee = adminFee()
			}
			o.Total = o.Subtotal + o.AdminFee
			p.Amount += o.Total
			p.Orders = append(p.Orders, o)
		}
		return tx.Create(&p).Error
	})
	if err != nil {
		return nil, err
	}

	id, qr, err := createQRIS(p.ExternalID, p.Amount, p.ExpiresAt)
	if err != nil {
		expirePayment(p.ExternalID)
		return nil, fmt.Errorf("gagal membuat QRIS: %v", err)
	}
	p.XenditID, p.QRString = id, qr
	db.Model(&p).Updates(map[string]any{"xendit_id": id, "qr_string": qr})
	return &p, nil
}

func buildItem(tx *gorm.DB, vendorID uint, it orderItemReq) (OrderItem, error) {
	var m Menu
	if err := tx.Preload("Addons").Where("id = ? AND vendor_id = ? AND is_available = ?", it.MenuID, vendorID, true).First(&m).Error; err != nil {
		return OrderItem{}, fmt.Errorf("menu %d tidak tersedia di kantin ini", it.MenuID)
	}
	res := tx.Model(&Menu{}).Where("id = ? AND stock >= ?", m.ID, it.Qty).
		UpdateColumn("stock", gorm.Expr("stock - ?", it.Qty))
	if res.Error != nil {
		return OrderItem{}, res.Error
	}
	if res.RowsAffected == 0 {
		return OrderItem{}, fmt.Errorf("stok %s tidak cukup", m.Name)
	}
	item := OrderItem{MenuID: m.ID, Name: m.Name, ImageURL: m.ImageURL, Price: m.Price, Qty: it.Qty, Note: strings.TrimSpace(it.Note)}
	var names []string
	for _, aid := range it.AddonIDs {
		found := false
		for _, a := range m.Addons {
			if a.ID == aid {
				item.AddonPrice += a.Price
				names = append(names, a.Name)
				found = true
			}
		}
		if !found {
			return OrderItem{}, fmt.Errorf("tambahan tidak valid untuk %s", m.Name)
		}
	}
	item.Addons = strings.Join(names, ", ")
	return item, nil
}

func myOrders(c *gin.Context) {
	var orders []Order
	db.Preload("Items").Where("student_id = ?", c.GetUint("uid")).Order("id desc").Limit(100).Find(&orders)
	c.JSON(200, orders)
}

// Dipakai aplikasi untuk polling status (mis. tiap 3-5 detik).
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

func myPayment(c *gin.Context) (Payment, bool) {
	var p Payment
	if err := db.Where("id = ? AND student_id = ?", c.Param("id"), c.GetUint("uid")).First(&p).Error; err != nil {
		fail(c, 404, "pembayaran tidak ditemukan")
		return p, false
	}
	// QRIS kedaluwarsa: batalkan pesanan dan kembalikan stok.
	if p.Status == PaymentPending && time.Now().After(p.ExpiresAt) {
		expirePayment(p.ExternalID)
	}
	db.Preload("Orders.Items").First(&p, p.ID)
	return p, true
}

// GET /payments/:id: status pembayaran + pesanan di dalamnya (untuk polling).
func getPayment(c *gin.Context) {
	if p, ok := myPayment(c); ok {
		c.JSON(200, gin.H{"payment": p, "payment_mode": paymentMode()})
	}
}

// GET /payments/:id/qr.png: gambar QRIS yang siap discan e-wallet / m-banking.
func paymentQR(c *gin.Context) {
	p, ok := myPayment(c)
	if !ok {
		return
	}
	png, err := qrcode.Encode(p.QRString, qrcode.Medium, 512)
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	c.Header("Cache-Control", "no-store")
	c.DataFromReader(200, int64(len(png)), "image/png", bytes.NewReader(png), nil)
}

// Pembayaran sukses: semua pesanan -> paid, saldo tiap kantin += subtotal. Aman dipanggil berulang.
// externalID boleh milik Payment (QRIS) atau Order lama (invoice).
func markPaid(externalID string) error {
	return db.Transaction(func(tx *gorm.DB) error {
		orders, err := lockPendingOrders(tx, externalID, PaymentPaid)
		if err != nil {
			return err
		}
		for _, o := range orders {
			if err := tx.Model(&o).Update("status", StatusPaid).Error; err != nil {
				return err
			}
			if err := tx.Model(&VendorProfile{}).Where("id = ?", o.VendorID).
				UpdateColumn("balance", gorm.Expr("balance + ?", o.Subtotal)).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// Batal/kedaluwarsa: kembalikan stok. Aman dipanggil berulang.
func expirePayment(externalID string) error {
	return db.Transaction(func(tx *gorm.DB) error {
		orders, err := lockPendingOrders(tx, externalID, PaymentExpired)
		if err != nil {
			return err
		}
		for _, o := range orders {
			var items []OrderItem
			tx.Where("order_id = ?", o.ID).Find(&items)
			for _, it := range items {
				tx.Model(&Menu{}).Where("id = ?", it.MenuID).UpdateColumn("stock", gorm.Expr("stock + ?", it.Qty))
			}
			if err := tx.Model(&o).Update("status", StatusExpired).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// Mengunci pembayaran/pesanan, mengubah status pembayaran, dan mengembalikan pesanan yang masih menunggu bayar.
func lockPendingOrders(tx *gorm.DB, externalID, newPaymentStatus string) ([]Order, error) {
	var p Payment
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("external_id = ?", externalID).First(&p).Error; err == nil {
		if p.Status != PaymentPending {
			return nil, nil
		}
		upd := map[string]any{"status": newPaymentStatus}
		if newPaymentStatus == PaymentPaid {
			upd["paid_at"] = time.Now()
		}
		if err := tx.Model(&p).Updates(upd).Error; err != nil {
			return nil, err
		}
		var orders []Order
		tx.Where("payment_id = ? AND status = ?", p.ID, StatusPendingPayment).Find(&orders)
		return orders, nil
	}
	var o Order
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("external_id = ?", externalID).First(&o).Error; err != nil {
		return nil, err
	}
	if o.Status != StatusPendingPayment {
		return nil, nil
	}
	return []Order{o}, nil
}

// Membersihkan QRIS yang tidak dibayar sampai batas waktu (stok kembali).
func expireStalePayments() {
	for range time.Tick(time.Minute) {
		var ps []Payment
		db.Where("status = ? AND expires_at < ?", PaymentPending, time.Now()).Find(&ps)
		for _, p := range ps {
			expirePayment(p.ExternalID)
		}
	}
}

// Hanya aktif di mode mock: simulasi "pembayaran berhasil" tanpa Xendit.
func devPayPayment(c *gin.Context) {
	p, ok := myPayment(c)
	if !ok {
		return
	}
	if p.Status != PaymentPending {
		fail(c, 400, "pembayaran sudah "+p.Status)
		return
	}
	if err := markPaid(p.ExternalID); err != nil {
		fail(c, 500, err.Error())
		return
	}
	db.Preload("Orders.Items").First(&p, p.ID)
	c.JSON(200, gin.H{"payment": p})
}

// POST /dev/payments/:id/simulate: tombol "Bayar (simulasi)" untuk demo di aplikasi.
// Mode mock: langsung lunas. Mode test Xendit: minta Xendit menyimulasikan pembayaran QR,
// lalu status berubah lewat callback seperti pembayaran asli (aplikasi tinggal polling).
func simulatePayment(c *gin.Context) {
	if mockMode() {
		devPayPayment(c)
		return
	}
	p, ok := myPayment(c)
	if !ok {
		return
	}
	if p.Status != PaymentPending {
		fail(c, 400, "pembayaran sudah "+p.Status)
		return
	}
	if err := simulateQRIS(p.XenditID); err != nil {
		fail(c, 502, "gagal simulasi pembayaran: "+err.Error())
		return
	}
	c.JSON(202, gin.H{"payment": p, "message": "simulasi dikirim, status berubah setelah callback Xendit"})
}

// Versi lama: bayar berdasarkan id pesanan (melunasi seluruh pembayaran yang memuatnya).
func devPay(c *gin.Context) {
	var o Order
	if err := db.Where("id = ? AND student_id = ?", c.Param("id"), c.GetUint("uid")).First(&o).Error; err != nil {
		fail(c, 404, "pesanan tidak ditemukan")
		return
	}
	ext := o.ExternalID
	var p Payment
	if o.PaymentID != 0 && db.First(&p, o.PaymentID).Error == nil {
		ext = p.ExternalID
	}
	if err := markPaid(ext); err != nil {
		fail(c, 500, err.Error())
		return
	}
	db.Preload("Items").First(&o, o.ID)
	c.JSON(200, o)
}

// Callback Xendit QR Codes (event qr.payment).
func xenditQRWebhook(c *gin.Context) {
	if !callbackTokenOK(c) {
		fail(c, 401, "callback token salah")
		return
	}
	var p struct {
		Event string `json:"event"`
		Data  struct {
			ReferenceID string `json:"reference_id"`
			Status      string `json:"status"`
		} `json:"data"`
	}
	if err := c.ShouldBindJSON(&p); err != nil {
		fail(c, 400, err.Error())
		return
	}
	var err error
	if p.Data.Status == "SUCCEEDED" {
		err = markPaid(p.Data.ReferenceID)
	}
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

// Callback Xendit Invoice (pesanan lama).
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
		err = expirePayment(p.ExternalID)
	}
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
