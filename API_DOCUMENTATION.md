# Dokumentasi API — Kantin Pre-Order Backend

Base URL: `http://localhost:8080/api` (lihat `API.md` untuk cara akses dari emulator/HP asli)

**Semua response sukses** mengikuti bentuk aslinya dari tiap handler (dijelaskan per endpoint di bawah).
**Semua response error** berbentuk sama: `{"error": "pesan error"}`.
**Header Authorization**: endpoint yang butuh login wajib menyertakan `Authorization: Bearer <token>`.
**Uang** selalu Rupiah dalam bentuk integer.

## Apa yang baru (revisi setelah wawancara pengguna)

| Masukan | Perubahan di backend |
|---|---|
| Menu perlu gambar | `Menu.image_url`, `VendorProfile.image_url`, upload foto `POST /vendor/upload`, data demo sudah bergambar (`/assets/food/*.svg`) |
| Search bar cari menu spesifik | `GET /menus?q=...&category=...` (cari nama/deskripsi menu & nama kantin) |
| Landing page langsung ada menu | `GET /menus` tanpa parameter mengembalikan semua menu + `store_name` + `image_url`, jadi beranda bisa langsung menampilkan menu |
| Keranjang per kantin & order beberapa kantin sekaligus | `POST /checkout` menerima beberapa keranjang (1 keranjang = 1 kantin), dibayar dengan **satu** QRIS |
| Bayar pakai QRIS langsung di app | Pembayaran pakai Xendit QR Codes; app menampilkan `GET /payments/:id/qr.png` |
| Tambah bahan (telur, bawang goreng, dll) / catatan | `Menu.addons` (tambahan berbayar) + `note` per item pesanan |
| Pilihan makan di tempat (masukan vendor) | `order_type`: `takeaway` / `dine_in` |
| Cara vendor daftar / join | `POST /vendors/register` (status `pending`) → admin verifikasi `PATCH /admin/vendors/:id/status` |
| Cara vendor mencairkan dana | `POST /vendor/withdrawals` → admin transfer & tandai `PATCH /admin/withdrawals/:id` |
| Memperjelas alur | `pickup_code` (kode ambil 4 huruf), `store_name` & `customer_name` di pesanan, kantin bisa Buka/Tutup (`is_open`) |

### Alur lengkap

**Mahasiswa**: lihat menu (`GET /menus`) → masukkan ke keranjang per kantin (disimpan di app) → `POST /checkout` → tampilkan QRIS (`GET /payments/:id/qr.png`) → polling `GET /payments/:id` sampai `paid` → polling `GET /orders` sampai `ready` → tunjukkan `pickup_code` di kantin.

**Kantin**: `POST /vendors/register` → tunggu verifikasi admin → isi menu + foto → terima pesanan (`GET /vendor/orders`) → `paid → preparing → ready → completed` → saldo bertambah otomatis saat dibayar → `POST /vendor/withdrawals` untuk cairkan ke rekening.

**Admin**: verifikasi kantin baru → transfer dana pencairan ke rekening kantin → tandai `processed` (atau `rejected`, saldo kembali ke kantin).

---

## 1. Auth & Akun

### `POST /api/register`
- **Akses**: publik — khusus mahasiswa
- **Input**: `name` ✅, `email` ✅ (format email), `password` ✅ (min 5)
- **Role hasil registrasi**: selalu `"student"`
- **Response** `201`: `{"user": {...}, "token": "eyJ..."}`
- **Error**: `400` validasi, `409` `"email sudah terdaftar"`

### `POST /api/login`
- **Akses**: publik — semua role (student, vendor, admin)
- **Input**: `email`, `password`
- **Response** `200`: `{"user": {"id":1,"name":"...","email":"...","role":"student|vendor|admin"}, "token":"eyJ..."}`
- Gunakan `user.role` untuk menentukan halaman yang dibuka (mahasiswa / kantin / admin).
- **Error**: `401` `"email atau password salah"`

### `GET /api/me`
- **Akses**: login — data akun sendiri

---

## 2. Publik (untuk dibrowsing pembeli, tanpa login)

### `GET /api/vendors`
- Hanya kantin berstatus `approved`, urut yang buka dulu.
- **Response** `200`:
  ```json
  [{"id":1,"store_name":"Warung Bu Sari","description":"Masakan rumahan...","image_url":"/assets/food/nasi-goreng.svg",
    "location":"Kantin Pusat, Gedung A lt. 1","is_open":true,"status":"approved","balance":0,"user_id":0}]
  ```
  Data rekening, telepon, dan saldo tidak ikut dikirim (selalu kosong/0 di endpoint publik).

### `GET /api/vendors/:id`
- Detail satu kantin (kolom sama seperti di atas). `404` bila tidak ada / belum diverifikasi.

### `GET /api/menus?vendor_id=&q=&category=`
- Semua parameter opsional. Tanpa parameter = semua menu tersedia dari semua kantin (cocok untuk beranda).
- `q`: cari di nama menu, deskripsi menu, dan nama kantin (tidak peka huruf besar/kecil).
- `category`: `nasi`, `mie`, `ayam`, `berkuah`, `sayur`, `snack`, `minuman` (bebas, diisi kantin).
- **Response** `200`:
  ```json
  [{"id":1,"vendor_id":1,"name":"Nasi Goreng Spesial","description":"Nasi goreng kecap...",
    "category":"nasi","image_url":"/assets/food/nasi-goreng.svg","price":15000,"stock":50,"is_available":true,
    "store_name":"Warung Bu Sari","vendor_open":true,
    "addons":[{"id":1,"menu_id":1,"name":"Telur ceplok","price":3000},
              {"id":2,"menu_id":1,"name":"Bawang goreng","price":1000}]}]
  ```
- `image_url` bisa berupa path relatif (`/assets/...`, `/uploads/...`) → gabungkan dengan host server, atau URL penuh (`https://...`).
- Tampilkan tombol "Tambah" nonaktif bila `vendor_open=false` atau `stock=0`.

### `POST /api/vendors/register` — pendaftaran kantin (mitra)
- **Akses**: publik
- **Input**:
  | Field | Wajib | Keterangan |
  |---|---|---|
  | name | ✅ | nama pemilik |
  | email | ✅ | dipakai untuk login |
  | password | ✅ | min 8 karakter |
  | store_name | ✅ | nama kantin |
  | location | ✅ | mis. "Kantin Teknik, Gedung C" |
  | phone | ✅ | nomor WhatsApp |
  | description | ❌ | |
  | bank_name, bank_account_no, bank_account_name | ✅ | rekening tujuan pencairan dana |
- **Response** `201`: `{"user": {...role:"vendor"}, "vendor": {..."status":"pending"}, "token":"eyJ..."}`
- Kantin `pending` sudah bisa login & menyiapkan menu, tapi **belum muncul** di app mahasiswa sampai admin menyetujui. Set env `VENDOR_AUTO_APPROVE=true` untuk langsung `approved` (praktis saat demo).
- **Error**: `400` validasi, `409` email sudah terdaftar

### `GET /api/health`
- `{"ok":true,"payment_mode":"mock|xendit","admin_fee":2000}`

---

## 3. Pemesanan & Pembayaran (role `student`)

### `POST /api/checkout` — keranjang per kantin, satu QRIS
- **Input**:
  ```json
  {"carts": [
    {"vendor_id": 1, "order_type": "dine_in", "pickup_time": "2026-10-15T12:00:00+07:00",
     "items": [{"menu_id": 1, "qty": 2, "addon_ids": [1, 2], "note": "tidak pedas"}]},
    {"vendor_id": 2,
     "items": [{"menu_id": 4, "qty": 1}]}
  ]}
  ```
  | Field | Wajib | Keterangan |
  |---|---|---|
  | carts | ✅ | 1–10 keranjang, satu kantin hanya boleh satu keranjang |
  | carts[].vendor_id | ✅ | |
  | carts[].items[].menu_id, qty | ✅ | qty min 1 |
  | carts[].items[].addon_ids | ❌ | id dari `menu.addons` |
  | carts[].items[].note | ❌ | catatan untuk penjual, maks 200 karakter |
  | carts[].order_type | ❌ | `takeaway` (default) atau `dine_in` |
  | carts[].pickup_time | ❌ | ISO8601; kosong = secepatnya |
- **Proses**: cek kantin buka & disetujui → cek & kurangi stok → hitung harga (harga menu + tambahan) × qty → 1 `Order` per kantin → 1 `Payment` QRIS untuk semuanya. Biaya layanan (`ADMIN_FEE`, default Rp2.000) dikenakan **sekali per checkout** (dicatat di pesanan pertama).
- **Response** `201`:
  ```json
  {"payment_mode":"mock",
   "payment":{"id":1,"external_id":"pay-94de69...","method":"qris","amount":53000,"status":"pending",
     "qr_string":"00020101...","expires_at":"2026-10-15T12:15:00+07:00",
     "orders":[
       {"id":1,"vendor_id":1,"payment_id":1,"store_name":"Warung Bu Sari","pickup_code":"ANAW","order_type":"dine_in",
        "subtotal":38000,"admin_fee":2000,"total":40000,"status":"pending_payment",
        "items":[{"menu_id":1,"name":"Nasi Goreng Spesial","image_url":"/assets/food/nasi-goreng.svg",
                  "price":15000,"addon_price":4000,"addons":"Telur ceplok, Bawang goreng","note":"tidak pedas","qty":2}]},
       {"id":2,"vendor_id":2,"store_name":"Mie & Bakso Pak Joko","pickup_code":"V9N7","subtotal":13000,"admin_fee":0,"total":13000,"...":"..."}]}}
  ```
  Satu pesanan = satu kantin, masing-masing punya `pickup_code` sendiri.
- **Error** `400`: `"menu {id} tidak tersedia di kantin ini"`, `"stok {nama} tidak cukup"`, `"tambahan tidak valid untuk {nama}"`, `"{kantin} sedang tutup"`, `"kantin tidak ditemukan"`, `"satu kantin hanya boleh satu keranjang per checkout"`, `"gagal membuat QRIS: ..."`

### `GET /api/payments/:id/qr.png`
- Gambar PNG 512×512 QRIS yang langsung discan dengan GoPay/OVO/DANA/ShopeePay/m-banking.
- Butuh header `Authorization`, jadi di app unduh sebagai bytes (mis. `Image.memory` di Flutter) — atau buat QR sendiri dari `payment.qr_string`.

### `GET /api/payments/:id`
- Polling tiap 3 detik di layar bayar. **Response** `200`: `{"payment": {..., "orders": [...]}, "payment_mode": "mock"}`
- `payment.status`: `pending` → `paid`, atau `expired` bila lewat `expires_at` (default 15 menit, env `PAYMENT_TTL_MINUTES`). Saat kedaluwarsa, stok dikembalikan otomatis.

### `POST /api/dev/payments/:id/simulate` — tombol "Bayar (simulasi)" untuk demo di aplikasi
- **Aktif** di mode mock **dan** saat `XENDIT_SECRET_KEY` adalah key test (`xnd_development_...`). Tidak ada (404) saat memakai key live.
- **Mode mock**: pembayaran langsung lunas, response `200` `{"payment": {..."status":"paid"}}`.
- **Mode test Xendit**: server meminta Xendit menyimulasikan pembayaran QR, lalu Xendit mengirim callback seperti pembayaran asli. Response `202` `{"payment": {..."status":"pending"}, "message": "..."}` — aplikasi **tetap polling** `GET /payments/:id` sampai `paid` (biasanya beberapa detik).
- QR di mode test Xendit berisi teks contoh (`some-random-qr-string`), jadi tidak bisa discan e-wallet sungguhan; pakai endpoint ini saat demo.
- **Error**: `400` `"pembayaran sudah paid|expired"`, `502` `"gagal simulasi pembayaran: ..."`.
- Rekomendasi di aplikasi: tampilkan tombol ini hanya bila `GET /health` → `payment_mode` = `mock`, atau beri label "Sandbox".

### `POST /api/dev/payments/:id/pay` (hanya mode mock)
- Simulasi QRIS berhasil dibayar → semua pesanan di dalamnya jadi `paid` + saldo masing-masing kantin bertambah.
- **Error** `400` `"pembayaran sudah paid|expired"`.

### `GET /api/orders`
- Riwayat pesanan sendiri (maks 100, terbaru dulu), lengkap dengan `items`, `pickup_code`, `store_name`, `payment_id`.
- Pesanan `pending_payment` bisa dilanjutkan bayar lewat `payment_id`.

### `GET /api/orders/:id`
- Detail satu pesanan. Boleh diakses pemilik, kantin terkait, atau admin.

### Kompatibilitas versi lama
- `POST /api/orders` (body satu kantin: `vendor_id`, `items`, `pickup_time`, `order_type`) masih bisa dipakai; response `{"order": {...}, "payment": {...}, "payment_mode": "..."}`.
- `POST /api/dev/pay/:order_id` masih ada (mode mock), melunasi seluruh pembayaran yang memuat pesanan itu.

Status pesanan: `pending_payment` → `paid` → `preparing` → `ready` → `completed` (atau `expired`).

---

## 4. Kantin (role `vendor`)

### `GET /api/vendor/profile`
- Profil lengkap termasuk `status` verifikasi (`pending`/`approved`/`rejected`), `balance`, `is_open`, dan data rekening.

### `PUT /api/vendor/profile` — update sebagian
- Semua field opsional, **hanya field yang dikirim yang diubah**: `store_name`, `description`, `image_url`, `location`, `phone`, `is_open`, `bank_name`, `bank_account_no`, `bank_account_name`.
- Tombol Buka/Tutup kantin: kirim `{"is_open": false}`. Saat tutup, menu tetap terlihat tapi tidak bisa dipesan.

### `POST /api/vendor/upload` — unggah foto
- `multipart/form-data`, field `file`; JPG/PNG/WEBP, maks 3 MB.
- **Response** `201`: `{"url": "/uploads/3f66f7...png"}` → kirim sebagai `image_url` menu atau profil.
- File disimpan di `UPLOAD_DIR` (default `./uploads`). Di Azure Container Apps, folder ini hilang saat container restart, jadi untuk produksi pasang Azure Files atau ganti ke Blob Storage.

### `GET /api/vendor/menus`
- Semua menu kantin sendiri (termasuk yang tidak tersedia), lengkap dengan `addons`.

### `POST /api/vendor/menus`
  | Field | Wajib | Keterangan |
  |---|---|---|
  | name | ✅ | |
  | price | ✅ | min 1 |
  | stock | ❌ | default 0 |
  | description, category, image_url | ❌ | |
  | is_available | ❌ | default `true` |
  | addons | ❌ | maks 15, `[{"name":"Telur ceplok","price":3000}]` (price boleh 0, mis. "Less ice") |

### `PUT /api/vendor/menus/:id`
- Body sama seperti create (bukan update sebagian; `name` & `price` wajib).
- `addons`: kalau dikirim, daftar tambahan **diganti seluruhnya**; kalau tidak dikirim, tambahan lama tetap.

### `DELETE /api/vendor/menus/:id`
- Hapus menu beserta tambahannya. Riwayat pesanan tidak terpengaruh (nama & harga sudah disalin ke pesanan).

### `GET /api/vendor/orders?status=`
- Pesanan yang sudah dibayar (tanpa `pending_payment`/`expired`), terbaru dulu, maks 200.
- Tampilkan `pickup_code`, `customer_name`, `order_type`, `pickup_time`, serta `items[].addons` dan `items[].note` agar penjual tidak salah masak.

### `PATCH /api/vendor/orders/:id/status`
- `{"status":"preparing"}` — harus berurutan `paid → preparing → ready → completed`.

### Pencairan dana
#### `GET /api/vendor/withdrawals`
- `{"balance": 38000, "min_amount": 10000, "withdrawals": [{"id":1,"amount":20000,"bank_name":"BCA","bank_account_no":"...","status":"requested|processed|rejected","admin_note":"","created_at":"...","processed_at":null}]}`

#### `POST /api/vendor/withdrawals`
- `{"amount": 20000}` — minimal Rp10.000, saldo langsung dipotong, ditransfer ke rekening di profil.
- **Error** `400`: `"minimal pencairan Rp10000"`, `"saldo tidak cukup"`, `"kantin belum diverifikasi admin"`, `"lengkapi data rekening bank di profil kantin terlebih dahulu"`.

Saldo (`balance`) bertambah sebesar `subtotal` pesanan saat pembayaran berhasil (biaya layanan menjadi pendapatan platform).

---

## 5. Admin (role `admin`)

| Method | Path | Keterangan |
|---|---|---|
| POST | /admin/vendors | Buat akun kantin langsung (`name`, `email`, `password` min 8, `store_name`), status langsung `approved` |
| GET | /admin/vendors?status=pending | Daftar kantin (lengkap dengan telepon & rekening) |
| PATCH | /admin/vendors/:id/status | `{"status":"approved"}` / `"rejected"` / `"pending"` |
| GET | /admin/withdrawals?status=requested | Daftar permintaan pencairan |
| PATCH | /admin/withdrawals/:id | `{"status":"processed","note":"Transfer BCA 15/10"}` setelah admin mentransfer manual; `{"status":"rejected","note":"..."}` mengembalikan saldo ke kantin |

---

## 6. Webhook (dipanggil Xendit, bukan mobile app)

Keduanya divalidasi lewat header `x-callback-token` = `XENDIT_CALLBACK_TOKEN`.

### `POST /api/webhooks/xendit/qris`
- Daftarkan URL ini di dashboard Xendit untuk event **QR Code payment**.
- Body: `{"event":"qr.payment","data":{"reference_id":"pay-...","status":"SUCCEEDED",...}}` → pembayaran & semua pesanannya jadi `paid`.

### `POST /api/webhooks/xendit/invoice`
- Untuk pesanan lama yang dibuat dengan Invoice: `{"external_id":"...","status":"PAID|SETTLED|EXPIRED"}`.

---

## 7. Data demo

Jalankan dengan `SEED_DEMO=true` di database kosong untuk mengisi 4 kantin + 13 menu bergambar:

| Role | Email | Password |
|---|---|---|
| Mahasiswa | mahasiswa@demo.id | demo1234 |
| Kantin | busari@demo.id, pakjoko@demo.id, kopikita@demo.id, sehat@demo.id | demo1234 |
| Admin | admin@demo.id (hanya bila `ADMIN_EMAIL` tidak di-set) | admin1234 |

Gambar contoh ada di `assets/food/` dan disajikan di `/assets/food/<nama>.svg`.

---

## Catatan tentang pesan error validasi (400)

Untuk error `400` yang berasal dari `ShouldBindJSON` gagal (field wajib kosong, format salah, dll), pesannya pesan mentah dari library validator Go, contoh:
```json
{"error": "Key: 'req.Email' Error:Field validation for 'Email' failed on the 'required' tag"}
```
Pesan ini **tidak cocok** ditampilkan langsung ke user — di sisi mobile app cukup tampilkan "Periksa kembali data yang diisi" untuk error jenis ini, kecuali error yang sudah berupa kalimat jelas (seperti `"stok ... tidak cukup"`, `"email sudah terdaftar"`, dll).
