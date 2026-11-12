# Panduan Kode dan Deploy PreEat

Dokumen ini menjelaskan alur kode backend dan cara men-deploy-nya ke Azure, untuk pengingat tim.

## Bagian 1: Alur kode

### 1.1 Saat server dinyalakan (`main.go`)

```
go run .  (atau container di Azure)
 1. Baca .env / environment variable      → godotenv.Load()
 2. Wajib ada JWT_SECRET                   → kalau kosong, server berhenti
 3. Sambung ke PostgreSQL                  → connectDB() memakai DB_DSN
 4. Buat / perbarui tabel otomatis         → db.AutoMigrate(...)
 5. Buat admin pertama                     → seedAdmin() dari ADMIN_EMAIL/ADMIN_PASSWORD
 6. Data demo (kalau SEED_DEMO=true)       → seedDemo()
 7. Jalankan proses latar                  → go expireStalePayments()  (tiap 1 menit)
 8. Pasang CORS, daftarkan semua route     → r.Use(cors()), api.GET/POST...
 9. Mulai melayani di port 8080            → r.Run(":8080")
```

### 1.2 Perjalanan setiap request

```
Request HTTP
  → cors()            izinkan website memanggil API
  → authRequired()    cek header "Authorization: Bearer <token>"  (401 kalau gagal)
  → roleRequired()    cek peran: student / vendor / admin          (403 kalau salah)
  → handler           fungsi yang mengerjakan (mis. checkout)
  → database          baca/tulis lewat GORM
  → response JSON     sukses: data; gagal: {"error": "pesan"}
```

Route publik (tanpa login): `/health`, `/register`, `/login`, `/vendors/register`, `/vendors`, `/menus`, `/home`, webhook Xendit.

### 1.3 Isi setiap file

| File | Isi |
|---|---|
| `main.go` | Langkah 1.1 dan daftar semua route |
| `models.go` | Struct = tabel: User, VendorProfile, Menu, MenuAddon, Payment, Order, OrderItem, Withdrawal; konstanta status; `pickupCode()` |
| `auth.go` | `register`, `login`, `newToken` (JWT 72 jam), `hashPassword` (bcrypt), `authRequired`, `roleRequired` |
| `handlers_order.go` | `checkout` → `placeOrders` → `buildItem`; `getPayment`, `paymentQR`; `markPaid`, `expirePayment`, `lockPendingOrders`; `expireStalePayments`; `simulatePayment`; `xenditQRWebhook` |
| `handlers_vendor.go` | `home`, `listMenus`, `listVendors`; `vendorRegister`; profil, menu, upload foto; `vendorOrders`, `vendorUpdateOrderStatus`; pencairan dana; fungsi admin |
| `xendit.go` | `createQRIS`, `simulateQRIS`, `callbackTokenOK`, `maxOrderAmount`, `adminFee`, `rupiah` |
| `flextime.go` | Membaca format jam ambil (`pickup_time`) |
| `cors.go` | Izin CORS untuk frontend web |
| `seed.go` | Data demo (4 kantin, 13 menu, akun demo) |

### 1.4 Alur checkout sampai lunas

```
1. POST /api/checkout  {"carts":[{vendor_id, items:[{menu_id, qty, addon_ids, note}]}]}
2. placeOrders()  — SEMUA di dalam satu transaksi database:
     - cek kantin disetujui & buka
     - buildItem(): kurangi stok  WHERE stock >= qty  (kalau tidak cukup → batal)
     - hitung harga dari database (bukan dari aplikasi), topping, subtotal
     - biaya layanan Rp2.000 sekali per checkout
     - total maksimal Rp10.000.000 (batas QRIS)
     - simpan Payment + Order (1 per kantin) + OrderItem
3. createQRIS()  — minta Xendit buat QRIS (15 menit). Gagal → expirePayment() (stok kembali)
4. Aplikasi menampilkan QR (qr_string atau /payments/{id}/qr.png) dan cek status tiap 3 detik
5. Mahasiswa bayar → Xendit memanggil POST /api/webhooks/xendit/qris
6. xenditQRWebhook(): cek token rahasia → markPaid()
     - lockPendingOrders(): kunci baris (FOR UPDATE); kalau sudah bukan pending → abaikan
     - status → paid, saldo setiap kantin += subtotal
7. Tidak dibayar 15 menit → expireStalePayments() → status expired, stok kembali
8. Kantin: PATCH /vendor/orders/{id}/status  paid → preparing → ready → completed
```

Demo tanpa scan sungguhan: `POST /api/dev/payments/{id}/simulate` (hanya aktif di mode mock atau key test Xendit).

## Bagian 2: Menjalankan di laptop

```bash
git clone -b claude/wonderful-bohr-djks1p https://github.com/Recovw/Venture_Creation.git
cd Venture_Creation
# buat file .env (lihat API.md), PostgreSQL harus menyala
go run .
# cek: http://localhost:8080/api/health
```

## Bagian 3: Deploy ke Azure

### 3.1 Gambaran

```
Kode di GitHub
  → GitHub Actions "build-image" (manual)   membangun image Docker (Dockerfile)
  → image dikirim ke ghcr.io/recovw/kantin-api:<sha>  (dan ke ACR)
  → Cloud Shell: az containerapp update --image ...   Azure menjalankan versi baru
  → saat menyala, server memperbarui tabel database otomatis
```

Azure Container Apps di project ini bertipe Express dan tidak bisa menarik image baru dari ACR, jadi selalu pakai image dari **ghcr.io**.

### 3.2 Langkah deploy versi baru

1. Push perubahan kode ke branch `claude/wonderful-bohr-djks1p`.
2. GitHub → **Actions → build-image → Run workflow**, pilih branch itu, tunggu centang hijau.
3. Ambil SHA commit terbaru:
   ```bash
   git rev-parse origin/claude/wonderful-bohr-djks1p
   ```
4. Azure Cloud Shell:
   ```bash
   az containerapp update -n kantin-api -g rg-kantin \
     --image ghcr.io/recovw/kantin-api:<SHA_LENGKAP>
   ```
5. Tunggu 1-2 menit, cek:
   `https://kantin-api.braveground-d38165d9.southeastasia.azurecontainerapps.io/api/health`

Kalau Azure masih menjalankan versi lama, ganti alamat image bergantian antara tag `:latest` dan `:<sha>` supaya Azure membuat kontainer baru.

### 3.3 Mengubah pengaturan (environment variable)

```bash
# variabel biasa
az containerapp update -n kantin-api -g rg-kantin --set-env-vars MAX_ORDER_AMOUNT=500000

# rahasia (mis. key Xendit baru)
az containerapp secret set -n kantin-api -g rg-kantin --secrets xendit-key=ISI_KEY xendit-token=ISI_TOKEN
az containerapp update -n kantin-api -g rg-kantin \
  --set-env-vars XENDIT_SECRET_KEY=secretref:xendit-key XENDIT_CALLBACK_TOKEN=secretref:xendit-token
```

Pengaturan yang dipakai: `DB_DSN`, `JWT_SECRET`, `ADMIN_EMAIL`, `ADMIN_PASSWORD`, `XENDIT_SECRET_KEY`, `XENDIT_CALLBACK_TOKEN`, `SEED_DEMO`, `ADMIN_FEE`, `MAX_ORDER_AMOUNT`, `PAYMENT_TTL_MINUTES`, `CORS_ALLOWED_ORIGINS`, `VENDOR_AUTO_APPROVE`.

### 3.4 Kembali ke versi sebelumnya

```bash
az containerapp update -n kantin-api -g rg-kantin --image ghcr.io/recovw/kantin-api:<SHA_VERSI_LAMA>
```

### 3.5 Terraform (`infra/`)

Terraform dipakai sekali untuk membuat resource Azure: resource group `rg-kantin`, container registry, PostgreSQL Flexible Server, Log Analytics, dan Container Apps.
File state Terraform sudah hilang, jadi **jangan jalankan `terraform apply` atau `terraform destroy`** dari folder hasil clone baru: Terraform akan menganggap belum ada resource dan mencoba membuat ulang semuanya. Semua perubahan rutin cukup lewat `az containerapp ...` di atas.

### 3.6 Cek kalau ada masalah

```bash
az containerapp show -n kantin-api -g rg-kantin --query "properties.template.containers[0].image" -o tsv   # image yang dipakai
az containerapp revision list -n kantin-api -g rg-kantin -o table                                            # revision aktif
```

Webhook Xendit (dashboard Xendit → Developers → Webhooks → QR code paid & refunded):
`https://kantin-api.braveground-d38165d9.southeastasia.azurecontainerapps.io/api/webhooks/xendit/qris`
