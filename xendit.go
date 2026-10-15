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

// Lama QRIS berlaku sebelum pesanan dibatalkan otomatis.
func paymentTTL() time.Duration {
	n, err := strconv.Atoi(getenv("PAYMENT_TTL_MINUTES", "15"))
	if err != nil || n <= 0 {
		n = 15
	}
	return time.Duration(n) * time.Minute
}

func xenditPost(url string, body any, headers map[string]string, out any) error {
	buf, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", url, bytes.NewReader(buf))
	req.SetBasicAuth(os.Getenv("XENDIT_SECRET_KEY"), "")
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("xendit %d: %s", resp.StatusCode, string(raw))
	}
	return json.Unmarshal(raw, out)
}

// Membuat QRIS dinamis. Di mode mock, QR berisi string contoh yang tidak bisa dibayar sungguhan.
func createQRIS(externalID string, amount int64, expiresAt time.Time) (id, qrString string, err error) {
	if mockMode() {
		return "mock-" + externalID, fmt.Sprintf("00020101021226MOCKQRIS-KANTIN|%s|%d|DEMO-TIDAK-BISA-DIBAYAR", externalID, amount), nil
	}
	var r struct {
		ID       string `json:"id"`
		QRString string `json:"qr_string"`
	}
	err = xenditPost("https://api.xendit.co/qr_codes", map[string]any{
		"reference_id": externalID, "type": "DYNAMIC", "currency": "IDR",
		"amount": amount, "expires_at": expiresAt.UTC().Format(time.RFC3339),
	}, map[string]string{"api-version": "2022-07-31"}, &r)
	return r.ID, r.QRString, err
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
