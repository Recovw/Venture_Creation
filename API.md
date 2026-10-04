# API MVP Kantin Pre-Order (untuk aplikasi mobile)

Base URL lokal: `http://localhost:8080/api`
Android emulator: `http://10.0.2.2:8080/api` | HP asli: pakai IP laptop di jaringan Wi-Fi yang sama.

Semua endpoint bertanda "login" butuh header `Authorization: Bearer <token>`.
Error selalu berbentuk `{"error": "pesan"}`. Uang dalam Rupiah (integer).

## Jalankan
```bash
cp .env.example .env     # isi JWT_SECRET, ADMIN_*; biarkan XENDIT_SECRET_KEY kosong = mode mock
go mod tidy
go run .
```
Cek: `GET /api/health` -> `{"ok":true,"payment_mode":"mock"}`

## Alur di aplikasi mobile
1. Mahasiswa daftar/login -> simpan `token`.
2. Beranda: `GET /vendors`, lalu `GET /menus?vendor_id=1`.
3. Checkout: `POST /orders` -> dapat `order` dan `payment_mode`.
   - `xendit`: buka `order.payment_url` (WebView/browser). Setelah bayar, status berubah otomatis lewat webhook.
   - `mock`: panggil `POST /dev/pay/{order.id}` untuk mensimulasikan pembayaran berhasil.
4. Layar status: panggil `GET /orders/{id}` tiap 3-5 detik sampai `status` = `ready`.
5. Kantin: `GET /vendor/orders`, lalu `PATCH /vendor/orders/{id}/status`.

Status pesanan: `pending_payment` -> `paid` -> `preparing` -> `ready` -> `completed` (atau `expired`).

## Endpoint

| Method | Path | Akses | Keterangan |
|---|---|---|---|
| GET | /health | publik | Cek server dan mode pembayaran |
| POST | /register | publik | Daftar mahasiswa (role selalu student) |
| POST | /login | publik | Login semua role |
| GET | /vendors | publik | Daftar kantin |
| GET | /menus?vendor_id= | publik | Menu yang tersedia |
| GET | /me | login | Data akun sendiri |
| POST | /orders | student | Buat pesanan + pembayaran |
| GET | /orders | student | Riwayat pesanan sendiri |
| GET | /orders/:id | login | Detail/status (pemilik, kantin terkait, admin) |
| POST | /dev/pay/:id | student | Hanya mode mock: simulasi sudah bayar |
| GET, PUT | /vendor/profile | vendor | Profil kantin + saldo pendapatan |
| GET, POST | /vendor/menus | vendor | Lihat / tambah menu |
| PUT | /vendor/menus/:id | vendor | Ubah menu, harga, stok |
| GET | /vendor/orders?status= | vendor | Pesanan masuk (sudah dibayar) |
| PATCH | /vendor/orders/:id/status | vendor | Maju ke status berikutnya |
| POST | /admin/vendors | admin | Buat akun kantin |
| POST | /webhooks/xendit/invoice | Xendit | Callback pembayaran (token di header) |

## Contoh request/response

**POST /register**
```json
{"name":"Rina","email":"rina@kampus.ac.id","password":"rahasia123"}
```
-> `201 {"user":{"id":2,"name":"Rina","role":"student",...},"token":"eyJ..."}`

**POST /login** `{"email":"...","password":"..."}` -> `200 {"user":{...},"token":"eyJ..."}`

**POST /orders** (login sebagai student)
```json
{"vendor_id":1,"items":[{"menu_id":11,"qty":2}],"pickup_time":"2026-10-05T12:00:00+07:00"}
```
`pickup_time` opsional. Satu pesanan hanya untuk satu kantin.
-> `201`
```json
{"payment_mode":"mock",
 "order":{"id":1,"vendor_id":1,"subtotal":30000,"admin_fee":2000,"total":32000,
          "status":"pending_payment","payment_url":"",
          "items":[{"menu_id":11,"name":"Nasi Ayam Geprek","price":15000,"qty":2}]}}
```
Error umum: `400 stok ... tidak cukup`, `400 menu ... tidak tersedia di kantin ini`.

**PATCH /vendor/orders/:id/status** `{"status":"preparing"}`
Hanya boleh berurutan: paid -> preparing -> ready -> completed.

## Belum ada (tahap berikutnya)
Pencairan pendapatan via Xendit Disbursement, ledger saldo, dashboard admin, notifikasi push, hapus menu, upload foto menu, refresh token.
