package com.subsenpai.ui

import android.content.Intent
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.subsenpai.R
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
                // Kepala ala Netflix: gambar penuh dengan tombol
                // kembali melayang; judul & aksi pindah ke bawah
                // gambar di atas latar pekat.
                Box(modifier = Modifier.fillMaxWidth().height(300.dp)) {
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
                    Box(
                        modifier = Modifier.padding(12.dp).size(36.dp)
                            .background(Color(0x99000000), CircleShape)
                            .clickable { onBack() },
                        contentAlignment = Alignment.Center,
                    ) {
                        Icon(
                            painter = painterResource(R.drawable.ic_arrow_back),
                            contentDescription = "Kembali",
                            tint = Color.White,
                            modifier = Modifier.size(20.dp),
                        )
                    }
                }
            }
            item {
                Column(modifier = Modifier.padding(horizontal = 16.dp)) {
                    // Blok judul & aksi ala Netflix: judul besar, baris
                    // meta, genre bertitik, tombol Putar putih selebar
                    // layar, lalu baris ikon Daftar Saya / Bagikan.
                    Text(
                        d.card.title,
                        style = MaterialTheme.typography.headlineSmall,
                        fontWeight = FontWeight.Black,
                        color = Color.White,
                    )
                    d.card.titleAlt?.let {
                        Text(it, style = MaterialTheme.typography.bodyMedium, color = Color(0xFFB3B3B3))
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
                        modifier = Modifier.padding(top = 6.dp),
                    )
                    if (d.card.genres.isNotEmpty()) {
                        Text(
                            d.card.genres.joinToString(" • "),
                            style = MaterialTheme.typography.bodyMedium,
                            color = Color(0xFFB3B3B3),
                            modifier = Modifier.padding(top = 4.dp),
                        )
                    }
                    if (d.episodes.isNotEmpty()) {
                        Button(
                            onClick = { onPlay(d.episodes.first().id) },
                            colors = ButtonDefaults.buttonColors(
                                containerColor = Color.White,
                                contentColor = Color.Black,
                            ),
                            shape = RoundedCornerShape(6.dp),
                            modifier = Modifier.fillMaxWidth().padding(top = 14.dp).height(48.dp),
                        ) {
                            Icon(
                                painter = painterResource(R.drawable.ic_play),
                                contentDescription = null,
                                tint = Color.Black,
                                modifier = Modifier.size(24.dp),
                            )
                            Spacer(Modifier.size(6.dp))
                            Text("Putar", fontWeight = FontWeight.Bold, fontSize = 17.sp)
                        }
                    }
                    val ctx = LocalContext.current
                    Row(
                        modifier = Modifier.fillMaxWidth().padding(top = 10.dp),
                        horizontalArrangement = Arrangement.spacedBy(28.dp),
                    ) {
                        DetailAction(
                            icon = if (inWatchlist) R.drawable.ic_check else R.drawable.ic_add,
                            label = "Daftar Saya",
                            onClick = {
                                if (!repo.isLoggedIn) {
                                    onNeedLogin()
                                } else if (!wlBusy) {
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
                        )
                        DetailAction(
                            icon = R.drawable.ic_share,
                            label = "Bagikan",
                            onClick = {
                                val send = Intent(Intent.ACTION_SEND).apply {
                                    type = "text/plain"
                                    putExtra(
                                        Intent.EXTRA_TEXT,
                                        "Nonton ${d.card.title} di SubSenpai" +
                                            (d.synopsis?.let { "\n\n$it" } ?: ""),
                                    )
                                }
                                ctx.startActivity(Intent.createChooser(send, "Bagikan"))
                            },
                        )
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
                        colors = CardDefaults.cardColors(containerColor = Color(0xFF1F1F1F)),
                        shape = RoundedCornerShape(8.dp),
                    ) {
                        Row(
                            modifier = Modifier.padding(horizontal = 14.dp, vertical = 12.dp),
                            verticalAlignment = Alignment.CenterVertically,
                        ) {
                            Text(
                                ep.episodeNo.toString(),
                                style = MaterialTheme.typography.headlineSmall,
                                fontWeight = FontWeight.Bold,
                                color = Color.White,
                                modifier = Modifier.size(width = 42.dp, height = 30.dp),
                            )
                            Column(modifier = Modifier.weight(1f)) {
                                Text(
                                    ep.title ?: "Episode ${ep.episodeNo}",
                                    style = MaterialTheme.typography.bodyLarge,
                                    color = Color.White,
                                    maxLines = 1,
                                )
                                ep.durationSec?.let {
                                    Text(
                                        "${it / 60} mnt",
                                        style = MaterialTheme.typography.bodySmall,
                                        color = Color(0xFFB3B3B3),
                                    )
                                }
                            }
                            Box(
                                modifier = Modifier.size(38.dp)
                                    .background(Color.Transparent, CircleShape),
                                contentAlignment = Alignment.Center,
                            ) {
                                Icon(
                                    painter = painterResource(R.drawable.ic_play),
                                    contentDescription = "Putar episode ${ep.episodeNo}",
                                    tint = Color.White,
                                    modifier = Modifier.size(28.dp),
                                )
                            }
                        }
                    }
                }
            }
            item { Spacer(Modifier.height(24.dp)) }
        }
    }
}

/** Aksi ikon-di-atas-label ala Netflix di halaman detail. */
@Composable
private fun DetailAction(icon: Int, label: String, onClick: () -> Unit) {
    Column(
        horizontalAlignment = Alignment.CenterHorizontally,
        modifier = Modifier.clickable { onClick() }.padding(horizontal = 6.dp, vertical = 4.dp),
    ) {
        Icon(
            painter = painterResource(icon),
            contentDescription = label,
            tint = Color.White,
            modifier = Modifier.size(26.dp),
        )
        Text(
            label,
            color = Color(0xFFB3B3B3),
            style = MaterialTheme.typography.labelMedium,
            modifier = Modifier.padding(top = 3.dp),
        )
    }
}
