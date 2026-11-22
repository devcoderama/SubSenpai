package com.subsenpai.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
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
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.Card
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
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
import kotlinx.coroutines.launch

private val typeFilters = listOf(
    null to "Semua", "anime" to "Anime", "donghua" to "Donghua", "film" to "Film",
)

private val rowTypes = listOf(
    "anime" to "Anime Pilihan", "donghua" to "Donghua Pilihan", "film" to "Film Pilihan",
)

/**
 * Beranda ala Netflix (redesain 2026-11-22 atas contoh UI Netflix
 * dari Rams): header melayang transparan di atas hero — logo "S"
 * merah, ikon cari & akun, tab kategori teks (Semua/Anime/Donghua/
 * Film) — hero layar penuh dengan baris aksi Daftar Saya / Putar /
 * Info, lalu baris-baris poster. Navigasi utama pindah ke bilah
 * bawah di MainActivity.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun HomeScreen(
    repo: CatalogRepository,
    authVersion: Int,
    onOpenTitle: (String) -> Unit,
    onOpenEpisode: (String) -> Unit,
    onLogin: () -> Unit,
    onOpenMore: () -> Unit,
) {
    val homeContext = LocalContext.current
    val settings = remember { SettingsStore(homeContext) }
    var trending by remember { mutableStateOf<List<TitleCard>>(emptyList()) }
    var homeLoaded by remember { mutableStateOf(false) }
    var terbaru by remember { mutableStateOf<List<TitleCard>>(emptyList()) }
    var perType by remember { mutableStateOf<Map<String, List<TitleCard>>>(emptyMap()) }
    var hasil by remember { mutableStateOf<List<TitleCard>>(emptyList()) }
    var lanjut by remember { mutableStateOf<List<com.subsenpai.data.WatchHistoryItem>>(emptyList()) }
    var error by remember { mutableStateOf<String?>(null) }
    var type by remember { mutableStateOf<String?>(null) }
    var query by remember { mutableStateOf("") }
    var genre by remember { mutableStateOf<String?>(null) }
    var genreList by remember { mutableStateOf<List<String>>(emptyList()) }
    var year by remember { mutableStateOf<Int?>(null) }
    var country by remember { mutableStateOf<String?>(null) }
    var network by remember { mutableStateOf<String?>(null) }
    var facets by remember { mutableStateOf<TitleFacets?>(null) }
    var filterOpen by remember { mutableStateOf(false) }
    var searchOpen by remember { mutableStateOf(false) }
    val browsing = type != null || query.isNotBlank()
    val scope = rememberCoroutineScope()

    LaunchedEffect(Unit) {
        try {
            val (tr, te) = repo.home()
            trending = tr
            terbaru = te
        } catch (e: Exception) {
            error = e.message
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
    LaunchedEffect(type, query, genre, year, country, network) {
        if (!browsing) return@LaunchedEffect
        try {
            hasil = repo.titles(type, query.ifBlank { null }, genre, year, country, network)
        } catch (e: Exception) {
            error = e.message
        }
    }

    Box(modifier = Modifier.fillMaxSize()) {
        LazyColumn(modifier = Modifier.fillMaxSize()) {
            if (browsing) {
                // Ruang untuk header melayang (lebih tinggi saat kolom
                // cari terbuka).
                item { Spacer(Modifier.height(if (searchOpen) 208.dp else 140.dp)) }
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
                        "Gagal memuat: $error",
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
                            val extraActive = listOf(year, country, network).count { it != null }
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
                if (hasil.isEmpty() && error == null) {
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
            } else {
                val hero = trending.firstOrNull() ?: terbaru.firstOrNull()
                if (hero != null) {
                    item {
                        HeroSection(
                            hero = hero,
                            repo = repo,
                            authVersion = authVersion,
                            hiRes = settings.posterHiRes,
                            onPlay = {
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
                            onInfo = { onOpenTitle(hero.id) },
                            onNeedLogin = onLogin,
                        )
                    }
                }
                if (lanjut.isNotEmpty()) {
                    item { SectionTitle("Lanjutkan Menonton") }
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
                if (trending.isNotEmpty()) {
                    item { SectionTitle("Sedang Tren") }
                    item { PosterRow(trending, onOpenTitle) }
                }
                for ((t, label) in rowTypes) {
                    val rows = perType[t].orEmpty()
                    if (rows.isNotEmpty()) {
                        item { SectionTitle(label) }
                        item { PosterRow(rows, onOpenTitle) }
                    }
                }
                if (terbaru.isNotEmpty()) {
                    item { SectionTitle("Baru Ditambahkan") }
                    item { PosterRow(terbaru, onOpenTitle) }
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
                IconButton(onClick = onOpenMore) {
                    Icon(
                        painter = painterResource(R.drawable.ic_person),
                        contentDescription = "Lainnya",
                        tint = Color.White,
                    )
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
                        }.padding(bottom = 10.dp),
                    )
                }
            }
            if (searchOpen) {
                OutlinedTextField(
                    value = query,
                    onValueChange = { query = it },
                    label = { Text("Cari judul, genre…") },
                    singleLine = true,
                    modifier = Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 8.dp),
                )
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
                    onGenre = { genre = it },
                    onYear = { year = it },
                    onCountry = { country = it },
                    onNetwork = { network = it },
                    onReset = {
                        genre = null
                        year = null
                        country = null
                        network = null
                    },
                    onClose = { filterOpen = false },
                )
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
    onGenre: (String?) -> Unit,
    onYear: (Int?) -> Unit,
    onCountry: (String?) -> Unit,
    onNetwork: (String?) -> Unit,
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
private fun SectionTitle(text: String) {
    Text(
        text,
        style = MaterialTheme.typography.titleLarge,
        fontWeight = FontWeight.Bold,
        modifier = Modifier.padding(start = 16.dp, end = 16.dp, top = 20.dp, bottom = 10.dp),
    )
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
fun PosterImage(url: String?, title: String, modifier: Modifier = Modifier) {
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
