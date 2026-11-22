package com.subsenpai.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Surface
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
    onBack: () -> Unit,
    onNeedLogin: () -> Unit,
) {
    var detail by remember { mutableStateOf<TitleDetail?>(null) }
    var error by remember { mutableStateOf<String?>(null) }
    var inWatchlist by remember { mutableStateOf(false) }
    var wlBusy by remember { mutableStateOf(false) }
    val scope = rememberCoroutineScope()

    LaunchedEffect(titleId) {
        try {
            detail = repo.titleDetail(titleId)
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
        if (detail == null && error == null) {
            // Selama detail dimuat: animasi merek, bukan halaman kosong.
            item {
                BrandLoading(modifier = Modifier.fillParentMaxHeight())
            }
        }
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
                    PosterImage(
                        url = d.card.posterUrl,
                        title = d.card.title,
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
                                d.durationMin?.let { "$it mnt/ep" },
                                d.card.type.replaceFirstChar { it.uppercase() },
                                d.card.status,
                            ).joinToString(" · "),
                            style = MaterialTheme.typography.bodyMedium,
                            color = Color(0xFFDDDDDD),
                            modifier = Modifier.padding(top = 4.dp),
                        )
                        if (d.card.genres.isNotEmpty()) {
                            Row(
                                modifier = Modifier.padding(top = 8.dp)
                                    .horizontalScroll(rememberScrollState()),
                                horizontalArrangement = Arrangement.spacedBy(6.dp),
                            ) {
                                d.card.genres.forEach { g ->
                                    Surface(
                                        shape = RoundedCornerShape(50),
                                        color = Color(0x33FFFFFF),
                                    ) {
                                        Text(
                                            g,
                                            style = MaterialTheme.typography.labelMedium,
                                            color = Color.White,
                                            modifier = Modifier.padding(horizontal = 10.dp, vertical = 4.dp),
                                        )
                                    }
                                }
                            }
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
                        Spacer(Modifier.height(14.dp))
                        Text(
                            "Sinopsis",
                            style = MaterialTheme.typography.titleSmall,
                            fontWeight = FontWeight.Bold,
                        )
                        Spacer(Modifier.height(4.dp))
                        Text(it, style = MaterialTheme.typography.bodyMedium)
                    }
                    val infoRows = listOfNotNull(
                        d.studio?.let { "Studio" to it },
                        d.network?.let { "Jaringan" to it },
                        d.country?.let { "Negara" to it },
                        d.season?.let { s ->
                            "Musim" to (s.replaceFirstChar { it.uppercase() } +
                                (d.card.year?.let { " $it" } ?: ""))
                        },
                        d.card.titleAlt?.let { "Judul Lain" to it },
                    )
                    if (infoRows.isNotEmpty()) {
                        Spacer(Modifier.height(14.dp))
                        Text(
                            "Detail",
                            style = MaterialTheme.typography.titleSmall,
                            fontWeight = FontWeight.Bold,
                        )
                        Spacer(Modifier.height(4.dp))
                        infoRows.forEach { (label, value) ->
                            Row(modifier = Modifier.padding(vertical = 2.dp)) {
                                Text(
                                    label,
                                    style = MaterialTheme.typography.bodyMedium,
                                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                                    modifier = Modifier.fillMaxWidth(0.28f),
                                )
                                Text(value, style = MaterialTheme.typography.bodyMedium)
                            }
                        }
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
                            "▶  Episode ${ep.episodeNo}" + (ep.title?.let { " — $it" } ?: "") +
                                (ep.durationSec?.let { " · ${it / 60} mnt" } ?: ""),
                            modifier = Modifier.padding(14.dp),
                        )
                    }
                }
            }
            item { Spacer(Modifier.height(24.dp)) }
        }
    }
}
