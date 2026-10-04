# Dokumentasi API — Kantin Pre-Order Backend

Base URL: `http://localhost:8080/api` (lihat `API.md` untuk cara akses dari emulator/HP asli)

**Semua response sukses** mengikuti bentuk aslinya dari tiap handler (dijelaskan per endpoint di bawah).
**Semua response error** berbentuk sama: `{"error": "pesan error"}`.
**Header Authorization**: endpoint yang butuh login wajib menyertakan `Authorization: Bearer <token>`.

---

## 1. Auth & Akun

### `POST /api/register`
- **Akses**: publik
- **Input (JSON body)**:
  | Field | Tipe | Wajib | Keterangan |
  |---|---|---|---|
  | name | string | ✅ | |
  | email | string | ✅ | harus format email valid |
  | password | string | ✅ | minimal 5 karakter |
- **Role hasil registrasi**: selalu `"student"` (tidak bisa diubah lewat input)
- **Response sukses** — `201`:
  ```json
  {
    "user": {"id":2,"name":"Rina","email":"rina@kampus.ac.id","role":"student","created_at":"..."},
    "token": "eyJ..."
  }
  ```
- **Response error**:
  - `400` — body tidak valid (pesan dari validator, contoh: field email/password kosong)
  - `409` — `{"error":"email sudah terdaftar"}`

---

### `POST /api/login`
- **Akses**: publik
- **Input (JSON body)**:
  | Field | Tipe | Wajib |
  |---|---|---|
  | email | string | ✅ |
  | password | string | ✅ |
- **Response sukses** — `200`:
  ```json
  {"user": {"id":1,"name":"...","email":"...","role":"student","created_at":"..."}, "token":"eyJ..."}
  ```
- **Response error**:
  - `400` — body tidak valid
  - `401` — `{"error":"email atau password salah"}`

---

### `GET /api/me`
- **Akses**: login (semua role)
- **Input**: tidak ada (data diambil dari token)
- **Response sukses** — `200`: object `User` (`id, name, email, role, created_at`)
- **Response error**: `404` — `{"error":"user tidak ditemukan"}`

---

## 2. Vendor (Publik — untuk dibrowsing pembeli)

### `GET /api/vendors`
- **Akses**: publik
- **Input**: tidak ada
- **Response sukses** — `200`: array `VendorProfile`
  ```json
  [{"id":1,"user_id":0,"store_name":"Kantin Bu Sari","description":"...","balance":0}]
  ```
  ⚠️ **Perhatian**: query di kode ini (`db.Select("id","store_name","description")`) **tidak ikut mengambil** `user_id` dan `balance` dari database — jadi dua field itu akan **selalu muncul sebagai `0`** di response ini (bukan nilai asli). Ini aman untuk endpoint publik (vendor tidak perlu tahu saldo kantin lain), tapi kalau nanti butuh `user_id` di response ini juga, field itu perlu ditambahkan ke `.Select(...)`.

### `GET /api/menus?vendor_id=`
- **Akses**: publik
- **Input (query param)**: `vendor_id` (opsional — kalau dikosongkan, ambil semua menu dari semua vendor)
- **Filter otomatis**: hanya menu dengan `is_available = true`
- **Response sukses** — `200`: array `Menu`
  ```json
  [{"id":11,"vendor_id":1,"name":"Nasi Ayam Geprek","price":15000,"stock":18,"is_available":true}]
  ```

---

## 3. Order (Mahasiswa/Student)

### `POST /api/orders`
- **Akses**: login, role `student`
- **Input (JSON body)**:
  | Field | Tipe | Wajib | Keterangan |
  |---|---|---|---|
  | vendor_id | uint | ✅ | satu order hanya untuk satu vendor |
  | items | array | ✅ | minimal 1 item |
  | items[].menu_id | uint | ✅ | |
  | items[].qty | int | ✅ | minimal 1 |
  | pickup_time | datetime (ISO8601) | ❌ opsional | |
- **Proses internal**: cek stok & ketersediaan tiap menu → kurangi stok → hitung `subtotal + admin_fee = total` → simpan order status `pending_payment` → buat invoice (Xendit asli atau mock)
- **Response sukses** — `201`:
  ```json
  {
    "payment_mode": "mock",
    "order": {
      "id":1,"vendor_id":1,"subtotal":30000,"admin_fee":2000,"total":32000,
      "status":"pending_payment","payment_url":"",
      "items":[{"menu_id":11,"name":"Nasi Ayam Geprek","price":15000,"qty":2}]
    }
  }
  ```
  Kalau `payment_mode` = `"xendit"` (bukan mock), field `order.payment_url` akan **berisi link** untuk dibuka mahasiswa menyelesaikan pembayaran.
- **Response error**:
  - `400` — body tidak valid, atau pesan spesifik: `"menu {id} tidak tersedia di kantin ini"` / `"stok {nama} tidak cukup"`
  - `404` — `{"error":"kantin tidak ditemukan"}`
  - `502` — `{"error":"gagal membuat invoice: ..."}` (gagal hubungi Xendit)

---

### `GET /api/orders`
- **Akses**: login, role `student`
- **Input**: tidak ada
- **Response sukses** — `200`: array `Order` (lengkap dengan `items`), milik student yang login, urut terbaru dulu

---

### `GET /api/orders/:id`
- **Akses**: login (semua role, tapi dicek manual di kode)
  - Boleh akses kalau: `role=admin`, ATAU `role=student` dan dia pemilik order, ATAU `role=vendor` dan order itu milik kantinnya
- **Input**: `id` di URL path
- **Response sukses** — `200`: object `Order` lengkap dengan `items`
- **Response error**: `404` — order tidak ditemukan; `403` — `{"error":"akses ditolak"}`

---

### `POST /api/dev/pay/:id`
- **Akses**: login, role `student` — **hanya aktif kalau `XENDIT_SECRET_KEY` kosong (mode mock)**
- **Input**: `id` di URL path, tidak ada body
- **Fungsi**: simulasi pembayaran berhasil tanpa Xendit asli — langsung ubah status order jadi `paid` + tambah saldo vendor
- **Response sukses** — `200`: object `Order` yang sudah terupdate (status `paid`)
- **Response error**: `404` — order tidak ditemukan / bukan milik student ini

---

## 4. Vendor (Kantin — butuh login sebagai vendor)

### `GET /api/vendor/profile`
- **Akses**: login, role `vendor`
- **Response sukses** — `200`: object `VendorProfile` **lengkap** (termasuk `balance` asli, beda dengan endpoint publik di atas)
- **Response error**: `404` — `{"error":"profil kantin tidak ditemukan"}`

### `PUT /api/vendor/profile`
- **Akses**: login, role `vendor`
- **Input (JSON body)**:
  | Field | Tipe | Wajib |
  |---|---|---|
  | store_name | string | ❌ opsional |
  | description | string | ❌ opsional |
- ⚠️ **Perhatian**: `store_name` hanya diubah **kalau diisi** (kosong = tetap nama lama). Tapi `description` **selalu ditimpa** dengan apa yang dikirim — kalau field ini dikirim kosong/tidak disertakan sama sekali, deskripsi lama akan **terhapus jadi kosong**. Pastikan Alvin selalu kirim ulang deskripsi lama kalau cuma mau update `store_name` saja.
- **Response sukses** — `200`: object `VendorProfile` yang sudah terupdate

### `GET /api/vendor/menus`
- **Akses**: login, role `vendor`
- **Response sukses** — `200`: array `Menu` milik vendor yang login (termasuk yang `is_available:false`)

### `POST /api/vendor/menus`
- **Akses**: login, role `vendor`
- **Input (JSON body)**:
  | Field | Tipe | Wajib | Keterangan |
  |---|---|---|---|
  | name | string | ✅ | |
  | price | int64 | ✅ | minimal 1 |
  | stock | int | ❌ | minimal 0, default 0 kalau tidak diisi |
  | is_available | bool | ❌ | default `true` kalau tidak dikirim sama sekali |
- **Response sukses** — `201`: object `Menu` yang baru dibuat

### `PUT /api/vendor/menus/:id`
- **Akses**: login, role `vendor` (hanya bisa edit menu miliknya sendiri)
- **Input (JSON body)**: **sama seperti create** (`name`, `price` wajib dikirim ulang; ini bukan partial update — field yang tidak dikirim akan ter-reset ke kosong/0, KECUALI `is_available` yang tetap opsional)
- **Response sukses** — `200`: object `Menu` yang sudah terupdate
- **Response error**: `404` — `{"error":"menu tidak ditemukan"}` (termasuk kalau menu itu milik vendor lain)

### `GET /api/vendor/orders?status=`
- **Akses**: login, role `vendor`
- **Input (query param)**: `status` (opsional filter, contoh: `?status=paid`)
- **Default (tanpa filter)**: semua order milik kantin ini **KECUALI** yang masih `pending_payment` atau `expired`
- **Response sukses** — `200`: array `Order` (dengan `items`), urut terbaru dulu

### `PATCH /api/vendor/orders/:id/status`
- **Akses**: login, role `vendor`
- **Input (JSON body)**: `{"status": "preparing"}`
- **Aturan transisi** (harus berurutan, tidak bisa lompat):
  `paid → preparing → ready → completed`
- **Response sukses** — `200`: object `Order` yang sudah terupdate
- **Response error**:
  - `404` — order tidak ditemukan / bukan milik kantin ini
  - `400` — `{"error":"transisi {status_lama} -> {status_baru} tidak valid"}`

---

## 5. Admin

### `POST /api/admin/vendors`
- **Akses**: login, role `admin`
- **Input (JSON body)**:
  | Field | Tipe | Wajib | Keterangan |
  |---|---|---|---|
  | name | string | ✅ | nama pemilik/akun |
  | email | string | ✅ | format email valid |
  | password | string | ✅ | minimal 8 karakter |
  | store_name | string | ✅ | nama kantin |
- **Response sukses** — `201`:
  ```json
  {"user": {"id":5,"name":"...","email":"...","role":"vendor","created_at":"..."},
   "vendor": {"id":3,"user_id":5,"store_name":"...","description":"","balance":0}}
  ```
- **Response error**: `400` — validasi; `409` — `{"error":"email sudah terdaftar"}`

---

## 6. Webhook (dipanggil Xendit, bukan mobile app)

### `POST /api/webhooks/xendit/invoice`
- **Akses**: khusus Xendit — divalidasi lewat header `x-callback-token` (harus cocok dengan `XENDIT_CALLBACK_TOKEN` di `.env`)
- **Input (JSON body dari Xendit)**: `{"external_id": "...", "status": "PAID" | "SETTLED" | "EXPIRED"}`
- **Response sukses** — `200`: `{"ok": true}`
- **Response error**: `401` — token callback salah; `400` — body tidak valid; `500` — gagal update status

---

## Catatan tentang pesan error validasi (400)

Untuk error `400` yang berasal dari `ShouldBindJSON` gagal (field wajib kosong, format salah, dll), pesannya **bukan** kalimat rapi buatan sendiri — itu pesan mentah dari library validator Go, contoh:
```json
{"error": "Key: 'req.Email' Error:Field validation for 'Email' failed on the 'required' tag"}
```
Alvin perlu tahu ini **tidak cocok** ditampilkan langsung ke user — sebaiknya di sisi mobile app cukup tampilkan pesan generik seperti "Periksa kembali data yang diisi" untuk error `400` jenis ini, kecuali untuk error spesifik yang memang sudah berupa kalimat jelas (seperti `"stok ... tidak cukup"`, `"email sudah terdaftar"`, dll).
