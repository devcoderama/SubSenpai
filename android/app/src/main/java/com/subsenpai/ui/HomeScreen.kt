package com.subsenpai.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyListState
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.Card
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.TextButton
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.OutlinedButton
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.material3.OutlinedTextField
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.text.TextStyle
import androidx.compose.material3.Text
import androidx.compose.animation.Crossfade
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.runtime.Composable
import androidx.compose.runtime.derivedStateOf
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import coil.compose.AsyncImage
import coil.request.ImageRequest
import com.subsenpai.R
import com.subsenpai.data.CatalogRepository
import com.subsenpai.data.SettingsStore
import com.subsenpai.data.TitleCard
import com.subsenpai.data.TitleFacets
import com.subsenpai.data.WatchHistoryItem
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.TimeoutCancellationException
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.withTimeout

private val typeFilters = listOf(
    null to "Semua", "film" to "Film", "series" to "Serial TV", "anime" to "Anime",
    "donghua" to "Donghua",
)

private val rowTypes = listOf(
    "anime" to "Anime Pilihan", "donghua" to "Donghua Pilihan", "film" to "Film Pilihan",
    "series" to "Serial TV Pilihan",
)

/**
 * Sesi telusur Beranda (permintaan Rams 2026-11-24): keadaan
 * halaman Beranda — tab kategori aktif, genre/filter, hasil yang
 * sudah dimuat, posisi scroll — disimpan di pemegang ini yang
 * hidup di MainActivity, DI LUAR NavHost. Dulu semuanya `remember`
 * di dalam HomeScreen: membuka Detail membuang komposisi Beranda,
 * jadi tombol kembali selalu mendarat di tab "Semua" posisi atas
 * dan memuat ulang dari nol. Kini kembali dari Detail/Pemutar
 * mengembalikan tab + posisi scroll persis seperti ditinggalkan
 * (di awal tetap di awal, di tengah tetap di tengah), tanpa muat
 * ulang. Sesi hanya di memori: aplikasi ditutup = mulai segar.
 */
class HomeSession {
    val trending = mutableStateOf<List<TitleCard>>(emptyList())
    val homeLoaded = mutableStateOf(false)
    val terbaru = mutableStateOf<List<TitleCard>>(emptyList())
    val perType = mutableStateOf<Map<String, List<TitleCard>>>(emptyMap())
    /** Rel Terbaru per jenis (tahun berjalan) — 28 Nov 2026. */
    val perTypeYear = mutableStateOf<Map<String, List<TitleCard>>>(emptyMap())
    /** Rel Ongoing per jenis (anime/donghua) — 28 Nov 2026. */
    val ongoing = mutableStateOf<Map<String, List<TitleCard>>>(emptyMap())
    val homeLive = mutableStateOf<Map<String, List<CatalogRepository.LiveBrowseItem>>>(emptyMap())
    /** Kanal TV Langsung aktif (IPTV, 28 Nov 2026) — dimuat di latar. */
    val tvChannels = mutableStateOf<List<CatalogRepository.TVChannel>>(emptyList())
    val lanjut = mutableStateOf<List<WatchHistoryItem>>(emptyList())
    val reloadKey = mutableStateOf(0)
    val type = mutableStateOf<String?>(null)
    val query = mutableStateOf("")

    /** Posisi scroll Beranda mode rel (tab Semua, tanpa telusur). */
    val homeScroll = LazyListState()

    /** reloadKey terakhir yang rel Berandanya sudah dimuat. */
    var homeReloadSeen = Int.MIN_VALUE

    private val tabs = mutableMapOf<String, HomeTabSession>()
    fun tabFor(type: String?): HomeTabSession =
        tabs.getOrPut(type ?: "semua") { HomeTabSession() }
}

/** Keadaan satu tab kategori Beranda (Semua/Anime/Donghua/Film/Serial TV). */
class HomeTabSession {
    val genre = mutableStateOf<String?>(null)
    val genreList = mutableStateOf<List<String>>(emptyList())
    val year = mutableStateOf<Int?>(null)
    val country = mutableStateOf<String?>(null)
    val network = mutableStateOf<String?>(null)
    /** Status ongoing/completed — disaring di klien (28 Nov 2026,
     *  backend belum punya filter status). */
    val status = mutableStateOf<String?>(null)
    val facets = mutableStateOf<TitleFacets?>(null)
    val hasil = mutableStateOf<List<TitleCard>>(emptyList())
    val hasilCursor = mutableStateOf<String?>(null)
    val hasilHasMore = mutableStateOf(false)
    val liveItems = mutableStateOf<List<CatalogRepository.LiveBrowseItem>>(emptyList())
    val liveSource = mutableStateOf("")
    val liveHasMore = mutableStateOf(false)
    val livePage = mutableStateOf(1)

    /** Posisi scroll daftar Hasil tab ini. */
    val scroll = LazyListState()

    /** Tanda tangan filter (genre|tahun|negara|jaringan|query) yang
     *  hasil saat ini mencerminkan; null = belum pernah dimuat.
     *  Dipakai agar kembali dari Detail TIDAK memuat ulang hasil
     *  dari halaman pertama (yang akan merusak posisi scroll). */
    var hasilSignature: String? = null

    /** Genre terakhir yang bagian live-nya sudah diproses, dan
     *  apakah muatannya berhasil — pasangan penanda untuk efek
     *  telusur langsung agar tidak mengulang saat kembali. */
    var liveGenre: String? = null
    var liveLoaded: Boolean = false
}

/**
 * Muat halaman katalog dengan saringan status di klien (28 Nov
 * 2026): backend belum punya filter status, jadi halaman diambil
 * satu demi satu sampai `want` judul cocok terkumpul atau katalog
 * habis (maks 4 halaman tambahan) supaya grid tidak kosong.
 * Tanpa status, tepat satu halaman — perilaku lama tidak berubah.
 */
private suspend fun titlesFiltered(
    repo: CatalogRepository,
    type: String?,
    query: String?,
    genre: String?,
    year: Int?,
    country: String?,
    network: String?,
    status: String?,
    cursor: String?,
    want: Int,
): Pair<List<TitleCard>, String?> {
    var out = emptyList<TitleCard>()
    var cur = cursor
    var next: String? = null
    var extra = 0
    do {
        val (items, nx) = repo.titlesPaged(type, query, genre, year, country, network, cur)
        out = out + if (status != null) items.filter { it.status == status } else items
        next = nx
        cur = nx
        extra++
    } while (status != null && out.size < want && next != null && extra <= 4)
    return out to next
}

/**
 * Beranda ala Netflix (redesain 2026-11-22 atas contoh UI Netflix
 * dari Rams): header melayang transparan di atas hero — logo "S"
 * merah, ikon cari & akun, tab kategori teks (Semua/Anime/Donghua/
 * Film/Serial TV) — hero layar penuh dengan baris aksi Daftar Saya / Putar /
 * Info, lalu baris-baris poster. Navigasi utama pindah ke bilah
 * bawah di MainActivity.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun HomeScreen(
    repo: CatalogRepository,
    session: HomeSession,
    authVersion: Int,
    onOpenTitle: (String) -> Unit,
    onOpenEpisode: (String) -> Unit,
    onLogin: () -> Unit,
    onOpenMore: () -> Unit,
    onOpenHistory: () -> Unit = {},
    onOpenNotices: () -> Unit = {},
    onOpenPremium: () -> Unit = {},
    onOpenTV: () -> Unit = {},
    onOpenChannel: (CatalogRepository.TVChannel) -> Unit = {},
    onFatalChange: (Boolean) -> Unit = {},
) {
    val homeContext = LocalContext.current
    val settings = remember { SettingsStore(homeContext) }
    // Keadaan telusur hidup di HomeSession (di luar NavHost) agar
    // selamat dari navigasi ke Detail/Pemutar — lihat kelasnya di
    // atas. Yang murni sesaat (dialog, penanda muat) tetap lokal.
    var type by session.type
    val tab = session.tabFor(type)
    var trending by session.trending
    var homeLoaded by session.homeLoaded
    var terbaru by session.terbaru
    var perType by session.perType
    // Baris Beranda dari BERANDA situs sumber (terbaru/populer versi
    // situs resminya — sama seperti halaman download; permintaan
    // Rams 2026-11-23). Bila sumber kosong/gagal, baris katalog
    // (perType) yang tampil seperti sebelumnya.
    var homeLive by session.homeLive
    var tvChannels by session.tvChannels
    var hasil by tab.hasil
    var lanjut by session.lanjut
    var error by remember { mutableStateOf<String?>(null) }
    var fatalKind by remember { mutableStateOf<AppErrorKind?>(null) }
    // Muat Beranda yang sudah >10 detik: tawarkan "Coba lagi" di
    // bawah layar (overlay) supaya pengguna tidak terjebak di
    // spinner tanpa jalan keluar saat koneksi menggantung.
    var showHomeRetry by remember { mutableStateOf(false) }
    var reloadKey by session.reloadKey
    var query by session.query
    var genre by tab.genre
    var genreList by tab.genreList
    var year by tab.year
    var country by tab.country
    var network by tab.network
    var status by tab.status
    var perTypeYear by session.perTypeYear
    var ongoing by session.ongoing
    // Tahun berjalan untuk rel Terbaru (Film/Anime/Donghua Terbaru).
    val thisYear = java.util.Calendar.getInstance().get(java.util.Calendar.YEAR)
    // Film Populer: film dari trending 24 jam (sama seperti web).
    val filmPopuler = trending.filter { it.type == "film" }
    // Serial TV Populer: serial dari trending 24 jam (pola yang sama).
    val seriesPopuler = trending.filter { it.type == "series" }
    var facets by tab.facets
    var filterOpen by remember { mutableStateOf(false) }
    var searchOpen by remember { mutableStateOf(false) }
    val browsing = type != null || query.isNotBlank()
    val scope = rememberCoroutineScope()

    // Telusur langsung dari situs sumber saat genre dipilih:
    // pilihan mengikuti halaman genre web sumbernya, bukan hanya
    // katalog lokal. Judul yang belum ada di katalog diimpor saat
    // diketuk.
    var liveItems by tab.liveItems
    var liveSource by tab.liveSource
    var liveLoading by remember { mutableStateOf(false) }
    // Gagal memuat halaman live berikutnya secara otomatis: muat
    // otomatis berhenti (agar tidak mengulang tanpa henti) dan
    // tombol manual tampil lagi sebagai jalan coba ulang.
    var liveAutoFailed by remember { mutableStateOf(false) }
    var liveHasMore by tab.liveHasMore
    var livePage by tab.livePage
    var importingSlug by remember { mutableStateOf<String?>(null) }
    LaunchedEffect(type, genre) {
        // Kembali dari Detail dengan genre sama yang sudah termuat:
        // jangan kosongkan / muat ulang bagian live.
        if (tab.liveGenre == genre && (genre == null || tab.liveLoaded)) {
            return@LaunchedEffect
        }
        tab.liveGenre = genre
        tab.liveLoaded = false
        liveItems = emptyList()
        liveSource = ""
        liveHasMore = false
        livePage = 1
        liveAutoFailed = false
        val t = type
        val g = genre
        if (t != null && g != null) {
            liveLoading = true
            try {
                val (src, more, items) = repo.liveBrowse(t, g, 1)
                liveSource = src
                liveHasMore = more
                liveItems = items
                tab.liveLoaded = true
            } catch (e: Exception) {
                // Sumber tak terjangkau: bagian katalog tetap tampil.
            } finally {
                liveLoading = false
            }
        }
    }
    val onLiveTap: (CatalogRepository.LiveBrowseItem) -> Unit = { item ->
        val cid = item.catalogId
        if (cid != null) {
            onOpenTitle(cid)
        } else if (importingSlug == null) {
            importingSlug = item.slug
            scope.launch {
                try {
                    val id = repo.importTitle(type ?: "", item, genre)
                    onOpenTitle(id)
                } catch (e: Exception) {
                    android.widget.Toast.makeText(
                        homeContext,
                        "Gagal menambahkan judul: ${e.message}",
                        android.widget.Toast.LENGTH_SHORT,
                    ).show()
                } finally {
                    importingSlug = null
                }
            }
        }
    }

    // Ketukan pada baris Beranda dari situs sumber: judul yang sudah
    // ada di katalog langsung dibuka; yang belum, diimpor dulu
    // (sama seperti telusur genre langsung & halaman download).
    val onHomeLiveTap: (String, CatalogRepository.LiveBrowseItem) -> Unit = { t, item ->
        val cid = item.catalogId
        if (cid != null) {
            onOpenTitle(cid)
        } else if (importingSlug == null) {
            importingSlug = item.slug
            scope.launch {
                try {
                    val id = repo.importTitle(t, item, null)
                    onOpenTitle(id)
                } catch (e: Exception) {
                    android.widget.Toast.makeText(
                        homeContext,
                        "Gagal menambahkan judul: ${e.message}",
                        android.widget.Toast.LENGTH_SHORT,
                    ).show()
                } finally {
                    importingSlug = null
                }
            }
        }
    }

    // "Lihat semua" rel Beranda (28 Nov 2026): buka mode telusur
    // jenis itu dengan filter relnya (tahun/status), lalu mode
    // Hasil memuat daftar yang sama — tanpa layar baru.
    val openBrowse: (String, Int?, String?) -> Unit = { t, y, s ->
        val target = session.tabFor(t)
        target.genre.value = null
        target.year.value = y
        target.country.value = null
        target.network.value = null
        target.status.value = s
        query = ""
        type = t
    }

    // Lencana lonceng di header: jumlah SEMUA notifikasi yang belum
    // dibaca (admin + postingan + episode baru favorit), diperiksa
    // berkala selama Beranda tampil (permintaan Rams: lonceng di
    // samping tombol cari, bukan di tab Lainnya).
    var notifUnread by remember { mutableStateOf(0) }
    LaunchedEffect(Unit) {
        while (true) {
            try {
                notifUnread = repo.feedNotifications(null).second
            } catch (e: Exception) {
                // Tamu / gagal jaringan: lencana tetap seperti terakhir.
            }
            delay(60_000)
        }
    }

    LaunchedEffect(reloadKey) {
        // Rel Beranda untuk reloadKey ini sudah termuat di sesi:
        // kembali dari halaman lain tidak perlu memuat ulang.
        if (session.homeReloadSeen == reloadKey && homeLoaded) {
            return@LaunchedEffect
        }
        session.homeReloadSeen = reloadKey
        try {
            // Batas 25 detik untuk muat utama: koneksi yang menggantung
            // (byte menetes pelan sehingga read timeout OkHttp tidak
            // sempat bekerja) berubah jadi halaman error + tombol coba
            // lagi, bukan spinner tanpa ujung (laporan Rams 27 Nov
            // 21:01 — Beranda tamu muter terus di v0.24.32).
            val (tr, te) = withTimeout(25_000) { repo.home() }
            trending = tr
            terbaru = te
            fatalKind = null
        } catch (e: TimeoutCancellationException) {
            fatalKind = AppErrorKind.NETWORK
        } catch (e: CancellationException) {
            // Efek dibatalkan (pindah halaman): bukan kegagalan muat.
            throw e
        } catch (e: Exception) {
            // Kegagalan muat utama TIDAK ditampilkan mentah di Beranda:
            // pengguna diarahkan ke halaman peringatan khusus.
            fatalKind = classifyError(e)
        }
        homeLoaded = true
        val loaded = mutableMapOf<String, List<TitleCard>>()
        for ((t, _) in rowTypes) {
            try {
                loaded[t] = repo.titles(t, null)
            } catch (e: Exception) {
                // Baris jenis ini dilewati bila gagal.
            }
        }
        perType = loaded
        // Rel Terbaru per jenis (tahun berjalan) — satu panggilan
        // per jenis, ikut muat utama.
        val yearLoaded = mutableMapOf<String, List<TitleCard>>()
        for ((t, _) in rowTypes) {
            try {
                yearLoaded[t] = repo.titles(t, null, year = thisYear)
            } catch (e: Exception) {
                // Rel Terbaru jenis ini dilewati bila gagal.
            }
        }
        perTypeYear = yearLoaded
        // Rel Ongoing dimuat di latar (tidak menahan Beranda): rel
        // muncul saat siap. Donghua ongoing jarang di halaman
        // teratas, jadi dipindai sampai 8 halaman.
        scope.launch {
            val ongoingLoaded = mutableMapOf<String, List<TitleCard>>()
            for (t in listOf("anime", "donghua")) {
                try {
                    val out = mutableListOf<TitleCard>()
                    var cursor: String? = null
                    var pages = 0
                    while (out.size < 12 && pages < 8) {
                        val (items, next) = repo.titlesPaged(t, null, cursor = cursor)
                        out += items.filter { it.status == "ongoing" }.take(12 - out.size)
                        cursor = next
                        pages++
                        if (next == null) break
                    }
                    if (out.isNotEmpty()) ongoingLoaded[t] = out
                } catch (e: Exception) {
                    // Rel Ongoing jenis ini dilewati bila gagal.
                }
            }
            ongoing = ongoingLoaded
        }
        val liveLoaded = mutableMapOf<String, List<CatalogRepository.LiveBrowseItem>>()
        for ((t, _) in rowTypes) {
            try {
                val (_, _, items) = repo.liveBrowse(t, null, 1)
                if (items.isNotEmpty()) liveLoaded[t] = items
            } catch (e: Exception) {
                // Sumber tak terjangkau: baris katalog tetap tampil.
            }
        }
        homeLive = liveLoaded
        // Kanal TV Langsung dimuat di latar (tidak menahan Beranda):
        // rel muncul saat siap; gagal = rel tidak tampil.
        scope.launch {
            try {
                tvChannels = repo.tvChannels()
            } catch (e: Exception) {
                // Rel TV dilewati bila gagal.
            }
        }
    }
    // Penghitung 10 detik untuk tombol "Coba lagi": mulai tiap kali
    // muat Beranda dimulai (homeLoaded false), batal bila muat
    // selesai sebelum 10 detik.
    LaunchedEffect(homeLoaded) {
        if (!homeLoaded) {
            delay(10_000)
            if (!homeLoaded) showHomeRetry = true
        } else {
            showHomeRetry = false
        }
    }
    LaunchedEffect(authVersion) {
        lanjut = if (repo.isLoggedIn) {
            try {
                repo.watchHistory()
            } catch (e: Exception) {
                emptyList()
            }
        } else {
            emptyList()
        }
    }
    LaunchedEffect(type) {
        if (type != null) {
            try {
                val f = repo.filters(type!!)
                facets = f
                genreList = f.genres
            } catch (e: Exception) {
                // Backend lama (tanpa /filters): chip genre tetap jalan.
                facets = null
                genreList = try {
                    repo.genres(type!!)
                } catch (e2: Exception) {
                    emptyList()
                }
            }
        } else {
            facets = null
            genreList = emptyList()
        }
    }
    // Pagination Hasil: halaman pertama dimuat saat filter berubah;
    // halaman berikutnya dimuat bertahap saat pengguna menggulir
    // mendekati bawah (ala Netflix/YouTube) agar ringan & mulus.
    var hasilCursor by tab.hasilCursor
    var hasilHasMore by tab.hasilHasMore
    var hasilLoadingMore by remember { mutableStateOf(false) }
    // Muat awal Hasil (filter/pencarian berubah): selama berjalan
    // dan Hasil masih kosong, grid menampilkan skeleton loading.
    var hasilLoading by remember { mutableStateOf(false) }
    LaunchedEffect(type, query, genre, year, country, network, status) {
        if (!browsing) return@LaunchedEffect
        // Hasil untuk kombinasi filter ini sudah termuat di sesi
        // (kembali dari Detail/Pemutar): pertahankan daftar yang
        // sudah terpaginasi + posisi scroll-nya, jangan muat ulang
        // dari halaman pertama.
        val sig = listOf(
            genre ?: "", year?.toString() ?: "", country ?: "",
            network ?: "", query, status ?: "",
        ).joinToString("|")
        if (tab.hasilSignature == sig) return@LaunchedEffect
        // Filter benar-benar berubah: mulai dari atas lagi.
        tab.scroll.scrollToItem(0)
        hasilLoading = true
        try {
            val (page, next) = titlesFiltered(
                repo, type, query.ifBlank { null }, genre, year, country, network, status, null, 30,
            )
            hasil = page
            hasilCursor = next
            hasilHasMore = next != null
            error = null
            // Tanda tangan dicatat HANYA sesudah muat berhasil:
            // muat yang gagal/terbatalkan harus dicoba lagi saat
            // pengguna kembali, bukan dianggap sudah termuat.
            tab.hasilSignature = sig
        } catch (e: CancellationException) {
            // Efek tergantikan (filter berubah lagi / halaman
            // ditinggalkan): BUKAN kegagalan muat — jangan tampilkan
            // "Gagal memuat" untuk pembatalan (bug v0.24.19).
            throw e
        } catch (e: Exception) {
            error = friendlyInlineError(e)
        } finally {
            hasilLoading = false
        }
    }
    val loadMoreHasil: () -> Unit = {
        val cur = hasilCursor
        if (!hasilLoadingMore && hasilHasMore && cur != null) {
            hasilLoadingMore = true
            scope.launch {
                try {
                    val (more, next) = titlesFiltered(
                        repo, type, query.ifBlank { null }, genre, year, country, network, status, cur, 30,
                    )
                    val have = hasil.map { it.id }.toSet()
                    hasil = hasil + more.filter { it.id !in have }
                    hasilCursor = next
                    hasilHasMore = next != null
                } catch (e: Exception) {
                    // Gagal memuat lanjutan: berhenti diam-diam,
                    // dicoba lagi pada guliran berikutnya.
                } finally {
                    hasilLoadingMore = false
                }
            }
        }
    }
    // Muat halaman berikutnya bagian live (sumber) — pola yang
    // sama dengan loadMoreHasil: dipicu guliran mendekati ujung.
    // Dulu bagian ini hanya punya tombol "Muat lebih banyak" yang
    // harus diketuk (keluhan Rams 2026-11-24 di tab Film: tidak
    // otomatis seperti Hasil); kini otomatis, tombol hanya tampil
    // lagi bila muat otomatis gagal.
    val loadMoreLive: () -> Unit = {
        val t = type
        val g = genre
        if (t != null && g != null && liveHasMore && !liveLoading && !liveAutoFailed) {
            val next = livePage + 1
            liveLoading = true
            scope.launch {
                try {
                    val (_, more, moreItems) = repo.liveBrowse(t, g, next)
                    livePage = next
                    liveHasMore = more
                    liveItems = liveItems + moreItems
                } catch (e: Exception) {
                    liveAutoFailed = true
                } finally {
                    liveLoading = false
                }
            }
        }
    }
    // Satu LazyColumn untuk dua mode: mode rel memakai scroll sesi
    // Beranda, mode telusur memakai scroll sesi tab aktif — posisi
    // masing-masing pulih persis saat kembali (lihat HomeSession).
    val listState = if (browsing) tab.scroll else session.homeScroll
    // WAJIB remember(listState): listState kini berganti objek per
    // mode/tab (lihat HomeSession) — remember polos akan menangkap
    // objek komposisi PERTAMA selamanya, sehingga pemicu muat-
    // lanjutan membaca daftar yang salah dan paginasi Hasil mati
    // (bug v0.24.19: hasil berhenti di halaman pertama).
    val nearEnd by remember(listState) {
        derivedStateOf {
            val info = listState.layoutInfo
            val last = info.visibleItemsInfo.lastOrNull()?.index ?: -1
            info.totalItemsCount > 0 && last >= info.totalItemsCount - 5
        }
    }
    LaunchedEffect(nearEnd, hasilHasMore, hasilLoadingMore, liveHasMore, liveLoading, liveAutoFailed) {
        if (nearEnd && browsing) {
            loadMoreHasil()
            loadMoreLive()
        }
    }

    LaunchedEffect(fatalKind) { onFatalChange(fatalKind != null) }
    if (fatalKind != null) {
        ErrorPage(
            kind = fatalKind!!,
            onRetry = {
                fatalKind = null
                homeLoaded = false
                reloadKey++
            },
        )
        return
    }

    Box(modifier = Modifier.fillMaxSize()) {
        LazyColumn(modifier = Modifier.fillMaxSize(), state = listState) {
            if (browsing) {
                // Ruang untuk header melayang (tingginya kini tetap —
                // bilah cari menggantikan baris ikon, bukan menambah
                // baris baru).
                item { Spacer(Modifier.height(140.dp)) }
            }
            if (!homeLoaded && error == null) {
                // Umpan beranda belum tiba: animasi merek, bukan halaman kosong.
                item {
                    BrandLoading(modifier = Modifier.fillParentMaxHeight())
                }
            }
            if (error != null) {
                item {
                    Text(
                        error!!,
                        color = MaterialTheme.colorScheme.error,
                        modifier = Modifier.padding(horizontal = 16.dp),
                    )
                }
            }
            if (browsing) {
                if (type != null && (genreList.isNotEmpty() || facets != null)) {
                    item {
                        LazyRow(
                            horizontalArrangement = Arrangement.spacedBy(8.dp),
                            modifier = Modifier.padding(horizontal = 16.dp, vertical = 6.dp),
                        ) {
                            val extraActive = listOf(year, country, network, status).count { it != null }
                            val hasExtra = facets?.let {
                                it.years.isNotEmpty() || it.countries.isNotEmpty() || it.networks.isNotEmpty()
                            } == true
                            if (hasExtra) {
                                item {
                                    GenreChip(
                                        if (extraActive > 0) "Filter ($extraActive)" else "Filter",
                                        extraActive > 0,
                                    ) { filterOpen = true }
                                }
                            }
                            item {
                                GenreChip("Semua Genre", genre == null) { genre = null }
                            }
                            items(genreList) { g ->
                                GenreChip(g, genre == g) { genre = g }
                            }
                        }
                    }
                }
                item { SectionTitle("Hasil") }
                if (hasil.isEmpty() && hasilLoading && error == null) {
                    // Skeleton loading (ala YouTube/Netflix): bayangan
                    // kartu poster selama hasil pertama dimuat.
                    items(3) { SkeletonPosterRow() }
                }
                if (hasil.isEmpty() && !hasilLoading && error == null) {
                    item { Text("Belum ada judul yang cocok.", modifier = Modifier.padding(horizontal = 16.dp)) }
                }
                items(hasil.chunked(3)) { rowTitles ->
                    Row(
                        modifier = Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 6.dp),
                        horizontalArrangement = Arrangement.spacedBy(10.dp),
                    ) {
                        for (t in rowTitles) {
                            Box(Modifier.weight(1f)) { PosterCard(t, onOpenTitle, Modifier.fillMaxWidth()) }
                        }
                        repeat(3 - rowTitles.size) { Spacer(Modifier.weight(1f)) }
                    }
                }
                if (hasilLoadingMore) {
                    // Halaman berikutnya: satu baris skeleton, bukan
                    // spinner — bentuknya sama dengan yang akan tiba.
                    item { SkeletonPosterRow() }
                }
                // Bagian langsung dari situs sumber (genre terpilih):
                // judul di luar katalog lokal, diketuk = diimpor.
                val hasilIds = hasil.map { it.id }.toSet()
                val liveShown = liveItems.filter { it.catalogId == null || it.catalogId !in hasilIds }
                if (liveLoading && liveItems.isEmpty()) {
                    item {
                        Row(
                            modifier = Modifier.padding(horizontal = 16.dp, vertical = 12.dp),
                            verticalAlignment = Alignment.CenterVertically,
                        ) {
                            CircularProgressIndicator(color = NetflixRed, modifier = Modifier.size(18.dp))
                            Spacer(Modifier.width(10.dp))
                            Text("Mencari langsung di sumber…", color = Color(0xFFB3B3B3))
                        }
                    }
                    items(2) { SkeletonPosterRow() }
                }
                if (liveShown.isNotEmpty()) {
                    item { SectionTitle("Langsung dari $liveSource") }
                    item {
                        Text(
                            "Belum di katalog — ketuk judulnya untuk menambahkan & membuka.",
                            color = Color(0xFFB3B3B3),
                            style = MaterialTheme.typography.bodySmall,
                            modifier = Modifier.padding(horizontal = 16.dp),
                        )
                    }
                    items(liveShown.chunked(3)) { rowItems ->
                        Row(
                            modifier = Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 6.dp),
                            horizontalArrangement = Arrangement.spacedBy(10.dp),
                        ) {
                            for (li in rowItems) {
                                Box(Modifier.weight(1f)) {
                                    LivePosterCard(li, importingSlug == li.slug) { onLiveTap(li) }
                                }
                            }
                            repeat(3 - rowItems.size) { Spacer(Modifier.weight(1f)) }
                        }
                    }
                    if (liveHasMore) {
                        item {
                            Box(
                                modifier = Modifier.fillMaxWidth().padding(vertical = 8.dp),
                                contentAlignment = Alignment.Center,
                            ) {
                                if (liveLoading) {
                                    CircularProgressIndicator(color = NetflixRed, modifier = Modifier.size(22.dp))
                                } else if (liveAutoFailed) {
                                    // Muat otomatis gagal: tombol manual
                                    // sebagai jalan coba ulang.
                                    TextButton(onClick = {
                                        liveAutoFailed = false
                                        loadMoreLive()
                                    }) {
                                        Text("Muat lebih banyak dari $liveSource", color = Color.White)
                                    }
                                }
                                // Selain itu halaman berikutnya dimuat
                                // OTOMATIS oleh pemicu nearEnd saat
                                // pengguna menggulir mendekati ujung —
                                // tanpa tombol, sama seperti Hasil.
                            }
                        }
                    }
                }
            } else {
                val heroList = (trending.ifEmpty { terbaru }).take(5)
                if (heroList.isNotEmpty()) {
                    item {
                        RotatingHeroSection(
                            heroes = heroList,
                            repo = repo,
                            authVersion = authVersion,
                            hiRes = settings.posterHiRes,
                            onPlay = { hero ->
                                scope.launch {
                                    try {
                                        val d = repo.titleDetail(hero.id)
                                        val first = d.episodes.firstOrNull()
                                        if (first != null) {
                                            onOpenEpisode(first.id)
                                        } else {
                                            onOpenTitle(hero.id)
                                        }
                                    } catch (e: Exception) {
                                        onOpenTitle(hero.id)
                                    }
                                }
                            },
                            onInfo = { onOpenTitle(it) },
                            onNeedLogin = onLogin,
                        )
                    }
                }
                // Pintu Premium di tengah Beranda (permintaan Rams
                // 2026-11-23): tombol Premium pindah dari bilah bawah
                // ke sini; bilah bawah tengah kini Jadwal.
                item {
                    Box(
                        modifier = Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 6.dp)
                            .clip(RoundedCornerShape(12.dp))
                            .background(
                                Brush.horizontalGradient(
                                    listOf(Color(0xFF3B2A5D), Color(0xFF7C3AED)),
                                ),
                            )
                            .clickable { onOpenPremium() }
                            .padding(horizontal = 14.dp, vertical = 12.dp),
                    ) {
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            Icon(
                                painter = painterResource(R.drawable.ic_star),
                                contentDescription = null,
                                tint = Color(0xFFFFC107),
                                modifier = Modifier.size(26.dp),
                            )
                            Spacer(Modifier.width(10.dp))
                            Column(modifier = Modifier.weight(1f)) {
                                Text(
                                    "SubSenpai Premium",
                                    color = Color.White,
                                    fontWeight = FontWeight.Bold,
                                    style = MaterialTheme.typography.titleSmall,
                                )
                                Text(
                                    "Buka semua episode anime, donghua & film — tanpa kunci",
                                    color = Color(0xFFE9E4F5),
                                    style = MaterialTheme.typography.bodySmall,
                                )
                            }
                            Text("›", color = Color.White, style = MaterialTheme.typography.headlineSmall)
                        }
                    }
                }
                if (lanjut.isNotEmpty()) {
                    item { SectionTitle("Lanjutkan Menonton", onSeeAll = { onOpenHistory() }) }
                    item {
                        LazyRow(
                            horizontalArrangement = Arrangement.spacedBy(10.dp),
                            modifier = Modifier.padding(horizontal = 16.dp),
                        ) {
                            items(lanjut) { h ->
                                Column(modifier = Modifier.width(120.dp).clickable { onOpenEpisode(h.episodeId) }) {
                                    PosterImage(
                                        url = h.posterUrl,
                                        title = h.title,
                                        modifier = Modifier.fillMaxWidth().aspectRatio(2f / 3f)
                                            .clip(RoundedCornerShape(8.dp)),
                                    )
                                    Text(
                                        h.title,
                                        maxLines = 1,
                                        overflow = TextOverflow.Ellipsis,
                                        style = MaterialTheme.typography.bodyMedium,
                                        modifier = Modifier.padding(top = 6.dp),
                                    )
                                    Text(
                                        "Episode ${h.episodeNo} · ${h.positionSec / 60}:${(h.positionSec % 60).toString().padStart(2, '0')}",
                                        style = MaterialTheme.typography.bodySmall,
                                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                                    )
                                }
                            }
                        }
                    }
                }
                // Urutan rel (28 Nov 2026, permintaan Rams): 10 rel,
                // tiap rel punya Lihat semua — sama seperti web.
                if (homeLive["film"].orEmpty().isNotEmpty() || perType["film"].orEmpty().isNotEmpty()) {
                    item { SectionTitle("Film", onSeeAll = { openBrowse("film", null, null) }) }
                    item {
                        TypeRow(
                            homeLive["film"].orEmpty(), perType["film"].orEmpty(), "film",
                            importingSlug, onOpenTitle, onHomeLiveTap,
                        )
                    }
                }
                if (perTypeYear["film"].orEmpty().isNotEmpty()) {
                    item { SectionTitle("Film Terbaru", onSeeAll = { openBrowse("film", thisYear, null) }) }
                    item { PosterRow(perTypeYear["film"]!!.take(12), onOpenTitle) }
                }
                if (filmPopuler.isNotEmpty()) {
                    item { SectionTitle("Film Populer", onSeeAll = { openBrowse("film", null, null) }) }
                    item { PosterRow(filmPopuler.take(12), onOpenTitle) }
                }
                if (homeLive["anime"].orEmpty().isNotEmpty() || perType["anime"].orEmpty().isNotEmpty()) {
                    item { SectionTitle("Anime", onSeeAll = { openBrowse("anime", null, null) }) }
                    item {
                        TypeRow(
                            homeLive["anime"].orEmpty(), perType["anime"].orEmpty(), "anime",
                            importingSlug, onOpenTitle, onHomeLiveTap,
                        )
                    }
                }
                if (ongoing["anime"].orEmpty().isNotEmpty()) {
                    item { SectionTitle("Anime Ongoing", onSeeAll = { openBrowse("anime", null, "ongoing") }) }
                    item { PosterRow(ongoing["anime"]!!.take(12), onOpenTitle) }
                }
                if (perTypeYear["anime"].orEmpty().isNotEmpty()) {
                    item { SectionTitle("Anime Terbaru", onSeeAll = { openBrowse("anime", thisYear, null) }) }
                    item { PosterRow(perTypeYear["anime"]!!.take(12), onOpenTitle) }
                }
                if (homeLive["donghua"].orEmpty().isNotEmpty() || perType["donghua"].orEmpty().isNotEmpty()) {
                    item { SectionTitle("Donghua", onSeeAll = { openBrowse("donghua", null, null) }) }
                    item {
                        TypeRow(
                            homeLive["donghua"].orEmpty(), perType["donghua"].orEmpty(), "donghua",
                            importingSlug, onOpenTitle, onHomeLiveTap,
                        )
                    }
                }
                if (ongoing["donghua"].orEmpty().isNotEmpty()) {
                    item { SectionTitle("Donghua Ongoing", onSeeAll = { openBrowse("donghua", null, "ongoing") }) }
                    item { PosterRow(ongoing["donghua"]!!.take(12), onOpenTitle) }
                }
                if (perTypeYear["donghua"].orEmpty().isNotEmpty()) {
                    item { SectionTitle("Donghua Terbaru", onSeeAll = { openBrowse("donghua", thisYear, null) }) }
                    item { PosterRow(perTypeYear["donghua"]!!.take(12), onOpenTitle) }
                }
                if (homeLive["series"].orEmpty().isNotEmpty() || perType["series"].orEmpty().isNotEmpty()) {
                    item { SectionTitle("Serial TV", onSeeAll = { openBrowse("series", null, null) }) }
                    item {
                        TypeRow(
                            homeLive["series"].orEmpty(), perType["series"].orEmpty(), "series",
                            importingSlug, onOpenTitle, onHomeLiveTap,
                        )
                    }
                }
                if (perTypeYear["series"].orEmpty().isNotEmpty()) {
                    item { SectionTitle("Serial TV Terbaru", onSeeAll = { openBrowse("series", thisYear, null) }) }
                    item { PosterRow(perTypeYear["series"]!!.take(12), onOpenTitle) }
                }
                if (seriesPopuler.isNotEmpty()) {
                    item { SectionTitle("Serial TV Populer", onSeeAll = { openBrowse("series", null, null) }) }
                    item { PosterRow(seriesPopuler.take(12), onOpenTitle) }
                }
                // Rel TV Langsung (IPTV, 28 Nov 2026): kartu kanal
                // horizontal + "Lihat semua" ke layar TV.
                if (tvChannels.isNotEmpty()) {
                    item { SectionTitle("TV Langsung", onSeeAll = onOpenTV) }
                    item {
                        LazyRow(
                            horizontalArrangement = Arrangement.spacedBy(10.dp),
                            modifier = Modifier.padding(horizontal = 16.dp),
                        ) {
                            items(tvChannels.take(12)) { ch ->
                                Box(Modifier.width(160.dp)) {
                                    TVChannelCard(ch, onOpenChannel, Modifier.fillMaxWidth())
                                }
                            }
                        }
                    }
                }
                item { Spacer(Modifier.height(24.dp)) }
            }
        }

        // Header melayang ala Netflix: transparan bergradasi di atas
        // hero; menjadi pekat saat menjelajah/mencari agar teks daftar
        // di bawahnya tetap terbaca.
        Column(
            modifier = Modifier.fillMaxWidth().background(
                if (browsing || searchOpen) {
                    Brush.verticalGradient(listOf(Color(0xF2141414), Color(0xF2141414)))
                } else {
                    Brush.verticalGradient(listOf(Color(0xB3000000), Color.Transparent))
                }
            ),
        ) {
            if (searchOpen) {
                // Bilah cari ala Google (permintaan Rams 2026-11-23):
                // pil melengkung penuh di posisi PALING ATAS menggantikan
                // baris ikon — kolom cari lama di bawah tab dihapus.
                val searchFocus = remember { FocusRequester() }
                LaunchedEffect(Unit) { searchFocus.requestFocus() }
                Row(
                    modifier = Modifier.fillMaxWidth().padding(horizontal = 12.dp, vertical = 8.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Row(
                        modifier = Modifier.weight(1f).height(46.dp)
                            .background(Color(0xFF2B2B2B), RoundedCornerShape(50))
                            .padding(horizontal = 14.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Icon(
                            painter = painterResource(R.drawable.ic_search),
                            contentDescription = null,
                            tint = Color(0xFFB3B3B3),
                            modifier = Modifier.size(20.dp),
                        )
                        Spacer(Modifier.width(10.dp))
                        BasicTextField(
                            value = query,
                            onValueChange = { query = it },
                            singleLine = true,
                            textStyle = TextStyle(color = Color.White, fontSize = 16.sp),
                            cursorBrush = SolidColor(Color.White),
                            modifier = Modifier.weight(1f).focusRequester(searchFocus),
                            decorationBox = { inner ->
                                if (query.isEmpty()) {
                                    Text(
                                        "Cari judul, genre…",
                                        color = Color(0xFF9E9E9E),
                                        fontSize = 16.sp,
                                    )
                                }
                                inner()
                            },
                        )
                        if (query.isNotEmpty()) {
                            IconButton(
                                onClick = { query = "" },
                                modifier = Modifier.size(32.dp),
                            ) {
                                Text("✕", color = Color(0xFFB3B3B3), fontSize = 15.sp)
                            }
                        }
                    }
                    TextButton(onClick = {
                        searchOpen = false
                        query = ""
                    }) {
                        Text("Batal", color = Color.White)
                    }
                }
            } else {
            Row(
                modifier = Modifier.fillMaxWidth().padding(horizontal = 8.dp, vertical = 4.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text(
                    "S",
                    color = NetflixRed,
                    fontSize = 36.sp,
                    fontWeight = FontWeight.Black,
                    modifier = Modifier.padding(start = 8.dp),
                )
                Spacer(Modifier.weight(1f))
                IconButton(onClick = {
                    searchOpen = !searchOpen
                    if (!searchOpen) query = ""
                }) {
                    Icon(
                        painter = painterResource(R.drawable.ic_search),
                        contentDescription = "Cari",
                        tint = Color.White,
                    )
                }
                Box {
                    IconButton(onClick = onOpenNotices) {
                        Icon(
                            painter = painterResource(R.drawable.ic_bell),
                            contentDescription = "Notifikasi",
                            tint = Color.White,
                        )
                    }
                    if (notifUnread > 0) {
                        Box(
                            modifier = Modifier.align(Alignment.TopEnd).padding(4.dp)
                                .background(NetflixRed, CircleShape)
                                .padding(horizontal = 5.dp, vertical = 1.dp),
                        ) {
                            Text(
                                notifUnread.coerceAtMost(99).toString(),
                                color = Color.White,
                                fontSize = 10.sp,
                                fontWeight = FontWeight.Bold,
                            )
                        }
                    }
                }
                IconButton(onClick = onOpenMore) {
                    Icon(
                        painter = painterResource(R.drawable.ic_person),
                        contentDescription = "Lainnya",
                        tint = Color.White,
                    )
                }
            }
            }
            Row(
                modifier = Modifier.fillMaxWidth().padding(horizontal = 16.dp),
                horizontalArrangement = Arrangement.spacedBy(24.dp, Alignment.CenterHorizontally),
            ) {
                typeFilters.forEach { (value, label) ->
                    Text(
                        label,
                        color = if (type == value) Color.White else Color(0xFFB3B3B3),
                        fontWeight = if (type == value) FontWeight.Bold else FontWeight.Normal,
                        style = MaterialTheme.typography.bodyLarge,
                        modifier = Modifier.clickable {
                            type = value
                            genre = null
                            year = null
                            country = null
                            network = null
                            status = null
                        }.padding(bottom = 10.dp),
                    )
                }
            }
        }
        if (filterOpen && facets != null) {
            ModalBottomSheet(onDismissRequest = { filterOpen = false }) {
                FilterSheetContent(
                    facets = facets!!,
                    genre = genre,
                    year = year,
                    country = country,
                    network = network,
                    status = status,
                    showStatus = type == "anime" || type == "donghua" || type == "series",
                    onGenre = { genre = it },
                    onYear = { year = it },
                    onCountry = { country = it },
                    onNetwork = { network = it },
                    onStatus = { status = it },
                    onReset = {
                        genre = null
                        year = null
                        country = null
                        network = null
                        status = null
                    },
                    onClose = { filterOpen = false },
                )
            }
        }
        // Muat Beranda menggantung >10 detik: tombol "Coba lagi" di
        // bawah layar — menaikkan reloadKey membatalkan muat yang
        // menggantung dan memulai ulang dari awal.
        if (showHomeRetry && !homeLoaded && fatalKind == null) {
            Column(
                modifier = Modifier.align(Alignment.BottomCenter)
                    .padding(bottom = 32.dp),
                horizontalAlignment = Alignment.CenterHorizontally,
            ) {
                Text(
                    "Masih memuat — koneksi tampaknya lambat.",
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
                Spacer(Modifier.height(8.dp))
                Button(
                    onClick = {
                        showHomeRetry = false
                        reloadKey++
                    },
                    colors = ButtonDefaults.buttonColors(containerColor = NetflixRed),
                ) { Text("Coba lagi") }
            }
        }
    }
}

/**
 * Panel filter lanjutan ala "Advance Filter" hashflix (permintaan
 * Rams 2026-11-23): genre, tahun rilis, negara (region), dan
 * network/perusahaan — semua pilihan dihitung backend dari katalog
 * sendiri. Pilihan langsung diterapkan; ketuk chip yang sama untuk
 * membatalkan pilihan itu.
 */
@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun FilterSheetContent(
    facets: TitleFacets,
    genre: String?,
    year: Int?,
    country: String?,
    network: String?,
    status: String? = null,
    showStatus: Boolean = false,
    onGenre: (String?) -> Unit,
    onYear: (Int?) -> Unit,
    onCountry: (String?) -> Unit,
    onNetwork: (String?) -> Unit,
    onStatus: (String?) -> Unit = {},
    onReset: () -> Unit,
    onClose: () -> Unit,
) {
    Column(
        modifier = Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 8.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        Text("Filter", style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.Bold)
        Text(
            "Saring judul berdasarkan genre, tahun, negara, dan lainnya. Pilihan langsung diterapkan.",
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
        if (facets.genres.isNotEmpty()) {
            Text("Genre", fontWeight = FontWeight.Bold)
            FlowRow(
                horizontalArrangement = Arrangement.spacedBy(8.dp),
                verticalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                facets.genres.forEach { g ->
                    GenreChip(g, genre == g) { onGenre(if (genre == g) null else g) }
                }
            }
        }
        if (facets.years.isNotEmpty()) {
            Text("Tahun Rilis", fontWeight = FontWeight.Bold)
            FlowRow(
                horizontalArrangement = Arrangement.spacedBy(8.dp),
                verticalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                facets.years.forEach { y ->
                    GenreChip(y.toString(), year == y) { onYear(if (year == y) null else y) }
                }
            }
        }
        if (facets.countries.isNotEmpty()) {
            Text("Negara", fontWeight = FontWeight.Bold)
            FlowRow(
                horizontalArrangement = Arrangement.spacedBy(8.dp),
                verticalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                facets.countries.forEach { c ->
                    GenreChip(c.name, country == c.value) {
                        onCountry(if (country == c.value) null else c.value)
                    }
                }
            }
        }
        if (facets.networks.isNotEmpty()) {
            Text("Network / Perusahaan", fontWeight = FontWeight.Bold)
            FlowRow(
                horizontalArrangement = Arrangement.spacedBy(8.dp),
                verticalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                facets.networks.forEach { n ->
                    GenreChip(n, network == n) { onNetwork(if (network == n) null else n) }
                }
            }
        }
        if (showStatus) {
            Text("Status", fontWeight = FontWeight.Bold)
            FlowRow(
                horizontalArrangement = Arrangement.spacedBy(8.dp),
                verticalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                GenreChip("Ongoing", status == "ongoing") {
                    onStatus(if (status == "ongoing") null else "ongoing")
                }
                GenreChip("Selesai", status == "completed") {
                    onStatus(if (status == "completed") null else "completed")
                }
            }
        }
        Row(
            horizontalArrangement = Arrangement.spacedBy(10.dp),
            modifier = Modifier.fillMaxWidth().padding(bottom = 20.dp, top = 4.dp),
        ) {
            OutlinedButton(onClick = onReset, modifier = Modifier.weight(1f)) { Text("Reset") }
            Button(
                onClick = onClose,
                modifier = Modifier.weight(1f),
                colors = ButtonDefaults.buttonColors(containerColor = NetflixRed),
            ) { Text("Terapkan") }
        }
    }
}

/** Chip filter genre di mode telusur (dipilih = merah Netflix). */
@Composable
private fun GenreChip(label: String, selected: Boolean, onClick: () -> Unit) {
    Box(
        modifier = Modifier
            .clip(RoundedCornerShape(16.dp))
            .background(if (selected) NetflixRed else Color(0xFF2A2A2A))
            .clickable { onClick() }
            .padding(horizontal = 14.dp, vertical = 7.dp),
    ) {
        Text(label, color = Color.White, style = MaterialTheme.typography.bodyMedium)
    }
}

/**
 * URL poster resolusi lebih tinggi untuk permukaan besar (hero
 * Beranda & kepala halaman detail): poster film TMDB disimpan
 * sebagai w500 (500px) yang terlihat burik saat direntangkan
 * selebar layar, jadi khusus permukaan besar diganti varian w780.
 * Kartu-kartu kecil tetap memakai w500 yang ringan. Sumber lain
 * (Kuronime/Anichin) tidak punya varian lebih besar — berkas
 * aslinya memang kecil — jadi URL-nya dikembalikan apa adanya.
 */
fun hiResPosterUrl(url: String?): String? =
    url?.replace("image.tmdb.org/t/p/w500", "image.tmdb.org/t/p/w780")

/** Hero layar penuh ala Netflix dengan aksi Daftar Saya/Putar/Info. */
// RotatingHeroSection: poster utama Beranda BERGANTI tiap 8 detik
// (permintaan Rams 2026-11-23 — biar tidak membosankan), berputar
// di antara 5 judul tren teratas dengan pudar silang; tombol Putar,
// Info, dan Daftar Saya selalu mengikuti judul yang sedang tampil.
@Composable
private fun RotatingHeroSection(
    heroes: List<TitleCard>,
    repo: CatalogRepository,
    authVersion: Int,
    hiRes: Boolean,
    onPlay: (TitleCard) -> Unit,
    onInfo: (String) -> Unit,
    onNeedLogin: () -> Unit,
) {
    var idx by remember { mutableStateOf(0) }
    LaunchedEffect(heroes.map { it.id }) {
        idx = 0
        while (heroes.size > 1) {
            delay(8000)
            idx = (idx + 1) % heroes.size
        }
    }
    val hero = heroes.getOrNull(idx) ?: heroes.firstOrNull() ?: return
    Crossfade(targetState = hero, animationSpec = tween(700), label = "hero-rotate") { h ->
        HeroSection(
            hero = h,
            repo = repo,
            authVersion = authVersion,
            hiRes = hiRes,
            onPlay = { onPlay(h) },
            onInfo = { onInfo(h.id) },
            onNeedLogin = onNeedLogin,
        )
    }
}

@Composable
private fun HeroSection(
    hero: TitleCard,
    repo: CatalogRepository,
    authVersion: Int,
    hiRes: Boolean,
    onPlay: () -> Unit,
    onInfo: () -> Unit,
    onNeedLogin: () -> Unit,
) {
    var inList by remember(hero.id) { mutableStateOf(false) }
    val scope = rememberCoroutineScope()
    LaunchedEffect(hero.id, authVersion) {
        inList = if (repo.isLoggedIn) {
            try {
                repo.watchlistCheck(hero.id)
            } catch (e: Exception) {
                false
            }
        } else {
            false
        }
    }
    Box(modifier = Modifier.fillMaxWidth().height(540.dp)) {
        PosterImage(
            url = if (hiRes) hiResPosterUrl(hero.posterUrl) else hero.posterUrl,
            title = hero.title,
            modifier = Modifier.fillMaxSize(),
            wide = true,
        )
        Box(
            modifier = Modifier.fillMaxSize().background(
                Brush.verticalGradient(
                    colors = listOf(
                        Color.Transparent,
                        Color.Transparent,
                        MaterialTheme.colorScheme.background,
                    ),
                    startY = 300f,
                )
            )
        )
        Column(
            modifier = Modifier.align(Alignment.BottomCenter).fillMaxWidth()
                .padding(horizontal = 24.dp, vertical = 16.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            Text(
                hero.title,
                style = MaterialTheme.typography.headlineLarge,
                fontWeight = FontWeight.Black,
                color = Color.White,
                textAlign = TextAlign.Center,
                maxLines = 2,
                overflow = TextOverflow.Ellipsis,
            )
            Text(
                listOfNotNull(
                    hero.type.replaceFirstChar { it.uppercase() },
                    hero.genres.joinToString(" • ").ifBlank { null },
                ).joinToString("  •  "),
                style = MaterialTheme.typography.bodyMedium,
                color = Color(0xFFDDDDDD),
                textAlign = TextAlign.Center,
                modifier = Modifier.padding(vertical = 10.dp),
            )
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceEvenly,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                HeroAction(
                    icon = if (inList) R.drawable.ic_check else R.drawable.ic_add,
                    label = "Daftar Saya",
                    onClick = {
                        if (!repo.isLoggedIn) {
                            onNeedLogin()
                        } else {
                            scope.launch {
                                try {
                                    if (inList) {
                                        repo.watchlistRemove(hero.id)
                                        inList = false
                                    } else {
                                        repo.watchlistAdd(hero.id)
                                        inList = true
                                    }
                                } catch (e: Exception) {
                                    // Abaikan — status dibaca ulang saat hero berganti.
                                }
                            }
                        }
                    },
                )
                Button(
                    onClick = onPlay,
                    colors = ButtonDefaults.buttonColors(
                        containerColor = Color.White,
                        contentColor = Color.Black,
                    ),
                    shape = RoundedCornerShape(6.dp),
                    modifier = Modifier.height(46.dp).width(160.dp),
                ) {
                    Icon(
                        painter = painterResource(R.drawable.ic_play),
                        contentDescription = null,
                        tint = Color.Black,
                        modifier = Modifier.size(22.dp),
                    )
                    Spacer(Modifier.width(6.dp))
                    Text("Putar", fontWeight = FontWeight.Bold, fontSize = 17.sp)
                }
                HeroAction(
                    icon = R.drawable.ic_info,
                    label = "Info",
                    onClick = onInfo,
                )
            }
        }
    }
}

@Composable
private fun HeroAction(icon: Int, label: String, onClick: () -> Unit) {
    Column(
        horizontalAlignment = Alignment.CenterHorizontally,
        modifier = Modifier.clickable { onClick() }.padding(horizontal = 8.dp, vertical = 4.dp),
    ) {
        Icon(
            painter = painterResource(icon),
            contentDescription = label,
            tint = Color.White,
            modifier = Modifier.size(26.dp),
        )
        Text(
            label,
            color = Color.White,
            style = MaterialTheme.typography.labelMedium,
            modifier = Modifier.padding(top = 2.dp),
        )
    }
}

@Composable
private fun SectionTitle(text: String, onSeeAll: (() -> Unit)? = null) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(start = 16.dp, end = 16.dp, top = 20.dp, bottom = 10.dp),
        horizontalArrangement = Arrangement.SpaceBetween,
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(
            text,
            style = MaterialTheme.typography.titleLarge,
            fontWeight = FontWeight.Bold,
        )
        if (onSeeAll != null) {
            Text(
                "Lihat semua ›",
                style = MaterialTheme.typography.bodyMedium,
                fontWeight = FontWeight.SemiBold,
                color = Color(0xFFA1A1AA),
                modifier = Modifier.clickable { onSeeAll() },
            )
        }
    }
}

/**
 * Baris jenis di Beranda: dari BERANDA situs sumber bila ada,
 * kalau tidak katalog (perilaku lama sejak 23 Nov 2026).
 */
@Composable
private fun TypeRow(
    liveRows: List<CatalogRepository.LiveBrowseItem>,
    rows: List<TitleCard>,
    type: String,
    importingSlug: String?,
    onOpenTitle: (String) -> Unit,
    onHomeLiveTap: (String, CatalogRepository.LiveBrowseItem) -> Unit,
) {
    if (liveRows.isNotEmpty()) {
        LazyRow(
            horizontalArrangement = Arrangement.spacedBy(10.dp),
            modifier = Modifier.padding(horizontal = 16.dp),
        ) {
            items(liveRows) { li ->
                Box(Modifier.width(112.dp)) {
                    LivePosterCard(li, importingSlug == li.slug) { onHomeLiveTap(type, li) }
                }
            }
        }
    } else if (rows.isNotEmpty()) {
        PosterRow(rows, onOpenTitle)
    }
}

@Composable
private fun PosterRow(list: List<TitleCard>, onOpen: (String) -> Unit) {
    LazyRow(
        horizontalArrangement = Arrangement.spacedBy(10.dp),
        modifier = Modifier.padding(horizontal = 16.dp),
    ) {
        items(list) { PosterCard(it, onOpen, Modifier.width(112.dp)) }
    }
}

/**
 * Gambar poster; bila URL kosong (judul hasil penemuan yang sumbernya
 * tidak menyediakan poster yang bisa dijangkau), tampilkan plakat judul
 * bergaya Netflix agar kisi tetap terlihat utuh.
 */
@Composable
fun PosterImage(url: String?, title: String, modifier: Modifier = Modifier, wide: Boolean = false) {
    var gagal by remember(url) { mutableStateOf(false) }
    if (url.isNullOrBlank() || gagal) {
        Box(
            modifier = modifier.background(
                Brush.verticalGradient(listOf(Color(0xFF4A0E12), Color(0xFF1B1B1B)))
            ),
            contentAlignment = Alignment.Center,
        ) {
            Text(
                title.take(30),
                color = Color.White,
                fontWeight = FontWeight.Bold,
                textAlign = TextAlign.Center,
                style = MaterialTheme.typography.titleSmall,
                modifier = Modifier.padding(10.dp),
            )
        }
    } else {
        // Dua lapis perlindungan sumber poster: (1) sebagian CDN
        // menolak User-Agent bawaan OkHttp — kirim UA peramban;
        // (2) anichin.moe dan situs sejenis memasang perlindungan
        // hotlink: berkas gambarnya hanya dilayani bila header
        // Referer menunjuk situsnya sendiri (terbukti: tanpa Referer
        // 403 dari Cloudflare, dengan Referer situs asal 200).
        // Referer diturunkan dari URL gambar itu sendiri, jadi
        // berlaku umum untuk semua sumber. Kegagalan muat jatuh
        // kembali ke plakat judul.
        val referer = try {
            val u = java.net.URL(url)
            "${u.protocol}://${u.host}/"
        } catch (e: Exception) {
            null
        }
        AsyncImage(
            model = ImageRequest.Builder(LocalContext.current)
                .data(url)
                // Batasi ukuran dekode (v0.24.12): poster sumber
                // bisa 2000px+ — mendekode ukuran penuh untuk sel
                // grid kecil memboroskan RAM puluhan kali lipat
                // dan membuat gulir tersendat sampai aplikasi
                // tidak merespons. Kartu cukup 540px; hero dan
                // latar detail (wide) 1080px.
                .size(if (wide) 1080 else 540, if (wide) 1620 else 810)
                .addHeader(
                    "User-Agent",
                    "Mozilla/5.0 (Linux; Android 13) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Mobile Safari/537.36",
                )
                .apply { if (referer != null) addHeader("Referer", referer) }
                .crossfade(true)
                .build(),
            contentDescription = title,
            contentScale = ContentScale.Crop,
            onError = { gagal = true },
            modifier = modifier,
        )
    }
}

/**
 * Kartu poster kerangka — SKELETON LOADING ala YouTube/Netflix
 * (permintaan Rams 2026-11-24): bayangan poster + dua garis judul
 * yang berdenyut (shimmer) selama data asli belum tiba; begitu
 * termuat, konten asli menggantikannya. Bentuknya meniru persis
 * PosterCard (poster 2:3 sudut 8dp + judul di bawah) agar tata
 * letak tidak meloncat saat berganti.
 */
@Composable
private fun SkeletonPosterCard(modifier: Modifier = Modifier) {
    val transition = rememberInfiniteTransition(label = "skeleton")
    val alpha by transition.animateFloat(
        initialValue = 0.4f,
        targetValue = 1f,
        animationSpec = infiniteRepeatable(
            animation = tween(durationMillis = 850),
            repeatMode = RepeatMode.Reverse,
        ),
        label = "skeletonAlpha",
    )
    val base = Color(0xFF2B2B30)
    Column(modifier = modifier) {
        Box(
            modifier = Modifier.fillMaxWidth().aspectRatio(2f / 3f)
                .clip(RoundedCornerShape(8.dp)).background(base.copy(alpha = alpha)),
        )
        Spacer(Modifier.height(10.dp))
        Box(
            modifier = Modifier.fillMaxWidth(0.85f).height(12.dp)
                .clip(RoundedCornerShape(6.dp)).background(base.copy(alpha = alpha)),
        )
        Spacer(Modifier.height(5.dp))
        Box(
            modifier = Modifier.fillMaxWidth(0.55f).height(12.dp)
                .clip(RoundedCornerShape(6.dp)).background(base.copy(alpha = alpha)),
        )
    }
}

/** Satu baris grid skeleton: 3 kartu, tata letak sama dengan
 *  baris Hasil/bagian live. */
@Composable
private fun SkeletonPosterRow() {
    Row(
        modifier = Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 6.dp),
        horizontalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        repeat(3) {
            Box(Modifier.weight(1f)) { SkeletonPosterCard(Modifier.fillMaxWidth()) }
        }
    }
}

@Composable
private fun PosterCard(t: TitleCard, onOpen: (String) -> Unit, modifier: Modifier = Modifier) {
    Column(modifier = modifier.clickable { onOpen(t.id) }) {
        PosterImage(
            url = t.posterUrl,
            title = t.title,
            modifier = Modifier.fillMaxWidth().aspectRatio(2f / 3f).clip(RoundedCornerShape(8.dp)),
        )
        Text(
            t.title,
            maxLines = 2,
            overflow = TextOverflow.Ellipsis,
            style = MaterialTheme.typography.bodySmall,
            modifier = Modifier.padding(top = 6.dp),
        )
    }
}

/** Kartu poster hasil telusur langsung: sama seperti PosterCard,
 *  dengan penanda putar saat judul sedang diimpor ke katalog. */
@Composable
private fun LivePosterCard(
    item: CatalogRepository.LiveBrowseItem,
    importing: Boolean,
    onTap: () -> Unit,
) {
    Column(modifier = Modifier.clickable(enabled = !importing) { onTap() }) {
        Box {
            PosterImage(
                url = item.posterUrl,
                title = item.title,
                modifier = Modifier.fillMaxWidth().aspectRatio(2f / 3f).clip(RoundedCornerShape(8.dp)),
            )
            if (importing) {
                Box(
                    modifier = Modifier.matchParentSize().background(Color(0x99000000)),
                    contentAlignment = Alignment.Center,
                ) {
                    CircularProgressIndicator(color = Color.White, modifier = Modifier.size(26.dp))
                }
            }
        }
        Text(
            item.title,
            maxLines = 2,
            overflow = TextOverflow.Ellipsis,
            style = MaterialTheme.typography.bodySmall,
            modifier = Modifier.padding(top = 6.dp),
        )
    }
}

/** Baris daftar klasik — dipakai WatchlistScreen. */
@Composable
fun TitleRow(t: TitleCard, onOpen: (String) -> Unit) {
    Card(modifier = Modifier.fillMaxWidth().clickable { onOpen(t.id) }) {
        Row(modifier = Modifier.padding(12.dp)) {
            PosterImage(
                url = t.posterUrl,
                title = t.title,
                modifier = Modifier.width(56.dp).aspectRatio(2f / 3f).clip(RoundedCornerShape(6.dp)),
            )
            Spacer(Modifier.width(12.dp))
            Column {
                Text(t.title, style = MaterialTheme.typography.titleMedium, maxLines = 1, overflow = TextOverflow.Ellipsis)
                Text(
                    listOfNotNull(
                        t.type.replaceFirstChar { it.uppercase() },
                        t.year?.toString(),
                        t.rating?.let { "★ $it" },
                        t.genres.joinToString(", ").ifBlank { null },
                    ).joinToString(" · "),
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
        }
    }
}
