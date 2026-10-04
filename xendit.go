package main

import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

var httpClient = &http.Client{Timeout: 20 * time.Second}

// Mode mock aktif bila XENDIT_SECRET_KEY kosong: tidak memanggil Xendit sama sekali.
func mockMode() bool { return os.Getenv("XENDIT_SECRET_KEY") == "" }

func createInvoice(externalID string, amount int64, payerEmail, desc string) (id, url string, err error) {
	if mockMode() {
		return "mock-" + externalID, "", nil
	}
	buf, _ := json.Marshal(map[string]any{
		"external_id": externalID, "amount": amount, "payer_email": payerEmail,
		"description": desc, "currency": "IDR", "invoice_duration": 900,
	})
	req, _ := http.NewRequest("POST", "https://api.xendit.co/v2/invoices", bytes.NewReader(buf))
	req.SetBasicAuth(os.Getenv("XENDIT_SECRET_KEY"), "")
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("xendit %d: %s", resp.StatusCode, string(out))
	}
	var r struct {
		ID         string `json:"id"`
		InvoiceURL string `json:"invoice_url"`
	}
	if err := json.Unmarshal(out, &r); err != nil {
		return "", "", err
	}
	return r.ID, r.InvoiceURL, nil
}

func callbackTokenOK(c *gin.Context) bool {
	want := os.Getenv("XENDIT_CALLBACK_TOKEN")
	return want != "" && subtle.ConstantTimeCompare([]byte(c.GetHeader("x-callback-token")), []byte(want)) == 1
}

func adminFee() int64 {
	n, err := strconv.ParseInt(getenv("ADMIN_FEE", "2000"), 10, 64)
	if err != nil {
		return 2000
	}
	return n
}
