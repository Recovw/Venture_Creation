package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func myVendor(c *gin.Context) (VendorProfile, bool) {
	var vp VendorProfile
	if err := db.Where("user_id = ?", c.GetUint("uid")).First(&vp).Error; err != nil {
		fail(c, 404, "profil kantin tidak ditemukan")
		return vp, false
	}
	return vp, true
}

// ---------- publik ----------
var publicVendorCols = []string{"id", "store_name", "description", "image_url", "location", "is_open", "status"}

func approvedVendors() []VendorProfile {
	var vs []VendorProfile
	db.Select(publicVendorCols).Where("status = ?", VendorApproved).Order("is_open desc, id").Find(&vs)
	return vs
}

func listVendors(c *gin.Context) {
	c.JSON(200, approvedVendors())
}

// GET /home?limit=10: beranda = menu terlaris (porsi terjual dari pesanan yang sudah dibayar) + daftar kantin.
// Menu yang belum pernah terjual tetap muncul di bawahnya, jadi beranda tidak kosong di awal.
func home(c *gin.Context) {
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if err != nil || limit < 1 || limit > 30 {
		limit = 10
	}
	paidStatuses := []string{StatusPaid, StatusPreparing, StatusReady, StatusCompleted}
	var best []Menu
	db.Model(&Menu{}).Preload("Addons").
		Select("menus.*, vendor_profiles.store_name AS store_name, vendor_profiles.is_open AS vendor_open, COALESCE(s.sold, 0) AS sold").
		Joins("JOIN vendor_profiles ON vendor_profiles.id = menus.vendor_id").
		Joins(`LEFT JOIN (SELECT order_items.menu_id, SUM(order_items.qty) AS sold FROM order_items
			JOIN orders ON orders.id = order_items.order_id WHERE orders.status IN ? GROUP BY order_items.menu_id) s
			ON s.menu_id = menus.id`, paidStatuses).
		Where("menus.is_available = ? AND vendor_profiles.status = ?", true, VendorApproved).
		Order("sold DESC, vendor_profiles.is_open DESC, menus.id").Limit(limit).Find(&best)
	c.JSON(200, gin.H{"best_sellers": best, "vendors": approvedVendors()})
}

func getVendor(c *gin.Context) {
	var v VendorProfile
	if err := db.Select(publicVendorCols).Where("id = ? AND status = ?", c.Param("id"), VendorApproved).First(&v).Error; err != nil {
		fail(c, 404, "kantin tidak ditemukan")
		return
	}
	c.JSON(200, v)
}

// GET /menus?vendor_id=&q=&category=  — q mencari nama/deskripsi menu dan nama kantin.
func listMenus(c *gin.Context) {
	q := db.Model(&Menu{}).Preload("Addons").
		Select("menus.*, vendor_profiles.store_name AS store_name, vendor_profiles.is_open AS vendor_open").
		Joins("JOIN vendor_profiles ON vendor_profiles.id = menus.vendor_id").
		Where("menus.is_available = ? AND vendor_profiles.status = ?", true, VendorApproved)
	if v := c.Query("vendor_id"); v != "" {
		q = q.Where("menus.vendor_id = ?", v)
	}
	if cat := c.Query("category"); cat != "" {
		q = q.Where("menus.category = ?", cat)
	}
	if s := strings.TrimSpace(c.Query("q")); s != "" {
		like := "%" + strings.ToLower(s) + "%"
		q = q.Where("LOWER(menus.name) LIKE ? OR LOWER(menus.description) LIKE ? OR LOWER(vendor_profiles.store_name) LIKE ?", like, like, like)
	}
	var ms []Menu
	q.Order("vendor_profiles.is_open desc, menus.stock > 0 desc, menus.id").Find(&ms)
	c.JSON(200, ms)
}

// ---------- pendaftaran kantin (mitra) ----------
type vendorRegisterReq struct {
	Name            string `json:"name" binding:"required"`
	Email           string `json:"email" binding:"required,email"`
	Password        string `json:"password" binding:"required,min=8"`
	StoreName       string `json:"store_name" binding:"required"`
	Description     string `json:"description"`
	Location        string `json:"location" binding:"required"`
	Phone           string `json:"phone" binding:"required"`
	BankName        string `json:"bank_name" binding:"required"`
	BankAccountNo   string `json:"bank_account_no" binding:"required"`
	BankAccountName string `json:"bank_account_name" binding:"required"`
}

// POST /vendors/register: pemilik kantin daftar sendiri, lalu diverifikasi admin
// (kecuali VENDOR_AUTO_APPROVE=true).
func vendorRegister(c *gin.Context) {
	var req vendorRegisterReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err.Error())
		return
	}
	status := VendorPending
	if os.Getenv("VENDOR_AUTO_APPROVE") == "true" {
		status = VendorApproved
	}
	vp := VendorProfile{StoreName: req.StoreName, Description: req.Description, Location: req.Location,
		Phone: req.Phone, Status: status, IsOpen: true, BankName: req.BankName,
		BankAccountNo: req.BankAccountNo, BankAccountName: req.BankAccountName}
	u, err := createVendorAccount(req.Name, req.Email, req.Password, &vp)
	if err != nil {
		fail(c, 409, "email sudah terdaftar")
		return
	}
	tok, _ := newToken(u)
	c.JSON(201, gin.H{"user": u, "vendor": vp, "token": tok})
}

func createVendorAccount(name, email, password string, vp *VendorProfile) (User, error) {
	u := User{Name: name, Email: strings.ToLower(email), Password: hashPassword(password), Role: RoleVendor}
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&u).Error; err != nil {
			return err
		}
		vp.UserID = u.ID
		return tx.Create(vp).Error
	})
	return u, err
}

// ---------- kantin ----------
func vendorGetProfile(c *gin.Context) {
	if vp, ok := myVendor(c); ok {
		c.JSON(200, vp)
	}
}

func vendorUpdateProfile(c *gin.Context) {
	vp, ok := myVendor(c)
	if !ok {
		return
	}
	var req struct {
		StoreName       *string `json:"store_name"`
		Description     *string `json:"description"`
		ImageURL        *string `json:"image_url"`
		Location        *string `json:"location"`
		Phone           *string `json:"phone"`
		IsOpen          *bool   `json:"is_open"`
		BankName        *string `json:"bank_name"`
		BankAccountNo   *string `json:"bank_account_no"`
		BankAccountName *string `json:"bank_account_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err.Error())
		return
	}
	upd := map[string]any{}
	set := func(col string, v *string) {
		if v != nil {
			upd[col] = strings.TrimSpace(*v)
		}
	}
	if req.StoreName != nil && strings.TrimSpace(*req.StoreName) != "" {
		set("store_name", req.StoreName)
	}
	set("description", req.Description)
	set("image_url", req.ImageURL)
	set("location", req.Location)
	set("phone", req.Phone)
	set("bank_name", req.BankName)
	set("bank_account_no", req.BankAccountNo)
	set("bank_account_name", req.BankAccountName)
	if req.IsOpen != nil {
		upd["is_open"] = *req.IsOpen
	}
	if len(upd) > 0 {
		db.Model(&vp).Updates(upd)
	}
	db.First(&vp, vp.ID)
	c.JSON(200, vp)
}

type addonReq struct {
	Name  string `json:"name" binding:"required"`
	Price int64  `json:"price" binding:"min=0"`
}

type menuReq struct {
	Name        string     `json:"name" binding:"required"`
	Description string     `json:"description"`
	Category    string     `json:"category"`
	ImageURL    string     `json:"image_url"`
	Price       int64      `json:"price" binding:"required,min=1"`
	Stock       int        `json:"stock" binding:"min=0"`
	IsAvailable *bool      `json:"is_available"`
	Addons      []addonReq `json:"addons" binding:"max=15,dive"`
}

func (r menuReq) addons() []MenuAddon {
	out := []MenuAddon{}
	for _, a := range r.Addons {
		out = append(out, MenuAddon{Name: strings.TrimSpace(a.Name), Price: a.Price})
	}
	return out
}

func vendorListMenus(c *gin.Context) {
	vp, ok := myVendor(c)
	if !ok {
		return
	}
	var ms []Menu
	db.Preload("Addons").Where("vendor_id = ?", vp.ID).Order("id").Find(&ms)
	c.JSON(200, ms)
}

func vendorCreateMenu(c *gin.Context) {
	vp, ok := myVendor(c)
	if !ok {
		return
	}
	var req menuReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err.Error())
		return
	}
	if req.Price > qrisMaxAmount {
		fail(c, 400, "harga menu maksimal "+rupiah(qrisMaxAmount))
		return
	}
	m := Menu{VendorID: vp.ID, Name: req.Name, Description: req.Description, Category: req.Category,
		ImageURL: req.ImageURL, Price: req.Price, Stock: req.Stock, IsAvailable: true, Addons: req.addons()}
	if req.IsAvailable != nil {
		m.IsAvailable = *req.IsAvailable
	}
	db.Create(&m)
	c.JSON(201, m)
}

func vendorUpdateMenu(c *gin.Context) {
	vp, ok := myVendor(c)
	if !ok {
		return
	}
	var m Menu
	if err := db.Where("id = ? AND vendor_id = ?", c.Param("id"), vp.ID).First(&m).Error; err != nil {
		fail(c, 404, "menu tidak ditemukan")
		return
	}
	var req menuReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err.Error())
		return
	}
	if req.Price > qrisMaxAmount {
		fail(c, 400, "harga menu maksimal "+rupiah(qrisMaxAmount))
		return
	}
	m.Name, m.Description, m.Category, m.ImageURL = req.Name, req.Description, req.Category, req.ImageURL
	m.Price, m.Stock = req.Price, req.Stock
	if req.IsAvailable != nil {
		m.IsAvailable = *req.IsAvailable
	}
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Omit("Addons").Save(&m).Error; err != nil {
			return err
		}
		// Daftar tambahan hanya diganti bila dikirim, supaya klien lama tidak menghapusnya.
		if req.Addons == nil {
			return nil
		}
		if err := tx.Where("menu_id = ?", m.ID).Delete(&MenuAddon{}).Error; err != nil {
			return err
		}
		adds := req.addons()
		for i := range adds {
			adds[i].MenuID = m.ID
		}
		if len(adds) > 0 {
			return tx.Create(&adds).Error
		}
		return nil
	})
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	db.Preload("Addons").First(&m, m.ID)
	c.JSON(200, m)
}

func vendorDeleteMenu(c *gin.Context) {
	vp, ok := myVendor(c)
	if !ok {
		return
	}
	res := db.Where("id = ? AND vendor_id = ?", c.Param("id"), vp.ID).Delete(&Menu{})
	if res.RowsAffected == 0 {
		fail(c, 404, "menu tidak ditemukan")
		return
	}
	db.Where("menu_id = ?", c.Param("id")).Delete(&MenuAddon{})
	c.JSON(200, gin.H{"ok": true})
}

// POST /vendor/upload (multipart "file"): unggah foto menu / kantin, balikan URL publik.
func vendorUpload(c *gin.Context) {
	if _, ok := myVendor(c); !ok {
		return
	}
	f, err := c.FormFile("file")
	if err != nil {
		fail(c, 400, "file wajib diisi")
		return
	}
	if f.Size > 3<<20 {
		fail(c, 400, "ukuran foto maksimal 3 MB")
		return
	}
	ext := map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/webp": ".webp"}[f.Header.Get("Content-Type")]
	if ext == "" {
		fail(c, 400, "format foto harus JPG, PNG, atau WEBP")
		return
	}
	dir := uploadDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fail(c, 500, err.Error())
		return
	}
	name := randHex(12) + ext
	if err := c.SaveUploadedFile(f, filepath.Join(dir, name)); err != nil {
		fail(c, 500, err.Error())
		return
	}
	c.JSON(201, gin.H{"url": "/uploads/" + name})
}

func uploadDir() string { return getenv("UPLOAD_DIR", "./uploads") }

func vendorOrders(c *gin.Context) {
	vp, ok := myVendor(c)
	if !ok {
		return
	}
	q := db.Preload("Items").Where("vendor_id = ? AND status NOT IN ?", vp.ID, []string{StatusPendingPayment, StatusExpired})
	if s := c.Query("status"); s != "" {
		q = q.Where("status = ?", s)
	}
	var orders []Order
	q.Order("id desc").Limit(200).Find(&orders)
	c.JSON(200, orders)
}

var nextStatus = map[string]string{StatusPaid: StatusPreparing, StatusPreparing: StatusReady, StatusReady: StatusCompleted}

func vendorUpdateOrderStatus(c *gin.Context) {
	vp, ok := myVendor(c)
	if !ok {
		return
	}
	var req struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err.Error())
		return
	}
	var o Order
	if err := db.Where("id = ? AND vendor_id = ?", c.Param("id"), vp.ID).First(&o).Error; err != nil {
		fail(c, 404, "pesanan tidak ditemukan")
		return
	}
	if nextStatus[o.Status] != req.Status {
		fail(c, 400, fmt.Sprintf("transisi %s -> %s tidak valid", o.Status, req.Status))
		return
	}
	db.Model(&o).Update("status", req.Status)
	c.JSON(200, o)
}

// ---------- pencairan dana ----------
const minWithdraw = 10000

func vendorWithdrawals(c *gin.Context) {
	vp, ok := myVendor(c)
	if !ok {
		return
	}
	var ws []Withdrawal
	db.Where("vendor_id = ?", vp.ID).Order("id desc").Find(&ws)
	c.JSON(200, gin.H{"balance": vp.Balance, "min_amount": minWithdraw, "withdrawals": ws})
}

// POST /vendor/withdrawals {amount}: saldo langsung dipotong, admin mentransfer ke rekening kantin.
func vendorRequestWithdrawal(c *gin.Context) {
	vp, ok := myVendor(c)
	if !ok {
		return
	}
	var req struct {
		Amount int64 `json:"amount" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err.Error())
		return
	}
	if req.Amount < minWithdraw {
		fail(c, 400, fmt.Sprintf("minimal pencairan Rp%d", minWithdraw))
		return
	}
	if vp.Status != VendorApproved {
		fail(c, 400, "kantin belum diverifikasi admin")
		return
	}
	if vp.BankName == "" || vp.BankAccountNo == "" || vp.BankAccountName == "" {
		fail(c, 400, "lengkapi data rekening bank di profil kantin terlebih dahulu")
		return
	}
	w := Withdrawal{VendorID: vp.ID, StoreName: vp.StoreName, Amount: req.Amount, BankName: vp.BankName,
		BankAccountNo: vp.BankAccountNo, BankAccountName: vp.BankAccountName, Status: WithdrawRequested}
	err := db.Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&VendorProfile{}).Where("id = ? AND balance >= ?", vp.ID, req.Amount).
			UpdateColumn("balance", gorm.Expr("balance - ?", req.Amount))
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return fmt.Errorf("saldo tidak cukup")
		}
		return tx.Create(&w).Error
	})
	if err != nil {
		fail(c, 400, err.Error())
		return
	}
	c.JSON(201, w)
}

// ---------- admin ----------
func adminListVendors(c *gin.Context) {
	q := db.Order("id desc")
	if s := c.Query("status"); s != "" {
		q = q.Where("status = ?", s)
	}
	var vs []VendorProfile
	q.Find(&vs)
	c.JSON(200, vs)
}

func adminSetVendorStatus(c *gin.Context) {
	var req struct {
		Status string `json:"status" binding:"required,oneof=approved rejected pending"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err.Error())
		return
	}
	res := db.Model(&VendorProfile{}).Where("id = ?", c.Param("id")).Update("status", req.Status)
	if res.RowsAffected == 0 {
		fail(c, 404, "kantin tidak ditemukan")
		return
	}
	var vp VendorProfile
	db.First(&vp, c.Param("id"))
	c.JSON(200, vp)
}

func adminListWithdrawals(c *gin.Context) {
	q := db.Order("id desc")
	if s := c.Query("status"); s != "" {
		q = q.Where("status = ?", s)
	}
	var ws []Withdrawal
	q.Find(&ws)
	c.JSON(200, ws)
}

// PATCH /admin/withdrawals/:id {status: processed|rejected, note}. Ditolak = saldo dikembalikan.
func adminProcessWithdrawal(c *gin.Context) {
	var req struct {
		Status string `json:"status" binding:"required,oneof=processed rejected"`
		Note   string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err.Error())
		return
	}
	var w Withdrawal
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ? AND status = ?", c.Param("id"), WithdrawRequested).First(&w).Error; err != nil {
			return fmt.Errorf("pencairan tidak ditemukan atau sudah diproses")
		}
		now := time.Now()
		res := tx.Model(&w).Where("status = ?", WithdrawRequested).
			Updates(map[string]any{"status": req.Status, "admin_note": req.Note, "processed_at": now})
		if res.Error != nil || res.RowsAffected == 0 {
			return fmt.Errorf("pencairan sudah diproses")
		}
		if req.Status == WithdrawRejected {
			return tx.Model(&VendorProfile{}).Where("id = ?", w.VendorID).
				UpdateColumn("balance", gorm.Expr("balance + ?", w.Amount)).Error
		}
		return nil
	})
	if err != nil {
		fail(c, 400, err.Error())
		return
	}
	db.First(&w, w.ID)
	c.JSON(200, w)
}
