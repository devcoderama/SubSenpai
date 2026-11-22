package com.subsenpai.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.Card
import androidx.compose.material3.FilterChip
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import coil.compose.AsyncImage
import coil.request.ImageRequest
import androidx.compose.ui.platform.LocalContext
import com.subsenpai.data.CatalogRepository
import com.subsenpai.data.TitleCard
import kotlinx.coroutines.launch

private val typeFilters = listOf(
    null to "Semua", "anime" to "Anime", "donghua" to "Donghua", "film" to "Film",
    "manga" to "Manga", "manhua" to "Manhua", "manhwa" to "Manhwa",
)

private val rowTypes = listOf(
    "anime" to "Anime Pilihan", "donghua" to "Donghua Pilihan", "film" to "Film Pilihan",
)

private val comicRowTypes = listOf(
    "manga" to "Manga Pilihan", "manhwa" to "Manhwa Pilihan", "manhua" to "Manhua Pilihan",
)

private val comicTypeSet = setOf("manga", "manhua", "manhwa")

@Composable
fun HomeScreen(
    repo: CatalogRepository,
    authVersion: Int,
    onOpenTitle: (String) -> Unit,
    onOpenEpisode: (String) -> Unit,
    onLogin: () -> Unit,
    onOpenWatchlist: () -> Unit,
    onOpenPremium: () -> Unit = {},
    onOpenChat: () -> Unit = {},
    onOpenComic: (com.subsenpai.data.ComicSeries) -> Unit = {},
) {
    var trending by remember { mutableStateOf<List<TitleCard>>(emptyList()) }
    var homeLoaded by remember { mutableStateOf(false) }
    var terbaru by remember { mutableStateOf<List<TitleCard>>(emptyList()) }
    var perType by remember { mutableStateOf<Map<String, List<TitleCard>>>(emptyMap()) }
    var hasil by remember { mutableStateOf<List<TitleCard>>(emptyList()) }
    var lanjut by remember { mutableStateOf<List<com.subsenpai.data.WatchHistoryItem>>(emptyList()) }
    var error by remember { mutableStateOf<String?>(null) }
    var type by remember { mutableStateOf<String?>(null) }
    var query by remember { mutableStateOf("") }
    val browsing = type != null || query.isNotBlank()
    val loggedIn = repo.isLoggedIn

    // Umpan komik perangkat (API manhuarmtl langsung dari aplikasi,
    // pola Mihon) — mengisi baris & kisi manga/manhua/manhwa di sini.
    val scope = androidx.compose.runtime.rememberCoroutineScope()
    var comicRaw by remember { mutableStateOf<List<com.subsenpai.data.ComicSeries>>(emptyList()) }
    var comicPage by remember { mutableStateOf(0) }
    var comicTotal by remember { mutableStateOf(1) }
    var comicLoading by remember { mutableStateOf(false) }
    var comicSearch by remember { mutableStateOf<List<com.subsenpai.data.ComicSeries>>(emptyList()) }
    val comicMode = type in comicTypeSet

    suspend fun loadComics(minEach: Int) {
        if (comicLoading) return
        comicLoading = true
        try {
            var fetched = 0
            while (fetched < 8 && comicPage < comicTotal) {
                val counts = comicTypeSet.associateWith { t -> comicRaw.count { it.type == t } }
                if (fetched > 0 && counts.values.all { it >= minEach }) break
                val (items, total) = com.subsenpai.data.ManhuarmtlApi.latest(comicPage + 1)
                comicTotal = total
                comicPage += 1
                fetched++
                val known = comicRaw.map { it.slug }.toSet()
                comicRaw = comicRaw + items.filter { it.slug !in known }
            }
        } catch (e: Exception) {
            // Baris komik dilewati bila sumber gagal dijangkau.
        } finally {
            comicLoading = false
        }
    }

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
    LaunchedEffect(type, query) {
        if (!browsing) return@LaunchedEffect
        if (comicMode) {
            if (query.isNotBlank()) {
                try {
                    comicSearch = com.subsenpai.data.ManhuarmtlApi.search(query, 1).first
                } catch (e: Exception) {
                    comicSearch = emptyList()
                }
            } else if (comicRaw.none { it.type == type }) {
                loadComics(12)
            }
            return@LaunchedEffect
        }
        try {
            hasil = repo.titles(type, query.ifBlank { null })
        } catch (e: Exception) {
            error = e.message
        }
    }
    LaunchedEffect(Unit) { loadComics(10) }

    LazyColumn(modifier = Modifier.fillMaxSize()) {
        item {
            Row(
                modifier = Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 10.dp),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text(
                    "SUBSENPAI",
                    color = MaterialTheme.colorScheme.primary,
                    style = MaterialTheme.typography.headlineSmall,
                    fontWeight = FontWeight.Black,
                )
                Row {
                    TextButton(onClick = onOpenPremium) { Text("Premium", color = Color.White) }
                    if (loggedIn) {
                        TextButton(onClick = onOpenChat) { Text("Chat", color = Color.White) }
                        TextButton(onClick = onOpenWatchlist) { Text("Watchlist", color = Color.White) }
                    }
                    TextButton(onClick = onLogin) { Text(if (loggedIn) "Akun" else "Masuk", color = Color.White) }
                }
            }
            OutlinedTextField(
                value = query,
                onValueChange = { query = it },
                label = { Text("Cari judul, genre…") },
                singleLine = true,
                modifier = Modifier.fillMaxWidth().padding(horizontal = 16.dp),
            )
            Spacer(Modifier.height(10.dp))
            LazyRow(
                horizontalArrangement = Arrangement.spacedBy(8.dp),
                modifier = Modifier.padding(horizontal = 16.dp),
            ) {
                items(typeFilters) { (value, label) ->
                    FilterChip(
                        selected = type == value,
                        onClick = { type = value },
                        label = { Text(label) },
                    )
                }
            }
            Spacer(Modifier.height(8.dp))
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
        if (browsing && comicMode) {
            val comicItems = (if (query.isNotBlank()) comicSearch else comicRaw)
                .filter { it.type == type }
            item { SectionTitle("Hasil") }
            if (comicItems.isEmpty()) {
                item {
                    Text(
                        if (comicLoading) "Memuat dari sumber komik…" else "Belum ada judul yang cocok.",
                        modifier = Modifier.padding(horizontal = 16.dp),
                    )
                }
            }
            items(comicItems.chunked(3)) { rowSeries ->
                Row(
                    modifier = Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 6.dp),
                    horizontalArrangement = Arrangement.spacedBy(10.dp),
                ) {
                    for (s in rowSeries) {
                        Box(Modifier.weight(1f)) { ComicPosterCard(s, onOpenComic, Modifier.fillMaxWidth()) }
                    }
                    repeat(3 - rowSeries.size) { Spacer(Modifier.weight(1f)) }
                }
            }
            if (query.isBlank() && comicPage < comicTotal) {
                item {
                    Button(
                        enabled = !comicLoading,
                        onClick = { scope.launch { loadComics(12) } },
                        modifier = Modifier.fillMaxWidth().padding(16.dp),
                    ) { Text(if (comicLoading) "Memuat…" else "Muat Lebih Banyak") }
                }
            }
        } else if (browsing) {
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
                item { HeroBanner(hero, onOpenTitle) }
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
            for ((t, label) in comicRowTypes) {
                val rows = comicRaw.filter { it.type == t }
                if (rows.isNotEmpty()) {
                    item { SectionTitle(label) }
                    item { ComicPosterRow(rows, onOpenComic) }
                }
            }
            if (terbaru.isNotEmpty()) {
                item { SectionTitle("Baru Ditambahkan") }
                item { PosterRow(terbaru, onOpenTitle) }
            }
            item { Spacer(Modifier.height(24.dp)) }
        }
    }
}

@Composable
private fun SectionTitle(text: String) {
    Text(
        text,
        style = MaterialTheme.typography.titleMedium,
        fontWeight = FontWeight.Bold,
        modifier = Modifier.padding(start = 16.dp, end = 16.dp, top = 18.dp, bottom = 10.dp),
    )
}

@Composable
private fun HeroBanner(t: TitleCard, onOpen: (String) -> Unit) {
    Box(modifier = Modifier.fillMaxWidth().height(430.dp).clickable { onOpen(t.id) }) {
        AsyncImage(
            model = t.posterUrl,
            contentDescription = t.title,
            contentScale = ContentScale.Crop,
            modifier = Modifier.fillMaxSize(),
        )
        Box(
            modifier = Modifier.fillMaxSize().background(
                Brush.verticalGradient(
                    colors = listOf(Color.Transparent, Color.Transparent, MaterialTheme.colorScheme.background),
                    startY = 120f,
                )
            )
        )
        Column(
            modifier = Modifier.align(Alignment.BottomStart).padding(16.dp),
        ) {
            Text(
                t.title,
                style = MaterialTheme.typography.headlineMedium,
                fontWeight = FontWeight.Black,
                color = Color.White,
                maxLines = 2,
                overflow = TextOverflow.Ellipsis,
            )
            Text(
                listOfNotNull(
                    t.rating?.let { "★ $it" },
                    t.year?.toString(),
                    t.type.replaceFirstChar { it.uppercase() },
                    t.genres.joinToString(", ").ifBlank { null },
                ).joinToString(" · "),
                style = MaterialTheme.typography.bodyMedium,
                color = Color(0xFFDDDDDD),
                modifier = Modifier.padding(vertical = 6.dp),
            )
            Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                Button(
                    onClick = { onOpen(t.id) },
                    colors = ButtonDefaults.buttonColors(containerColor = Color.White, contentColor = Color.Black),
                ) { Text("▶  Tonton", fontWeight = FontWeight.Bold) }
                OutlinedButton(onClick = { onOpen(t.id) }) { Text("＋ Daftar Saya", color = Color.White) }
            }
        }
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

@Composable
private fun ComicPosterRow(list: List<com.subsenpai.data.ComicSeries>, onOpen: (com.subsenpai.data.ComicSeries) -> Unit) {
    LazyRow(
        horizontalArrangement = Arrangement.spacedBy(10.dp),
        modifier = Modifier.padding(horizontal = 16.dp),
    ) {
        items(list) { ComicPosterCard(it, onOpen, Modifier.width(112.dp)) }
    }
}

@Composable
private fun ComicPosterCard(
    s: com.subsenpai.data.ComicSeries,
    onOpen: (com.subsenpai.data.ComicSeries) -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(modifier = modifier.clickable { onOpen(s) }) {
        PosterImage(
            url = s.poster,
            title = s.title,
            modifier = Modifier.fillMaxWidth().aspectRatio(2f / 3f).clip(RoundedCornerShape(8.dp)),
        )
        Text(
            s.title,
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
