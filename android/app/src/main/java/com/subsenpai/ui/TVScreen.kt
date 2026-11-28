package com.subsenpai.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.border
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
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
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
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import coil.compose.AsyncImage
import coil.request.ImageRequest
import com.subsenpai.data.CatalogRepository

/** Kategori kanal TV (sama dengan `category` di tv_channels). */
private val tvCategories = listOf("Semua", "Nasional", "Berita", "Olahraga", "Daerah", "Internasional")

/**
 * Layar TV Langsung (IPTV, permintaan Rams 2026-11-28): kanal TV
 * live HLS yang diputar langsung dari perangkat ke sumber stream.
 * Gratis semua tier. Lencana stabilitas: "Resmi" hijau untuk
 * stream resmi yang terverifikasi hidup, "Tidak stabil" kuning
 * untuk restream / yang belum terverifikasi.
 */
@Composable
fun TVScreen(
    repo: CatalogRepository,
    onBack: () -> Unit,
    onOpenChannel: (CatalogRepository.TVChannel) -> Unit,
) {
    var channels by remember { mutableStateOf<List<CatalogRepository.TVChannel>>(emptyList()) }
    var error by remember { mutableStateOf<String?>(null) }
    var loaded by remember { mutableStateOf(false) }
    var category by remember { mutableStateOf("Semua") }
    var refreshKey by remember { mutableStateOf(0) }

    LaunchedEffect(refreshKey) {
        loaded = false
        error = null
        try {
            channels = repo.tvChannels()
        } catch (e: Exception) {
            error = e.message
        } finally {
            loaded = true
        }
    }

    val shown = if (category == "Semua") channels else channels.filter { it.category == category }

    LazyColumn(modifier = Modifier.fillMaxSize().padding(horizontal = 16.dp)) {
        item { TextButton(onClick = onBack, modifier = Modifier.padding(top = 4.dp)) { Text("← Kembali") } }
        item {
            Text("TV Langsung", style = MaterialTheme.typography.headlineSmall, fontWeight = FontWeight.Bold)
        }
        item {
            Text(
                "Siaran TV lokal & luar, langsung dari sumbernya — gratis.",
                style = MaterialTheme.typography.bodyMedium,
                color = Color(0xFFB3B3B3),
                modifier = Modifier.padding(top = 4.dp, bottom = 12.dp),
            )
        }
        item {
            LazyRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                items(tvCategories) { c ->
                    TVCategoryChip(c, c == category) { category = c }
                }
            }
        }
        item { Spacer(Modifier.height(14.dp)) }
        if (error != null) {
            item {
                Text(
                    "Gagal memuat kanal TV: $error",
                    color = MaterialTheme.colorScheme.error,
                    modifier = Modifier.padding(vertical = 8.dp),
                )
            }
            item {
                TextButton(onClick = { refreshKey++ }) { Text("Coba lagi") }
            }
        }
        if (!loaded) {
            item {
                Box(
                    modifier = Modifier.fillMaxWidth().padding(vertical = 48.dp),
                    contentAlignment = Alignment.Center,
                ) {
                    CircularProgressIndicator(color = Color(0xFFE50914))
                }
            }
        }
        if (loaded && shown.isEmpty() && error == null) {
            item {
                Text(
                    if (channels.isEmpty()) "Belum ada kanal TV yang aktif saat ini."
                    else "Tidak ada kanal kategori $category.",
                    color = Color(0xFFB3B3B3),
                    modifier = Modifier.padding(vertical = 8.dp),
                )
            }
        }
        items(shown.chunked(3)) { row ->
            Row(
                modifier = Modifier.fillMaxWidth().padding(vertical = 6.dp),
                horizontalArrangement = Arrangement.spacedBy(10.dp),
            ) {
                for (ch in row) {
                    Box(Modifier.weight(1f)) { TVChannelCard(ch, onOpenChannel, Modifier.fillMaxWidth()) }
                }
                repeat(3 - row.size) { Spacer(Modifier.weight(1f)) }
            }
        }
        item { Spacer(Modifier.height(24.dp)) }
    }
}

@Composable
private fun TVCategoryChip(label: String, selected: Boolean, onClick: () -> Unit) {
    Box(
        modifier = Modifier
            .clip(RoundedCornerShape(50))
            .background(if (selected) Color(0xFFE50914) else Color(0xFF2B2B2B))
            .clickable { onClick() }
            .padding(horizontal = 14.dp, vertical = 8.dp),
        contentAlignment = Alignment.Center,
    ) {
        Text(
            label,
            color = Color.White,
            fontWeight = if (selected) FontWeight.Bold else FontWeight.Normal,
            style = MaterialTheme.typography.bodyMedium,
        )
    }
}

/**
 * Kartu kanal TV: logo 16:9, nama, kualitas, dan lencana
 * stabilitas. Dipakai grid layar TV + rel Beranda.
 */
@Composable
fun TVChannelCard(
    channel: CatalogRepository.TVChannel,
    onOpen: (CatalogRepository.TVChannel) -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(modifier = modifier.clickable { onOpen(channel) }) {
        ChannelLogo(
            url = channel.logoUrl,
            name = channel.name,
            modifier = Modifier.fillMaxWidth().aspectRatio(16f / 9f)
                .clip(RoundedCornerShape(8.dp)),
        )
        Text(
            channel.name,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
            style = MaterialTheme.typography.bodyMedium,
            fontWeight = FontWeight.SemiBold,
            modifier = Modifier.padding(top = 6.dp),
        )
        Row(
            verticalAlignment = Alignment.CenterVertically,
            modifier = Modifier.padding(top = 2.dp),
        ) {
            if (!channel.quality.isNullOrBlank()) {
                Text(
                    channel.quality,
                    style = MaterialTheme.typography.bodySmall,
                    color = Color(0xFFB3B3B3),
                )
                Spacer(Modifier.width(6.dp))
            }
            StabilityBadge(channel.stability)
        }
    }
}

/** Lencana stabilitas: "Resmi" hijau, "Tidak stabil" kuning. */
@Composable
fun StabilityBadge(stability: String) {
    val good = stability == "bagus"
    Box(
        modifier = Modifier
            .clip(RoundedCornerShape(4.dp))
            .background(if (good) Color(0xFF2E7D32) else Color(0xFFFFC107))
            .padding(horizontal = 6.dp, vertical = 2.dp),
    ) {
        Text(
            if (good) "Resmi" else "Tidak stabil",
            color = if (good) Color.White else Color.Black,
            style = MaterialTheme.typography.labelSmall,
            fontWeight = FontWeight.Bold,
        )
    }
}

/**
 * Logo kanal; bila URL kosong / gagal muat, tampilkan plakat nama
 * kanal (pola yang sama dengan PosterImage untuk poster judul).
 * Logo dari wikimedia/imgur adalah PNG (thumbnail .svg.png) jadi
 * Coil bisa mendekodenya langsung.
 */
@Composable
fun ChannelLogo(url: String?, name: String, modifier: Modifier = Modifier) {
    var gagal by remember(url) { mutableStateOf(false) }
    if (url.isNullOrBlank() || gagal) {
        Box(
            modifier = modifier.background(
                Brush.verticalGradient(listOf(Color(0xFF3B2A5D), Color(0xFF1B1B1B))),
            ),
            contentAlignment = Alignment.Center,
        ) {
            Text(
                name,
                color = Color.White,
                fontWeight = FontWeight.Bold,
                textAlign = TextAlign.Center,
                style = MaterialTheme.typography.titleSmall,
                maxLines = 2,
                overflow = TextOverflow.Ellipsis,
                modifier = Modifier.padding(8.dp),
            )
        }
    } else {
        val referer = try {
            val u = java.net.URL(url)
            "${u.protocol}://${u.host}/"
        } catch (e: Exception) {
            null
        }
        AsyncImage(
            model = ImageRequest.Builder(LocalContext.current)
                .data(url)
                .size(540, 304)
                .addHeader(
                    "User-Agent",
                    "Mozilla/5.0 (Linux; Android 13) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Mobile Safari/537.36",
                )
                .apply { if (referer != null) addHeader("Referer", referer) }
                .crossfade(true)
                .build(),
            contentDescription = name,
            contentScale = ContentScale.Fit,
            onError = { gagal = true },
            modifier = modifier.background(Color(0xFF1F1F1F)),
        )
    }
}
