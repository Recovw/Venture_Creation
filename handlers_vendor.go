package main

import (
	"fmt"

	"github.com/gin-gonic/gin"
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
func listVendors(c *gin.Context) {
	var vs []VendorProfile
	db.Select("id", "store_name", "description").Find(&vs)
	c.JSON(200, vs)
}

func listMenus(c *gin.Context) {
	q := db.Where("is_available = ?", true)
	if v := c.Query("vendor_id"); v != "" {
		q = q.Where("vendor_id = ?", v)
	}
	var ms []Menu
	q.Find(&ms)
	c.JSON(200, ms)
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
		StoreName   string `json:"store_name"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err.Error())
		return
	}
	if req.StoreName != "" {
		vp.StoreName = req.StoreName
	}
	vp.Description = req.Description
	db.Save(&vp)
	c.JSON(200, vp)
}

type menuReq struct {
	Name        string `json:"name" binding:"required"`
	Price       int64  `json:"price" binding:"required,min=1"`
	Stock       int    `json:"stock" binding:"min=0"`
	IsAvailable *bool  `json:"is_available"`
}

func vendorListMenus(c *gin.Context) {
	vp, ok := myVendor(c)
	if !ok {
		return
	}
	var ms []Menu
	db.Where("vendor_id = ?", vp.ID).Find(&ms)
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
	m := Menu{VendorID: vp.ID, Name: req.Name, Price: req.Price, Stock: req.Stock, IsAvailable: true}
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
	m.Name, m.Price, m.Stock = req.Name, req.Price, req.Stock
	if req.IsAvailable != nil {
		m.IsAvailable = *req.IsAvailable
	}
	db.Save(&m)
	c.JSON(200, m)
}

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
	q.Order("id desc").Find(&orders)
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
