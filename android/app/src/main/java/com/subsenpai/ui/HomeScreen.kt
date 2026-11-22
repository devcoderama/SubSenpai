package com.subsenpai.ui

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
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
import androidx.compose.material3.Card
import androidx.compose.material3.FilterChip
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import coil.compose.AsyncImage
import com.subsenpai.data.CatalogRepository
import com.subsenpai.data.TitleCard

private val typeFilters = listOf(
    null to "Semua", "anime" to "Anime", "donghua" to "Donghua", "film" to "Film",
    "manga" to "Manga", "manhua" to "Manhua", "manhwa" to "Manhwa",
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

    LazyColumn(
        modifier = Modifier.fillMaxSize().padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        item {
            Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
                Text("SubSenpai", style = MaterialTheme.typography.headlineMedium)
                Row {
                    TextButton(onClick = onOpenPremium) { Text("Premium") }
                    if (loggedIn) {
                        TextButton(onClick = onOpenChat) { Text("Chat") }
                        TextButton(onClick = onOpenWatchlist) { Text("Watchlist") }
                    }
                    TextButton(onClick = onLogin) { Text(if (loggedIn) "Akun" else "Masuk") }
                }
            }
            Spacer(Modifier.height(8.dp))
            OutlinedTextField(
                value = query,
                onValueChange = { query = it },
                label = { Text("Cari judul…") },
                singleLine = true,
                modifier = Modifier.fillMaxWidth(),
            )
            Spacer(Modifier.height(8.dp))
            LazyRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                items(typeFilters) { (value, label) ->
                    FilterChip(
                        selected = type == value,
                        onClick = { type = value },
                        label = { Text(label) },
                    )
                }
            }
        }
        if (error != null) {
            item { Text("Gagal memuat: $error", color = MaterialTheme.colorScheme.error) }
        }
        if (browsing) {
            item { Text("Hasil", style = MaterialTheme.typography.titleMedium) }
            items(hasil) { TitleRow(it, onOpenTitle) }
            if (hasil.isEmpty() && error == null) {
                item { Text("Belum ada judul yang cocok.") }
            }
        } else {
            if (lanjut.isNotEmpty()) {
                item { Text("Lanjutkan", style = MaterialTheme.typography.titleMedium) }
                items(lanjut) { h ->
                    Card(modifier = Modifier.fillMaxWidth().clickable { onOpenEpisode(h.episodeId) }) {
                        Row(modifier = Modifier.padding(12.dp)) {
                            AsyncImage(
                                model = h.posterUrl,
                                contentDescription = h.title,
                                contentScale = ContentScale.Crop,
                                modifier = Modifier.width(56.dp).aspectRatio(3f / 4f),
                            )
                            Spacer(Modifier.width(12.dp))
                            Column {
                                Text(h.title, style = MaterialTheme.typography.titleMedium, maxLines = 1, overflow = TextOverflow.Ellipsis)
                                Text(
                                    "Episode ${h.episodeNo} · posisi ${h.positionSec / 60}:${(h.positionSec % 60).toString().padStart(2, '0')}",
                                    style = MaterialTheme.typography.bodySmall,
                                )
                            }
                        }
                    }
                }
            }
            if (trending.isNotEmpty()) {
                item { Text("Sedang Tren", style = MaterialTheme.typography.titleMedium) }
                item {
                    LazyRow(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                        items(trending) { TitlePosterCard(it, onOpenTitle) }
                    }
                }
            }
            item { Text("Terbaru", style = MaterialTheme.typography.titleMedium) }
            items(terbaru) { TitleRow(it, onOpenTitle) }
        }
    }
}

@Composable
private fun TitlePosterCard(t: TitleCard, onOpen: (String) -> Unit) {
    Card(modifier = Modifier.width(120.dp).clickable { onOpen(t.id) }) {
        Column {
            AsyncImage(
                model = t.posterUrl,
                contentDescription = t.title,
                contentScale = ContentScale.Crop,
                modifier = Modifier.fillMaxWidth().aspectRatio(3f / 4f),
            )
            Text(
                t.title,
                maxLines = 2,
                overflow = TextOverflow.Ellipsis,
                style = MaterialTheme.typography.bodyMedium,
                modifier = Modifier.padding(8.dp),
            )
        }
    }
}

@Composable
fun TitleRow(t: TitleCard, onOpen: (String) -> Unit) {
    Card(modifier = Modifier.fillMaxWidth().clickable { onOpen(t.id) }) {
        Row(modifier = Modifier.padding(12.dp)) {
            AsyncImage(
                model = t.posterUrl,
                contentDescription = t.title,
                contentScale = ContentScale.Crop,
                modifier = Modifier.width(56.dp).aspectRatio(3f / 4f),
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
                )
            }
        }
    }
}
