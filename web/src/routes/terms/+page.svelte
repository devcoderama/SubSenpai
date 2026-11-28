<svelte:head><title>Syarat Layanan — SubSenpai</title></svelte:head>

<div class="container legal">
  <a class="back" href="/" on:click|preventDefault={() => history.back()}>← Kembali</a>
  <div class="brand">SUB<span>SENPAI</span></div>
  <h1>Syarat Layanan</h1>
  <p class="date">Terakhir diperbarui: 28 November 2026</p>

  <p>Dengan menggunakan aplikasi dan web SubSenpai, kamu menyetujui syarat-syarat berikut.</p>

  <h2>Layanan</h2>
  <p>
    SubSenpai adalah aplikasi dan situs katalog serta pemutar yang menampilkan metadata judul
    (anime, donghua, film) dan memutar konten dari sumber pihak ketiga yang tersedia untuk publik.
    Ketersediaan judul dan konten dapat berubah sewaktu-waktu mengikuti sumbernya.
  </p>

  <h2>Akun</h2>
  <ul>
    <li>Pendaftaran dan masuk hanya melalui akun Google.</li>
    <li>
      Anime dan donghua dapat ditonton tanpa login; film memerlukan Premium atau dibuka dengan
      menonton iklan. Fitur penuh (Daftar Saya, riwayat tersinkron, komentar, chat) memerlukan login.
    </li>
    <li>Kamu bertanggung jawab atas aktivitas yang terjadi melalui akunmu.</li>
    <li>
      Akun yang melanggar syarat ini (spam, penipuan, penyalahgunaan) dapat dibatasi atau dinonaktifkan.
    </li>
  </ul>

  <h2>Premium dan voucher</h2>
  <ul>
    <li>
      Langganan premium memberikan akses tanpa iklan dan fitur tambahan selama masa aktifnya.
    </li>
    <li>
      Pembayaran diproses oleh gerbang pembayaran pihak ketiga (saat ini Tokopay). Masa premium
      ditambahkan setelah pembayaran terkonfirmasi dan dapat menumpuk dengan masa yang sudah ada.
    </li>
    <li>
      Voucher bersifat sekali pakai, tidak dapat diuangkan, dan dapat dibatalkan bila terbukti
      diperoleh secara tidak sah.
    </li>
  </ul>

  <h2>Perilaku pengguna</h2>
  <ul>
    <li>
      Dilarang menyebarkan tautan eksternal dan promosi di chat maupun komentar, kecuali tautan
      undangan grup resmi yang dibuat aplikasi.
    </li>
    <li>Dilarang mengunggah konten yang melanggar hukum atau hak pihak lain.</li>
    <li>Dilarang mencoba mengganggu, membebani, atau mengakali batasan layanan.</li>
  </ul>

  <h2>Konten pihak ketiga</h2>
  <p>
    Judul, gambar poster, dan konten yang ditampilkan berasal dari sumber pihak ketiga dan merupakan
    milik pemegang haknya masing-masing. SubSenpai tidak menyimpan berkas video di servernya.
    Pemegang hak yang keberatan atas penampilan suatu judul dapat menghubungi kami melalui kanal
    kontak di bawah untuk ditindaklanjuti.
  </p>

  <h2>Batasan tanggung jawab</h2>
  <p>
    Layanan disediakan "sebagaimana adanya". Kami berupaya menjaga layanan tetap berjalan, tetapi
    tidak menjamin layanan bebas gangguan atau bahwa setiap judul selalu tersedia.
  </p>

  <h2>Perubahan syarat</h2>
  <p>
    Syarat ini dapat diperbarui; versi terbaru selalu tersedia di halaman ini beserta tanggal
    pembaruannya. Penggunaan berkelanjutan setelah perubahan berarti kamu menyetujui versi terbaru.
  </p>

  <h2>Kontak</h2>
  <p>
    Pertanyaan soal syarat layanan dapat disampaikan kepada pengelola SubSenpai melalui aplikasi
    atau web (menu Lainnya).
  </p>

  <footer>SubSenpai — <a href="/">Beranda</a> · <a href="/privacy">Kebijakan Privasi</a></footer>
</div>

<style>
  .legal { max-width: 720px; padding-top: 32px; padding-bottom: 64px; line-height: 1.65; }
  .back { display: inline-block; margin-bottom: 20px; color: var(--muted); font-size: 14px; }
  .back:hover { color: #fff; }
  .brand { color: var(--red); font-weight: 800; letter-spacing: 2px; font-size: 20px; }
  .brand span { color: #fff; }
  h1 { font-size: 28px; margin: 12px 0 4px; }
  h2 { font-size: 18px; margin: 28px 0 6px; color: #fff; }
  p, li { color: #c9c9c9; font-size: 15px; }
  .date { color: var(--muted); font-size: 13px; }
  a { color: #ff5a52; }
  footer { margin-top: 40px; border-top: 1px solid var(--border); padding-top: 16px; font-size: 13px; color: var(--muted); }
</style>
