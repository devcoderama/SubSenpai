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
import com.subsenpai.data.CatalogRepository
import com.subsenpai.data.TitleCard

private val typeFilters = listOf(
    null to "Semua", "anime" to "Anime", "donghua" to "Donghua", "film" to "Film",
    "manga" to "Manga", "manhua" to "Manhua", "manhwa" to "Manhwa",
)

private val rowTypes = listOf(
    "anime" to "Anime Pilihan", "donghua" to "Donghua Pilihan", "film" to "Film Pilihan",
    "manga" to "Manga Pilihan", "manhwa" to "Manhwa Pilihan", "manhua" to "Manhua Pilihan",
)

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
) {
    var trending by remember { mutableStateOf<List<TitleCard>>(emptyList()) }
    var terbaru by remember { mutableStateOf<List<TitleCard>>(emptyList()) }
    var perType by remember { mutableStateOf<Map<String, List<TitleCard>>>(emptyMap()) }
    var hasil by remember { mutableStateOf<List<TitleCard>>(emptyList()) }
    var lanjut by remember { mutableStateOf<List<com.subsenpai.data.WatchHistoryItem>>(emptyList()) }
    var error by remember { mutableStateOf<String?>(null) }
    var type by remember { mutableStateOf<String?>(null) }
    var query by remember { mutableStateOf("") }
    val browsing = type != null || query.isNotBlank()
    val loggedIn = repo.isLoggedIn

    LaunchedEffect(Unit) {
        try {
            val (tr, te) = repo.home()
            trending = tr
            terbaru = te
        } catch (e: Exception) {
            error = e.message
        }
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
        try {
            hasil = repo.titles(type, query.ifBlank { null })
        } catch (e: Exception) {
            error = e.message
        }
    }

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
    if (url.isNullOrBlank()) {
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
        AsyncImage(
            model = url,
            contentDescription = title,
            contentScale = ContentScale.Crop,
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
