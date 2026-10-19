package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Zona waktu kampus; waktu tanpa zona dianggap WIB.
var campusTZ = func() *time.Location {
	if loc, err := time.LoadLocation(getenv("APP_TIMEZONE", "Asia/Jakarta")); err == nil {
		return loc
	}
	return time.FixedZone("WIB", 7*3600)
}()

// flexTime menerima beberapa format waktu dari aplikasi mobile:
// RFC3339 ("2026-10-19T12:00:00+07:00", "...Z"), tanpa zona ("2026-10-19T12:00:00.000",
// keluaran DateTime.toIso8601String() Flutter), dengan spasi ("2026-10-19 12:00"),
// atau jam saja ("12:30" = hari ini). String kosong / null = secepatnya.
type flexTime struct{ t *time.Time }

var flexLayouts = []string{
	"2006-01-02T15:04:05.999999999",
	"2006-01-02T15:04:05",
	"2006-01-02T15:04",
	"2006-01-02 15:04:05.999999999",
	"2006-01-02 15:04:05",
	"2006-01-02 15:04",
}

func (f *flexTime) UnmarshalJSON(b []byte) error {
	var s *string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("pickup_time harus berupa teks, contoh \"2026-10-19T12:00:00+07:00\" atau \"12:00\"")
	}
	if s == nil || strings.TrimSpace(*s) == "" {
		f.t = nil
		return nil
	}
	v := strings.TrimSpace(*s)
	if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
		f.t = &t
		return nil
	}
	for _, layout := range flexLayouts {
		if t, err := time.ParseInLocation(layout, v, campusTZ); err == nil {
			f.t = &t
			return nil
		}
	}
	for _, layout := range []string{"15:04", "15:04:05"} {
		if hm, err := time.Parse(layout, v); err == nil {
			now := time.Now().In(campusTZ)
			t := time.Date(now.Year(), now.Month(), now.Day(), hm.Hour(), hm.Minute(), hm.Second(), 0, campusTZ)
			f.t = &t
			return nil
		}
	}
	return fmt.Errorf("format pickup_time tidak dikenali: %q (contoh: \"2026-10-19T12:00:00+07:00\" atau \"12:00\")", v)
}

func (f *flexTime) Time() *time.Time {
	if f == nil {
		return nil
	}
	return f.t
}
