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

Isi data contoh (4 kantin, 13 menu bergambar): tambahkan `SEED_DEMO=true` di `.env` (boleh di database yang sudah berisi data, akun yang sudah ada dilewati).

Env tambahan: `ADMIN_FEE` (default 2000), `MAX_ORDER_AMOUNT` (default 10000000 = batas QRIS), `PAYMENT_TTL_MINUTES` (default 15), `VENDOR_AUTO_APPROVE` (`true` = kantin baru langsung aktif), `UPLOAD_DIR` (default `./uploads`), `XENDIT_CALLBACK_TOKEN`.

## Alur di aplikasi mobile
1. Mahasiswa daftar/login -> simpan `token`. Arahkan halaman sesuai `user.role` (student / vendor / admin).
2. Beranda: `GET /home` (menu terlaris bergambar + daftar kantin). Search bar: `GET /menus?q=geprek`.
3. Keranjang disimpan di app, **dipisah per kantin**. Detail menu menampilkan `addons` (telur, bawang goreng, dll) + kolom catatan.
4. Checkout: `POST /checkout` (satu atau beberapa kantin sekaligus) -> dapat `payment`.
5. Layar bayar: tampilkan gambar `GET /payments/{id}/qr.png`, hitung mundur ke `expires_at`, polling `GET /payments/{id}` tiap 3 detik.
   - Demo (mode mock atau key test Xendit): tombol "Bayar (simulasi)" -> `POST /dev/payments/{id}/simulate`, lalu lanjut polling sampai `paid`.
6. Layar pesanan: `GET /orders` tiap 5 detik sampai `ready`, tunjukkan `pickup_code` ke kantin.
7. Kantin: daftar lewat `POST /vendors/register`, kelola menu + foto, `GET /vendor/orders`, `PATCH /vendor/orders/{id}/status`, cairkan saldo `POST /vendor/withdrawals`.
8. Admin: verifikasi kantin & proses pencairan dana.

Status pesanan: `pending_payment` -> `paid` -> `preparing` -> `ready` -> `completed` (atau `expired`).

## Endpoint

| Method | Path | Akses | Keterangan |
|---|---|---|---|
| GET | /health | publik | Cek server, mode pembayaran, biaya layanan |
| POST | /register | publik | Daftar mahasiswa (role selalu student) |
| POST | /login | publik | Login semua role |
| POST | /vendors/register | publik | Pendaftaran kantin baru (menunggu verifikasi admin) |
| GET | /vendors | publik | Daftar kantin terverifikasi |
| GET | /vendors/:id | publik | Detail kantin |
| GET | /home?limit=10 | publik | Beranda: menu terlaris + daftar kantin |
| GET | /menus?vendor_id=&q=&category= | publik | Menu tersedia + gambar + tambahan; bisa dicari |
| GET | /me | login | Data akun sendiri |
| POST | /checkout | student | Checkout beberapa keranjang kantin, satu QRIS |
| POST | /orders | student | (lama) Checkout satu kantin |
| GET | /orders | student | Riwayat pesanan sendiri |
| GET | /orders/:id | login | Detail/status (pemilik, kantin terkait, admin) |
| GET | /payments/:id | student | Status pembayaran + pesanan di dalamnya |
| GET | /payments/:id/qr.png | student | Gambar QRIS |
| POST | /dev/payments/:id/simulate | student | Tombol "Bayar (simulasi)" untuk demo: mode mock dan key test Xendit |
| POST | /dev/payments/:id/pay | student | Hanya mode mock: simulasi QRIS dibayar |
| GET, PUT | /vendor/profile | vendor | Profil, status verifikasi, saldo, rekening, buka/tutup |
| POST | /vendor/upload | vendor | Upload foto (multipart `file`) |
| GET, POST | /vendor/menus | vendor | Lihat / tambah menu (+ foto, kategori, tambahan) |
| PUT, DELETE | /vendor/menus/:id | vendor | Ubah / hapus menu |
| GET | /vendor/orders?status= | vendor | Pesanan masuk (sudah dibayar) |
| PATCH | /vendor/orders/:id/status | vendor | Maju ke status berikutnya |
| GET, POST | /vendor/withdrawals | vendor | Riwayat / ajukan pencairan dana |
| POST | /admin/vendors | admin | Buat akun kantin langsung |
| GET | /admin/vendors?status= | admin | Daftar kantin untuk diverifikasi |
| PATCH | /admin/vendors/:id/status | admin | Setujui / tolak kantin |
| GET | /admin/withdrawals?status= | admin | Daftar pencairan dana |
| PATCH | /admin/withdrawals/:id | admin | Tandai sudah ditransfer / tolak |
| POST | /webhooks/xendit/qris | Xendit | Callback pembayaran QRIS |
| POST | /webhooks/xendit/invoice | Xendit | Callback invoice (pesanan lama) |

Detail request/response setiap endpoint ada di `API_DOCUMENTATION.md`; panduan layar per layar untuk tim mobile ada di `PANDUAN_MOBILE.md`.

## Belum ada (tahap berikutnya)
Pencairan otomatis via Xendit Disbursement (sekarang admin transfer manual), penyimpanan foto di Blob Storage, notifikasi push, refresh token.
