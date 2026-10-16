package main

import "log"

type seedMenu struct {
	name, desc, cat, img string
	price                int64
	addons               []MenuAddon
}

type seedVendor struct {
	email, owner, store, desc, loc, img string
	menus                               []seedMenu
}

var (
	addTelur   = MenuAddon{Name: "Telur ceplok", Price: 3000}
	addBawang  = MenuAddon{Name: "Bawang goreng", Price: 1000}
	addSambal  = MenuAddon{Name: "Extra sambal", Price: 1000}
	addKerupuk = MenuAddon{Name: "Kerupuk", Price: 2000}
	addNasi    = MenuAddon{Name: "Nasi tambah", Price: 4000}
	addKeju    = MenuAddon{Name: "Keju parut", Price: 3000}
	addEs      = MenuAddon{Name: "Less ice", Price: 0}
)

var demoVendors = []seedVendor{
	{"busari@demo.id", "Bu Sari", "Warung Bu Sari", "Masakan rumahan: nasi goreng, ayam geprek, soto.", "Kantin Pusat, Gedung A lt. 1", "/assets/food/nasi-goreng.svg", []seedMenu{
		{"Nasi Goreng Spesial", "Nasi goreng kecap dengan telur, ayam suwir, acar, dan kerupuk.", "nasi", "/assets/food/nasi-goreng.svg", 15000, []MenuAddon{addTelur, addBawang, addSambal, addKerupuk}},
		{"Ayam Geprek Sambal Bawang", "Ayam goreng tepung renyah digeprek sambal bawang pedas + nasi.", "ayam", "/assets/food/ayam-geprek.svg", 16000, []MenuAddon{addSambal, addNasi, addKeju, addTelur}},
		{"Soto Ayam Lamongan", "Kuah kuning gurih, suwiran ayam, soun, telur rebus, koya.", "berkuah", "/assets/food/soto-ayam.svg", 14000, []MenuAddon{addNasi, addKerupuk, addTelur}},
	}},
	{"pakjoko@demo.id", "Pak Joko", "Mie & Bakso Pak Joko", "Mie ayam dan bakso urat kenyal, kuah kaldu sapi.", "Kantin Teknik, Gedung C", "/assets/food/bakso.svg", []seedMenu{
		{"Mie Ayam Komplit", "Mie kenyal, ayam kecap, pangsit, sawi, dan daun bawang.", "mie", "/assets/food/mie-ayam.svg", 13000, []MenuAddon{{Name: "Bakso 2 butir", Price: 4000}, addBawang, addSambal, {Name: "Pangsit goreng", Price: 2000}}},
		{"Bakso Urat Jumbo", "Bakso urat jumbo + bakso halus, bihun, kuah kaldu sapi.", "berkuah", "/assets/food/bakso.svg", 15000, []MenuAddon{{Name: "Mie kuning", Price: 2000}, addBawang, addSambal}},
		{"Sate Ayam (10 tusuk)", "Sate ayam bakar bumbu kacang dan kecap, dengan lontong.", "ayam", "/assets/food/sate-ayam.svg", 18000, []MenuAddon{{Name: "Lontong tambah", Price: 3000}, addBawang}},
	}},
	{"kopikita@demo.id", "Rani", "Kedai Kopi & Jajan", "Kopi susu, es teh, roti bakar, pisang goreng untuk teman nugas.", "Kantin Perpustakaan", "/assets/food/kopi-susu.svg", []seedMenu{
		{"Es Kopi Susu Gula Aren", "Espresso, susu segar, dan gula aren.", "minuman", "/assets/food/kopi-susu.svg", 12000, []MenuAddon{{Name: "Extra shot", Price: 4000}, addEs}},
		{"Es Teh Manis Jumbo", "Teh melati segar ukuran jumbo.", "minuman", "/assets/food/es-teh.svg", 5000, []MenuAddon{{Name: "Lemon", Price: 2000}, addEs}},
		{"Es Jeruk Peras", "Jeruk peras asli tanpa pengawet.", "minuman", "/assets/food/es-jeruk.svg", 7000, []MenuAddon{addEs}},
		{"Roti Bakar Coklat Keju", "Roti tebal dipanggang mentega, isi coklat dan keju.", "snack", "/assets/food/roti-bakar.svg", 12000, []MenuAddon{addKeju, {Name: "Susu kental manis", Price: 1000}}},
		{"Pisang Goreng Madu (5 pcs)", "Pisang kepok goreng renyah dengan madu.", "snack", "/assets/food/pisang-goreng.svg", 10000, []MenuAddon{addKeju, {Name: "Coklat meses", Price: 2000}}},
	}},
	{"sehat@demo.id", "Dewi", "Dapur Sehat Mbak Dewi", "Pilihan sehat: gado-gado, salad buah, nasi merah.", "Kantin FEB, dekat parkiran", "/assets/food/gado-gado.svg", []seedMenu{
		{"Gado-Gado Siram", "Sayur rebus, tahu, tempe, telur, lontong, bumbu kacang.", "sayur", "/assets/food/gado-gado.svg", 13000, []MenuAddon{addKerupuk, addTelur, {Name: "Bumbu pedas", Price: 0}}},
		{"Salad Buah Yoghurt", "Potongan buah segar dengan saus yoghurt dan keju.", "snack", "/assets/food/salad-buah.svg", 12000, []MenuAddon{addKeju}},
	}},
}

func emailExists(email string) bool {
	var n int64
	db.Model(&User{}).Where("email = ?", email).Count(&n)
	return n > 0
}

// seedDemo (SEED_DEMO=true) menambahkan akun & kantin contoh yang belum ada, dicek per email,
// jadi aman dijalankan berulang dan pada database yang sudah berisi data.
func seedDemo() {
	added := 0
	if !emailExists("admin@demo.id") {
		db.Create(&User{Name: "Admin Demo", Email: "admin@demo.id", Password: hashPassword("admin1234"), Role: RoleAdmin})
		added++
	}
	if !emailExists("mahasiswa@demo.id") {
		db.Create(&User{Name: "Mahasiswa Demo", Email: "mahasiswa@demo.id", Password: hashPassword("demo1234"), Role: RoleStudent})
		added++
	}
	for _, sv := range demoVendors {
		if emailExists(sv.email) {
			continue
		}
		vp := VendorProfile{StoreName: sv.store, Description: sv.desc, Location: sv.loc, ImageURL: sv.img,
			Phone: "081234567890", Status: VendorApproved, IsOpen: true, BankName: "BCA",
			BankAccountNo: "1234567890", BankAccountName: sv.owner}
		if _, err := createVendorAccount(sv.owner, sv.email, "demo1234", &vp); err != nil {
			log.Println("seed:", err)
			continue
		}
		added++
		for _, sm := range sv.menus {
			adds := append([]MenuAddon(nil), sm.addons...)
			db.Create(&Menu{VendorID: vp.ID, Name: sm.name, Description: sm.desc, Category: sm.cat,
				ImageURL: sm.img, Price: sm.price, Stock: 50, IsAvailable: true, Addons: adds})
		}
	}
	if added > 0 {
		log.Println("data demo ditambahkan: mahasiswa@demo.id / demo1234, busari@demo.id / demo1234, admin@demo.id / admin1234")
	}
}
