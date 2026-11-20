# NOTE.md — Catatan Keputusan & Pertimbangan SubSenpai

Catatan keputusan yang belum (atau sengaja tidak) menjadi bagian dokumen rancangan utama, disimpan agar alasannya tidak hilang. Dokumen rancangan: [README.md](README.md) · [STRUKTUR.md](STRUKTUR.md) · [DATABASE.md](database/DATABASE.md) · [STRUKTUR_ADMIN.md](STRUKTUR_ADMIN.md) · [FITUR.md](FITUR.md) · [KEAMANAN.md](KEAMANAN.md) · [OPTIMASI.md](OPTIMASI.md).

---

## 1. Distribusi Aplikasi: Hanya Website Resmi & GitHub (Keputusan, 20 Nov 2026)

**Keputusan:** selama tahap awal, SubSenpai **tidak dirilis ke Google Play Store**. Aplikasi hanya bisa diunduh dari dua sumber resmi:

1. **Website resmi SubSenpai** — halaman unduh statis (bukan aplikasi web; SubSenpai tetap aplikasi Android saja, tanpa versi web).
2. **GitHub Releases** di repo resmi SubSenpai (github.com/devcoderama/SubSenpai).

**Alasan:** aplikasi masih tahap awal dan ada kendala biaya — biaya akun Play Store memang hanya sekali bayar, tetapi ditunda dulu, dan pertimbangan kebijakan di Bagian 2 membuat jalur Play Store belum menarik untuk sekarang.

**Konsekuensi yang disadari:**
- User memasang APK secara manual (perlu mengizinkan pemasangan dari browser/aplikasi GitHub di perangkatnya).
- Tidak ada pembaruan otomatis dari Play Store — diatasi oleh **gerbang versi minimum (O17, KEAMANAN.md)** dan layar perbarui di aplikasi yang selalu menunjuk ke sumber unduhan resmi.
- Pertumbuhan mengandalkan promosi komunitas dan tautan resmi aplikasi (kartu bagikan, tautan undangan grup) — bukan penemuan lewat Play Store.

**Keamanan distribusi:**
- APK resmi **hanya** ada di dua sumber di atas; sumber lain yang mengatasnamakan SubSenpai dianggap tidak resmi.
- Halaman unduh & rilis GitHub menampilkan **hash SHA-256** berkas APK agar user bisa memverifikasi berkas yang diunduh (pola yang sama dengan rilis D TRADE).

## 2. Pertimbangan Google Play Store (Dicatat untuk Ditinjau Ulang Nanti)

Fakta & aturan Play Store per November 2026, hasil penelusuran saat keputusan di Bagian 1 diambil:

- **Biaya akun**: Play Console **US$25 sekali bayar** seumur akun (kartu kredit/debit; kartu prepaid tidak diterima). Tidak ada biaya tahunan dan tidak ada biaya per aplikasi. Bila menjual lewat sistem pembayaran Play, ada potongan layanan 15% untuk penghasilan sampai US$1 juta pertama per tahun.
- **Syarat akun personal baru**: verifikasi identitas (dokumen resmi) dan verifikasi kepemilikan perangkat Android asli; **wajib uji tertutup minimal 12 penguji yang aktif selama 14 hari** sebelum bisa mengajukan akses produksi; review produksi biasanya sampai ±7 hari. Pilihan jenis akun **personal vs organisasi tidak bisa diubah** belakangan (organisasi butuh nomor D-U-N-S dan bebas dari aturan uji tertutup).
- **Format unggahan**: wajib **AAB (Android App Bundle)** — APK polos tidak diterima untuk aplikasi baru; tanda tangan aplikasi dikelola Google (Play App Signing).
- **Benturan terbesar — Play Billing**: langganan/barang digital yang dijual **di dalam** aplikasi yang tayang di Play Store **wajib memakai Google Play Billing**. Rancangan monetisasi SubSenpai (Midtrans/Xendit/GoPay + voucher berbayar di dalam aplikasi) tidak sesuai dengan aturan itu untuk penjualan di dalam aplikasi. Yang tidak terkena aturan ini: voucher dari admin, hadiah antar-user, dan voucher referral — karena bukan penjualan di dalam aplikasi.
- **Risiko review konten**: aplikasi streaming dengan konten hasil scraping sensitif terhadap kebijakan hak cipta Play — ada risiko penolakan saat review atau pencabutan di kemudian hari.
- **Konteks tambahan**: verifikasi identitas developer Android untuk distribusi **di luar** Play Store juga mulai berlaku bertahap (Indonesia termasuk gelombang awal) — jalur APK pun ke depan memiliki gerbang identitas developer, walau bukan gerbang review konten seperti Play Store.

## 3. Syarat Meninjau Ulang Keputusan Distribusi

Keputusan di Bagian 1 ditinjau ulang bila **ketiganya** terpenuhi:

1. Aplikasi sudah stabil dan dipakai rutin (bukan lagi tahap awal).
2. Anggaran akun Play Console tersedia.
3. Rancangan pembayaran premium sudah disesuaikan untuk dua jalur: Play Billing untuk versi Play Store, dan jalur pembayaran sendiri (Midtrans/GoPay/voucher) tetap untuk distribusi APK — premium user harus setara di kedua jalur.

Hasil peninjauan dicatat di dokumen ini, dan pernyataan distribusi di STRUKTUR.md/README.md diselaraskan lagi saat itu.
