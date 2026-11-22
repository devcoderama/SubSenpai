package com.subsenpai.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.Card
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import coil.compose.AsyncImage
import com.subsenpai.data.CatalogRepository
import com.subsenpai.data.TitleDetail
import kotlinx.coroutines.launch

@Composable
fun DetailScreen(
    repo: CatalogRepository,
    titleId: String,
    onPlay: (String) -> Unit,
    onRead: (String) -> Unit,
    onBack: () -> Unit,
    onNeedLogin: () -> Unit,
    onOpenDeviceSource: (String) -> Unit = {},
) {
    var detail by remember { mutableStateOf<TitleDetail?>(null) }
    var error by remember { mutableStateOf<String?>(null) }
    var inWatchlist by remember { mutableStateOf(false) }
    var wlBusy by remember { mutableStateOf(false) }
    val scope = rememberCoroutineScope()

    LaunchedEffect(titleId) {
        try {
            val d = repo.titleDetail(titleId)
            detail = d
            com.subsenpai.data.ReaderNav.titleId = titleId
            com.subsenpai.data.ReaderNav.titleType = d.card.type
            com.subsenpai.data.ReaderNav.chapters = d.chapters
            com.subsenpai.data.ReaderNav.detail = d
        } catch (e: Exception) {
            error = e.message
        }
        if (repo.isLoggedIn) {
            try {
                inWatchlist = repo.watchlistCheck(titleId)
            } catch (e: Exception) {
                // Abaikan — tombol tetap bisa dicoba.
            }
        }
    }

    LazyColumn(modifier = Modifier.fillMaxSize(), verticalArrangement = Arrangement.spacedBy(10.dp)) {
        if (error != null) {
            item {
                TextButton(onClick = onBack) { Text("← Kembali", color = Color.White) }
                Text(
                    "Gagal memuat: $error",
                    color = MaterialTheme.colorScheme.error,
                    modifier = Modifier.padding(horizontal = 16.dp),
                )
            }
        }
        val d = detail
        if (d != null) {
            item {
                Box(modifier = Modifier.fillMaxWidth().height(360.dp)) {
                    AsyncImage(
                        model = d.card.posterUrl,
                        contentDescription = d.card.title,
                        contentScale = ContentScale.Crop,
                        modifier = Modifier.fillMaxSize(),
                    )
                    Box(
                        modifier = Modifier.fillMaxSize().background(
                            Brush.verticalGradient(
                                colors = listOf(Color(0x66000000), Color.Transparent, MaterialTheme.colorScheme.background),
                                startY = 0f,
                            )
                        )
                    )
                    TextButton(
                        onClick = onBack,
                        modifier = Modifier.padding(4.dp),
                    ) { Text("← Kembali", color = Color.White) }
                    Column(modifier = Modifier.padding(16.dp).fillMaxWidth()) {
                        Spacer(Modifier.height(215.dp))
                        Text(
                            d.card.title,
                            style = MaterialTheme.typography.headlineMedium,
                            fontWeight = FontWeight.Black,
                            color = Color.White,
                        )
                        d.card.titleAlt?.let {
                            Text(it, style = MaterialTheme.typography.bodyMedium, color = Color(0xFFDDDDDD))
                        }
                        Text(
                            listOfNotNull(
                                d.card.rating?.let { "★ $it" },
                                d.card.year?.toString(),
                                d.card.type.replaceFirstChar { it.uppercase() },
                                d.card.status,
                            ).joinToString(" · "),
                            style = MaterialTheme.typography.bodyMedium,
                            color = Color(0xFFDDDDDD),
                            modifier = Modifier.padding(top = 4.dp),
                        )
                        if (d.card.genres.isNotEmpty()) {
                            Text(
                                d.card.genres.joinToString(", "),
                                style = MaterialTheme.typography.bodySmall,
                                color = MaterialTheme.colorScheme.onSurfaceVariant,
                            )
                        }
                    }
                }
            }
            item {
                Column(modifier = Modifier.padding(horizontal = 16.dp)) {
                    Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                        when {
                            d.episodes.isNotEmpty() -> {
                                Button(
                                    onClick = { onPlay(d.episodes.first().id) },
                                    colors = ButtonDefaults.buttonColors(containerColor = Color.White, contentColor = Color.Black),
                                ) { Text("▶  Putar", fontWeight = FontWeight.Bold) }
                            }
                            d.chapters.isNotEmpty() -> {
                                Button(
                                    onClick = { onRead(d.chapters.first().id) },
                                    colors = ButtonDefaults.buttonColors(containerColor = Color.White, contentColor = Color.Black),
                                ) { Text("▶  Baca", fontWeight = FontWeight.Bold) }
                            }
                            d.fetchMode == "device" -> {
                                Button(
                                    onClick = { onOpenDeviceSource(titleId) },
                                    colors = ButtonDefaults.buttonColors(containerColor = Color.White, contentColor = Color.Black),
                                ) { Text("▶  Buka dari Sumber", fontWeight = FontWeight.Bold) }
                            }
                        }
                        OutlinedButton(
                            enabled = !wlBusy,
                            onClick = {
                                if (!repo.isLoggedIn) {
                                    onNeedLogin()
                                } else {
                                    wlBusy = true
                                    scope.launch {
                                        try {
                                            if (inWatchlist) {
                                                repo.watchlistRemove(titleId)
                                                inWatchlist = false
                                            } else {
                                                repo.watchlistAdd(titleId)
                                                inWatchlist = true
                                            }
                                        } catch (e: Exception) {
                                            error = e.message
                                        } finally {
                                            wlBusy = false
                                        }
                                    }
                                }
                            },
                        ) {
                            Text(
                                when {
                                    !repo.isLoggedIn -> "＋ Daftar Saya (masuk dulu)"
                                    inWatchlist -> "✓ Di Daftar Saya"
                                    else -> "＋ Daftar Saya"
                                },
                                color = Color.White,
                            )
                        }
                    }
                    d.synopsis?.let {
                        Spacer(Modifier.height(12.dp))
                        Text(it, style = MaterialTheme.typography.bodyMedium)
                    }
                }
            }
            if (d.episodes.isNotEmpty()) {
                item {
                    Text(
                        "Episode",
                        style = MaterialTheme.typography.titleMedium,
                        fontWeight = FontWeight.Bold,
                        modifier = Modifier.padding(horizontal = 16.dp),
                    )
                }
                items(d.episodes) { ep ->
                    Card(
                        modifier = Modifier.fillMaxWidth().padding(horizontal = 16.dp)
                            .clickable { onPlay(ep.id) },
                    ) {
                        Text(
                            "Episode ${ep.episodeNo}" + (ep.title?.let { " — $it" } ?: ""),
                            modifier = Modifier.padding(14.dp),
                        )
                    }
                }
            }
            if (d.fetchMode == "device") {
                item {
                    Text(
                        "Sumber Perangkat (F39)",
                        style = MaterialTheme.typography.titleMedium,
                        fontWeight = FontWeight.Bold,
                        modifier = Modifier.padding(horizontal = 16.dp),
                    )
                }
                item {
                    Text(
                        "Judul ini diambil langsung dari ${d.sourceDomain ?: "sumbernya"} di perangkatmu " +
                            "agar lolos perlindungan Cloudflare. Daftar chapter & halaman dimuat dari situs sumber.",
                        style = MaterialTheme.typography.bodySmall,
                        modifier = Modifier.padding(horizontal = 16.dp),
                    )
                }
                item {
                    Button(
                        onClick = { onOpenDeviceSource(titleId) },
                        modifier = Modifier.padding(horizontal = 16.dp),
                    ) { Text("Buka dari Sumber") }
                }
            }
            if (d.chapters.isNotEmpty()) {
                item {
                    Text(
                        "Chapter",
                        style = MaterialTheme.typography.titleMedium,
                        fontWeight = FontWeight.Bold,
                        modifier = Modifier.padding(horizontal = 16.dp),
                    )
                }
                items(d.chapters) { ch ->
                    Card(
                        modifier = Modifier.fillMaxWidth().padding(horizontal = 16.dp)
                            .clickable { onRead(ch.id) },
                    ) {
                        Text(
                            "Chapter ${formatChapterNo(ch.chapterNo)}" + (ch.title?.let { " — $it" } ?: "") +
                                " · ${ch.pageCount} halaman",
                            modifier = Modifier.padding(14.dp),
                        )
                    }
                }
            }
            item { Spacer(Modifier.height(24.dp)) }
        }
    }
}

private fun formatChapterNo(n: Double): String =
    if (n == n.toLong().toDouble()) n.toLong().toString() else n.toString()
