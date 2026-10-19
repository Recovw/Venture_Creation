# Panduan Integrasi API untuk Tim Mobile

Diperbarui: 16 Oktober 2026

## Ringkasan

Backend sudah mendukung semua masukan wawancara; APK perlu menambah empat layar utama dan memperbarui alur bayar ke QRIS. Endpoint lama (`POST /orders`, `/register`, `/login`) tetap berjalan, jadi APK lama tidak rusak, tetapi tombol bayarnya tidak berfungsi saat server memakai Xendit.

Prioritas untuk demo, urut dari yang paling penting:

1. **Layar pembayaran QRIS**: gambar QR, tombol "Bayar (simulasi)", cek status sampai lunas (bagian Pembayaran QRIS).
2. **Beranda bergambar + search bar**: menu langsung tampil dengan foto (bagian Alur mahasiswa).
3. **Keranjang per kantin**: satu keranjang per kantin, bisa dibayar sekaligus (bagian Alur mahasiswa).
4. **Detail menu dengan topping dan catatan** (bagian Alur mahasiswa).
5. **Layar pesanan dengan kode ambil** (bagian Layar pesanan).
6. Sisi kantin: daftar mitra, kelola menu + foto, saldo dan pencairan (bagian Alur kantin).

Referensi lengkap setiap endpoint ada di `API_DOCUMENTATION.md` di branch `claude/wonderful-bohr-djks1p`; panduan ini fokus pada urutan layar dan cara memanggilnya.

## Dasar

Semua endpoint diawali base URL yang sama, dan endpoint yang butuh login wajib mengirim token di header.

| Hal | Nilai |
| --- | --- |
| Base URL (Azure) | `https://kantin-api.braveground-d38165d9.southeastasia.azurecontainerapps.io/api` |
| Base URL (laptop, emulator Android) | `http://10.0.2.2:8080/api` |
| Header login | `Authorization: Bearer <token>` |
| Body request | JSON, header `Content-Type: application/json` (kecuali upload foto) |
| Uang | Rupiah, bilangan bulat (`15000` = Rp15.000) |
| Waktu | ISO 8601, contoh `2026-10-16T12:00:00+07:00` |
| Error | Selalu `{"error": "pesan"}` dengan status 4xx/5xx |

**Gambar.** `image_url` bisa berupa path relatif (`/assets/food/bakso.svg`, `/uploads/abc.jpg`) atau URL penuh. Path relatif digabung dengan host tanpa `/api`: `https://kantin-api.braveground-d38165d9.southeastasia.azurecontainerapps.io/assets/food/bakso.svg`. Gambar demo berformat SVG, jadi pastikan komponen gambar di APK bisa menampilkan SVG (atau pakai library SVG); foto unggahan kantin berformat JPG/PNG/WEBP.

**Mode pembayaran.** Panggil `GET /health` saat aplikasi dibuka: `{"ok":true,"payment_mode":"xendit","admin_fee":2000}`. `payment_mode` = `mock` (tanpa Xendit) atau `xendit`. `admin_fee` = biaya layanan yang ditampilkan di ringkasan keranjang.

**Pesan error.** Error 400 dari validasi (field kosong, format salah) berupa teks teknis dari library, misalnya `Key: 'req.Email' Error:Field validation ...`. Untuk error yang mengandung `Key:`, tampilkan pesan umum seperti "Periksa kembali data yang diisi"; error lain ("stok Nasi Goreng tidak cukup", "email sudah terdaftar") boleh ditampilkan apa adanya.

## Login, daftar, dan pembagian halaman per role

Satu layar login untuk semua role; setelah login, buka halaman sesuai `user.role`.

| Endpoint | Body | Hasil |
| --- | --- | --- |
| `POST /login` | `{"email","password"}` | `200 {"user":{"id","name","email","role"},"token"}` · `401` email/password salah |
| `POST /register` (mahasiswa) | `{"name","email","password"}` (password min 5) | `201` sama seperti login · `409` email sudah terdaftar |
| `POST /vendors/register` (kantin) | lihat bagian Alur kantin | `201` + token, role `vendor` |
| `GET /me` | — | data akun dari token |

Arah halaman setelah login:

- `student` → beranda mahasiswa.
- `vendor` → dasbor kantin (cek `GET /vendor/profile` untuk status verifikasi).
- `admin` → halaman admin, atau cukup didemokan lewat Postman bila aplikasi tidak punya mode admin.

Simpan token di penyimpanan aman perangkat. Token berlaku 72 jam; kalau API membalas `401` "token tidak valid", arahkan kembali ke layar login.

Beranda dan pencarian **tidak** butuh login, jadi mahasiswa bisa melihat menu dulu dan baru diminta login saat checkout.

## Alur mahasiswa: beranda sampai checkout

Beranda langsung menampilkan menu bergambar; keranjang disimpan di aplikasi, dipisah per kantin, dan dikirim sekaligus saat checkout.

### Beranda dan pencarian

| Kebutuhan layar | Endpoint |
| --- | --- |
| Daftar kantin (kartu dengan foto) | `GET /vendors` → `id, store_name, description, image_url, location, is_open` |
| Beranda: menu terlaris + daftar kantin | `GET /home` → `{"best_sellers": [...], "vendors": [...]}`; `best_sellers` urut dari `sold` terbanyak, maks 10 (`?limit=` sampai 30) |
| Semua menu | `GET /menus` |
| Search bar | `GET /menus?q=geprek` (cari di nama menu, deskripsi, dan nama kantin) |
| Chip kategori | `GET /menus?category=minuman` |
| Halaman satu kantin | `GET /vendors/{id}` + `GET /menus?vendor_id={id}` |

Satu item menu berisi `id, vendor_id, store_name, name, description, category, image_url, price, stock, vendor_open, addons[]`. Kategori di data demo: `nasi, mie, ayam, berkuah, sayur, snack, minuman`. Nonaktifkan tombol "Tambah" bila `vendor_open` = false (tulis "Tutup") atau `stock` = 0 (tulis "Habis").

### Detail menu: topping dan catatan

Tampilkan `addons` sebagai pilihan centang, misalnya `{"id":1,"name":"Telur ceplok","price":3000}`, plus kolom catatan (maks 200 karakter, misalnya "tidak pedas") dan tombol jumlah. Harga satu porsi = `price` + jumlah harga topping yang dicentang.

### Keranjang per kantin

Keranjang hanya disimpan di aplikasi (backend tidak menyimpan keranjang). Kelompokkan per `vendor_id`; item sama dengan topping dan catatan sama cukup ditambah jumlahnya. Contoh bentuk data:

```json
{
  "1": {"store_name": "Warung Bu Sari", "order_type": "dine_in", "pickup_time": null,
        "items": [{"menu_id": 1, "qty": 2, "addon_ids": [1, 2], "note": "tidak pedas"}]},
  "2": {"store_name": "Mie & Bakso Pak Joko", "order_type": "takeaway", "pickup_time": null,
        "items": [{"menu_id": 4, "qty": 1, "addon_ids": [], "note": ""}]}
}
```

Per kartu kantin, tampilkan pilihan **Bawa pulang / Makan di tempat** (`order_type`: `takeaway` / `dine_in`) dan **Ambil secepatnya / jam tertentu** (`pickup_time`: null = secepatnya; atau `"12:30"` untuk jam hari ini, `"2026-10-19T12:30:00"` tanpa zona dianggap WIB, atau ISO lengkap `"2026-10-19T12:30:00+07:00"`; keluaran `DateTime.toIso8601String()` Flutter juga diterima). Di bawah semua kartu: subtotal semua kantin + biaya layanan (`admin_fee` dari `/health`, dikenakan sekali per checkout) = total.

### Checkout

`POST /checkout` (login mahasiswa). Kirim keranjang yang dipilih, satu atau beberapa kantin (maks 10):

```json
{"carts": [
  {"vendor_id": 1, "order_type": "dine_in",
   "items": [{"menu_id": 1, "qty": 2, "addon_ids": [1, 2], "note": "tidak pedas"}]},
  {"vendor_id": 2, "items": [{"menu_id": 4, "qty": 1}]}
]}
```

Balasan `201`: `{"payment": {"id", "amount", "status":"pending", "expires_at", "orders":[...]}, "payment_mode"}`. Simpan `payment.id`, hapus keranjang yang sudah dikirim, lalu buka layar pembayaran. Satu checkout = satu QRIS, berapa pun jumlah kantinnya; satu pesanan (`order`) per kantin, masing-masing dengan `pickup_code` sendiri.

Error `400` yang perlu ditangani: `stok {nama} tidak cukup`, `{kantin} sedang tutup`, `menu {id} tidak tersedia di kantin ini`, `tambahan tidak valid untuk {nama}`. Tampilkan pesannya dan biarkan keranjang tetap ada.

## Layar pembayaran QRIS

Layar ini menampilkan gambar QRIS, lalu mengecek status setiap 3 detik sampai `paid` atau `expired`; saat demo, tombol "Bayar (simulasi)" menggantikan scan e-wallet.

1. **Tampilkan QR.** Ambil `GET /payments/{id}/qr.png` (PNG 512×512). Endpoint ini butuh header `Authorization`, jadi unduh sebagai bytes lalu tampilkan dari memori; URL-nya tidak bisa langsung dipasang di komponen gambar biasa.
2. **Tampilkan nominal dan batas waktu.** `payment.amount` sebagai total, hitung mundur ke `payment.expires_at` (15 menit), dan rincian per kantin dari `payment.orders[]` (`store_name`, `total`).
3. **Instruksi singkat.** "Buka GoPay, OVO, DANA, ShopeePay, atau m-banking, pilih Scan/QRIS, cek nominal, lalu bayar."
4. **Tombol "Bayar (simulasi)"** memanggil `POST /dev/payments/{id}/simulate` (login mahasiswa, tanpa body).
5. **Cek status** dengan `GET /payments/{id}` setiap 3 detik, mulai sejak layar dibuka; berhenti saat layar ditutup atau status bukan `pending` lagi.
6. **Selesai.** `paid` → layar berhasil yang menampilkan `pickup_code` setiap pesanan (contoh "GMGY" untuk Warung Bu Sari). `expired` → "Waktu bayar habis, pesanan dibatalkan" dan tombol kembali ke beranda (stok sudah dikembalikan server).

| Hasil tombol simulasi | Arti | Yang dilakukan APK |
| --- | --- | --- |
| `200`, `payment.status` = `paid` | Mode mock: langsung lunas | Pindah ke layar berhasil |
| `202`, status masih `pending` | Mode Xendit (sandbox): Xendit memproses lalu mengirim konfirmasi ke server | Teruskan cek status; biasanya `paid` dalam beberapa detik |
| `400` "pembayaran sudah paid/expired" | Sudah selesai | Muat ulang status |
| `404` | Server memakai key Xendit live; simulasi dimatikan | Sembunyikan tombol |
| `502` | Xendit gagal dihubungi | Tampilkan pesan, coba lagi |

Di mode Xendit sandbox, isi QR hanya teks contoh (`some-random-qr-string`), jadi scan dengan e-wallet sungguhan memang tidak berhasil. Saat demo, tekan tombol simulasi dan jelaskan bahwa di mode produksi QR yang sama bisa discan e-wallet apa pun.

Pembayaran yang belum selesai bisa dilanjutkan dari layar pesanan: pesanan berstatus `pending_payment` membawa `payment_id`, buka lagi layar ini dengan id tersebut.

## Layar pesanan mahasiswa

Layar pesanan menampilkan status setiap pesanan dan kode ambilnya, diperbarui dengan `GET /orders` setiap 5 detik selama ada pesanan aktif.

`GET /orders` mengembalikan maks 100 pesanan, terbaru dulu. Field yang dipakai layar: `store_name`, `pickup_code`, `order_type`, `pickup_time`, `status`, `total`, `payment_id`, `created_at`, dan `items[]` (`name`, `image_url`, `qty`, `addons`, `note`). Detail satu pesanan: `GET /orders/{id}`.

| `status` | Label di APK | Tampilan |
| --- | --- | --- |
| `pending_payment` | Menunggu pembayaran | Tombol "Lanjutkan bayar" ke layar QRIS (`payment_id`) |
| `paid` | Pesanan diterima | Langkah 1 dari 4 menyala |
| `preparing` | Sedang dimasak | Langkah 2 dari 4 |
| `ready` | Siap diambil | Langkah 3, kode ambil ditampilkan besar: "Tunjukkan kode GMGY di kantin" |
| `completed` | Selesai | Masuk tab Riwayat |
| `expired` | Dibatalkan (tidak dibayar) | Masuk tab Riwayat |

Bagi layar menjadi tab **Aktif** (`pending_payment`, `paid`, `preparing`, `ready`) dan **Riwayat** (`completed`, `expired`). Status hanya bisa maju berurutan, diubah oleh kantin; mahasiswa tidak mengubah status apa pun. Notifikasi push belum tersedia, jadi status diperbarui dengan pengecekan berkala.

## Alur kantin

Kantin mendaftar sendiri, menunggu verifikasi admin, lalu mengelola menu, memproses pesanan, dan mencairkan saldo; semua endpoint di bawah butuh login role `vendor`.

### Daftar jadi mitra

`POST /vendors/register` (tanpa login). Cocok dibuat sebagai form 3 langkah: akun, info kantin, rekening.

```json
{"name": "Ucok", "email": "ucok@kampus.id", "password": "rahasia123", "phone": "0812345678",
 "store_name": "Lapo Ucok", "location": "Gedung B lt. 1", "description": "Nasi campur",
 "bank_name": "BRI", "bank_account_no": "1234567", "bank_account_name": "Ucok"}
```

Semua field wajib kecuali `description`; password min 8 karakter. Balasan `201` berisi `token` (langsung login) dan `vendor.status` = `pending`.

### Status verifikasi dan profil

`GET /vendor/profile` mengembalikan `status`, `balance`, `is_open`, `image_url`, lokasi, telepon, dan data rekening.

- `pending` → tampilkan banner "Pendaftaran sedang diverifikasi admin". Kantin sudah bisa menyiapkan menu, tetapi belum muncul di aplikasi mahasiswa.
- `rejected` → banner "Pendaftaran ditolak, hubungi admin".
- `approved` → normal.

`PUT /vendor/profile` mengubah sebagian: kirim hanya field yang berubah, misalnya `{"is_open": false}` untuk tombol Buka/Tutup, atau `{"bank_name":"BCA","bank_account_no":"...","bank_account_name":"..."}` untuk rekening.

### Menu dan foto

| Aksi | Endpoint |
| --- | --- |
| Daftar menu sendiri (termasuk yang disembunyikan) | `GET /vendor/menus` |
| Tambah menu | `POST /vendor/menus` |
| Ubah menu | `PUT /vendor/menus/{id}` (`name` dan `price` wajib dikirim ulang) |
| Hapus menu | `DELETE /vendor/menus/{id}` |
| Upload foto | `POST /vendor/upload`, `multipart/form-data` field `file`, JPG/PNG/WEBP maks 3 MB → `{"url": "/uploads/..."}` |

Body tambah/ubah menu:

```json
{"name": "Nasi Uduk Komplit", "price": 15000, "stock": 20, "category": "nasi",
 "description": "Nasi uduk, ayam goreng, tempe orek", "image_url": "/uploads/3f66f717.jpg",
 "is_available": true,
 "addons": [{"name": "Telur balado", "price": 4000}, {"name": "Bawang goreng", "price": 1000}]}
```

Alur foto: upload dulu, ambil `url` dari balasan, lalu kirim sebagai `image_url`. Saat mengubah menu, `addons` yang dikirim menggantikan seluruh daftar topping; kalau `addons` tidak dikirim, topping lama tetap. Foto unggahan di Azure hilang bila server di-restart, jadi untuk demo pakai gambar bawaan `/assets/food/...`.

### Pesanan masuk

`GET /vendor/orders` (cek setiap 5 detik) mengembalikan pesanan yang sudah dibayar. Filter per kolom dengan `?status=paid`, `preparing`, atau `ready`. Tampilkan besar `pickup_code` dan `customer_name`, lalu `order_type`, `pickup_time`, dan setiap item dengan `addons` serta `note` (beri warna mencolok agar tidak terlewat).

Tombol aksi memanggil `PATCH /vendor/orders/{id}/status` dengan status berikutnya; urutan wajib `paid` → `preparing` → `ready` → `completed`:

| Status sekarang | Label tombol | Body |
| --- | --- | --- |
| `paid` | Mulai masak | `{"status":"preparing"}` |
| `preparing` | Siap diambil | `{"status":"ready"}` |
| `ready` | Sudah diambil | `{"status":"completed"}` |

### Saldo dan pencairan dana

`GET /vendor/withdrawals` → `{"balance": 38000, "min_amount": 10000, "withdrawals": [...]}`. Saldo bertambah sebesar subtotal pesanan setiap kali pembayaran lunas.

`POST /vendor/withdrawals` dengan `{"amount": 20000}` mengajukan pencairan ke rekening di profil; saldo langsung berkurang dan status pengajuan `requested`, lalu admin mentransfer dan mengubahnya ke `processed` (atau `rejected`, saldo kembali). Tampilkan `admin_note` di riwayat.

Error `400` yang perlu ditangani: `minimal pencairan Rp10000`, `saldo tidak cukup`, `kantin belum diverifikasi admin`, `lengkapi data rekening bank di profil kantin terlebih dahulu`.

Di mode test Xendit tidak ada uang sungguhan, jadi pencairan saat demo hanya menunjukkan alurnya.

## Admin (opsional)

Halaman admin tidak wajib ada di APK; bila tidak dibuat, verifikasi kantin dan pencairan cukup didemokan lewat Postman.

| Aksi | Endpoint | Body |
| --- | --- | --- |
| Daftar kantin menunggu verifikasi | `GET /admin/vendors?status=pending` | — |
| Setujui atau tolak kantin | `PATCH /admin/vendors/{id}/status` | `{"status":"approved"}` atau `{"status":"rejected"}` |
| Buat akun kantin langsung | `POST /admin/vendors` | `{"name","email","password","store_name"}` (password min 8) |
| Daftar pengajuan pencairan | `GET /admin/withdrawals?status=requested` | — |
| Tandai sudah ditransfer | `PATCH /admin/withdrawals/{id}` | `{"status":"processed","note":"Transfer BCA 16/10"}` |
| Tolak pencairan (saldo kembali) | `PATCH /admin/withdrawals/{id}` | `{"status":"rejected","note":"Rekening salah"}` |

Data admin memuat telepon dan rekening kantin, jadi tampilkan keduanya di halaman verifikasi agar admin bisa mengecek sebelum menyetujui.

## Demo

Demo memakai akun contoh di server Azure dan berjalan dari pesan menu sampai kantin menyelesaikan pesanan, seluruhnya di HP.

| Role | Email | Password |
| --- | --- | --- |
| Mahasiswa | mahasiswa@demo.id | demo1234 |
| Kantin | busari@demo.id (juga pakjoko@demo.id, kopikita@demo.id, sehat@demo.id) | demo1234 |
| Admin | admin@demo.id | admin1234 |

Akun ini muncul di Azure setelah server dijalankan dengan `SEED_DEMO=true` (diatur tim backend).

Skenario di depan dosen dan expert:

1. Buka aplikasi sebagai tamu: beranda langsung menampilkan menu bergambar; ketik "geprek" di search bar.
2. Buka Ayam Geprek, centang "Extra sambal", tulis catatan "tidak pedas", tambah ke keranjang Warung Bu Sari.
3. Tambah Mie Ayam dari Mie & Bakso Pak Joko: keranjang kini punya dua kartu kantin.
4. Checkout (login sebagai mahasiswa), lalu layar QRIS muncul dengan satu QR untuk dua kantin.
5. Tekan "Bayar (simulasi)", lalu layar berubah menjadi berhasil dengan dua kode ambil.
6. Di HP kedua, login sebagai busari@demo.id: pesanan masuk dengan catatan "tidak pedas"; tekan Mulai masak lalu Siap diambil.
7. Kembali ke HP mahasiswa: status "Siap diambil" dan kode ambil tampil besar.
8. Di HP kantin, buka Saldo: saldo bertambah; ajukan pencairan Rp10.000.

Checklist sebelum hari demo:

- [ ] Base URL APK mengarah ke Azure dan `GET /health` berhasil dari HP
- [ ] Gambar SVG dari `/assets/food/` tampil di beranda
- [ ] Layar QRIS menampilkan gambar QR dan tombol simulasi mengubah status menjadi lunas
- [ ] Akun demo bisa login (mahasiswa, kantin, admin)
- [ ] Alur kantin `paid` → `preparing` → `ready` → `completed` berjalan dari APK
- [ ] Dua HP (mahasiswa dan kantin) siap, atau satu HP dengan logout di antara langkah
