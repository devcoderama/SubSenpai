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
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.Card
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.Icon
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
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.blur
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.subsenpai.R
import com.subsenpai.data.CatalogRepository
import com.subsenpai.data.SettingsStore
import com.subsenpai.data.TitleCard
import com.subsenpai.data.TitleDetail
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

@Composable
fun DetailScreen(
    repo: CatalogRepository,
    titleId: String,
    onPlay: (String) -> Unit,
    onBack: () -> Unit,
    onNeedLogin: () -> Unit,
    onOpenTitle: (String) -> Unit,
    onShareToPost: (TitleDetail) -> Unit,
    onOpenPremium: () -> Unit = {},
    onOpenAdUnlock: (String) -> Unit = {},
) {
    var detail by remember { mutableStateOf<TitleDetail?>(null) }
    var error by remember { mutableStateOf<String?>(null) }
    // Episode masih ditarik server dari sumber di latar belakang.
    var epLoading by remember { mutableStateOf(false) }
    // Episode tak kunjung muncul setelah jendela polling panjang:
    // tampilkan tombol coba lagi (dulu polling berhenti diam-diam
    // setelah ±30 detik dan halaman mandek selamanya).
    var epFailed by remember { mutableStateOf(false) }
    var epRetry by remember { mutableStateOf(0) }
    // Episode terkunci yang diketuk: tampilkan pilihan Premium atau
    // buka lewat iklan (Monetag).
    var lockedEp by remember { mutableStateOf<com.subsenpai.data.EpisodeItem?>(null) }
    var inWatchlist by remember { mutableStateOf(false) }
    var wlBusy by remember { mutableStateOf(false) }
    var serupa by remember { mutableStateOf<List<TitleCard>>(emptyList()) }
    val scope = rememberCoroutineScope()
    val detailContext = LocalContext.current
    val posterHiRes = remember { SettingsStore(detailContext).posterHiRes }
    // Kartu yang diketuk di Beranda/daftar: kepalanya digambar
    // SEKETIKA dari cache selagi detail penuh masih dimuat.
    val cachedCard = remember(titleId) { repo.cachedCard(titleId) }

    // Pilihan membuka episode terkunci: Premium atau iklan (Monetag).
    lockedEp?.let { ep ->
        AlertDialog(
            onDismissRequest = { lockedEp = null },
            title = { Text("Episode ${ep.episodeNo} Terkunci") },
            text = {
                Text(
                    "Episode ini khusus Premium. Kamu juga bisa membukanya " +
                        "GRATIS dengan menonton iklan singkat — sekali dibuka, " +
                        "episode ini terbuka permanen untukmu.",
                )
            },
            confirmButton = {
                Button(
                    onClick = {
                        lockedEp = null
                        onOpenAdUnlock(ep.id)
                    },
                    colors = ButtonDefaults.buttonColors(containerColor = NetflixRed),
                ) { Text("Tonton Iklan", color = Color.White) }
            },
            dismissButton = {
                Row {
                    TextButton(
                        onClick = {
                            lockedEp = null
                            onOpenPremium()
                        },
                    ) { Text("Premium") }
                    TextButton(onClick = { lockedEp = null }) { Text("Nanti Saja") }
                }
            },
        )
    }

    // "Judul Serupa" ala Netflix: judul lain berjenis sama. Sengaja
    // ditunda sampai episode selesai dimuat (bukan prioritas) dan
    // cukup mengambil 12 kartu, bukan satu halaman penuh 30.
    LaunchedEffect(detail?.card?.type, epLoading) {
        val d = detail ?: return@LaunchedEffect
        if (epLoading) return@LaunchedEffect
        try {
            serupa = repo.titlesPaged(d.card.type, null, limit = 12).first
                .filter { it.id != titleId }.take(9)
        } catch (e: Exception) {
            // Bagian ini hiasan — abaikan bila gagal.
        }
    }

    // Status Daftar Saya dicek sejajar dengan pemuatan detail,
    // tidak mengantre di belakangnya.
    LaunchedEffect(titleId) {
        if (repo.isLoggedIn) {
            try {
                inWatchlist = repo.watchlistCheck(titleId)
            } catch (e: Exception) {
                // Abaikan — tombol tetap bisa dicoba.
            }
        }
    }

    // Prefetch playback episode pertama begitu daftar episode tiba:
    // tombol Putar nanti tinggal memakai hasilnya, memangkas satu
    // perjalanan jaringan penuh sebelum video mulai.
    LaunchedEffect(detail?.episodes?.firstOrNull()?.id) {
        val first = detail?.episodes?.firstOrNull() ?: return@LaunchedEffect
        if (!first.locked) {
            launch { repo.prefetchPlayback(first.id) }
        }
    }

    LaunchedEffect(titleId, epRetry) {
        try {
            var d = repo.titleDetail(titleId)
            detail = d
            // Episode kosong untuk judul video: server menariknya
            // dari sumber saat detail dibuka (bisa selesai di latar
            // beberapa detik sesudah respons pertama) — muat ulang
            // sampai episode muncul, agar halaman detail yang
            // terbuka langsung berisi episode (permintaan Rams
            // 2026-11-22). Jendela polling PANJANG dengan jeda
            // membesar (1 dtk → 2,5 dtk → 5 dtk, total ±2,7 menit):
            // pengisian judul panjang di server bisa melewati
            // jendela pendek, dan halaman tidak boleh mandek
            // selamanya (keluhan Rams 2026-11-23).
            if (d.episodes.isEmpty() &&
                (d.card.type == "anime" || d.card.type == "donghua" || d.card.type == "film")
            ) {
                epLoading = true
                epFailed = false
                var tries = 0
                while (d.episodes.isEmpty() && tries < 50) {
                    delay(
                        when {
                            tries == 0 -> 800
                            tries < 10 -> 1000
                            tries < 30 -> 2500
                            else -> 5000
                        },
                    )
                    tries++
                    try {
                        d = repo.titleDetail(titleId)
                        detail = d
                    } catch (e: Exception) {
                        // Abaikan — percobaan berikutnya / keadaan terakhir dipakai.
                    }
                }
                epLoading = false
                epFailed = d.episodes.isEmpty()
            }
        } catch (e: Exception) {
            error = e.message
        }
    }

    LazyColumn(modifier = Modifier.fillMaxSize(), verticalArrangement = Arrangement.spacedBy(10.dp)) {
        if (detail == null && error == null) {
            val c = cachedCard
            if (c != null) {
                // Kepala instan dari kartu yang diketuk: poster,
                // judul, dan meta langsung tampil; sisanya menyusul.
                item {
                    DetailBackdrop(url = c.posterUrl, title = c.title, onBack = onBack, hiRes = posterHiRes)
                }
                item {
                    Column(modifier = Modifier.padding(horizontal = 16.dp)) {
                        DetailHeading(card = c, durationMin = null)
                        Row(
                            verticalAlignment = Alignment.CenterVertically,
                            modifier = Modifier.padding(top = 16.dp),
                        ) {
                            CircularProgressIndicator(color = NetflixRed, modifier = Modifier.size(18.dp))
                            Spacer(Modifier.size(10.dp))
                            Text("Memuat detail…", color = Color(0xFFB3B3B3))
                        }
                    }
                }
            } else {
                // Tanpa kartu cache: animasi merek, bukan halaman kosong.
                item {
                    BrandLoading(modifier = Modifier.fillParentMaxHeight())
                }
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
                // Kepala ala Netflix: latar buram dari poster yang
                // sama memenuhi lebar layar, poster tajam (varian
                // resolusi tinggi untuk film) berdiri di tengah,
                // tombol kembali melayang. Judul & aksi di bawahnya.
                DetailBackdrop(url = d.card.posterUrl, title = d.card.title, onBack = onBack, hiRes = posterHiRes)
            }
            item {
                Column(modifier = Modifier.padding(horizontal = 16.dp)) {
                    // Blok judul & aksi ala Netflix: judul besar, baris
                    // meta, genre bertitik, tombol Putar putih selebar
                    // layar, lalu baris ikon Daftar Saya / Bagikan.
                    DetailHeading(card = d.card, durationMin = d.durationMin)
                    if (d.episodes.isNotEmpty()) {
                        val firstLocked = d.episodes.first().locked
                        Button(
                            onClick = {
                                if (firstLocked) lockedEp = d.episodes.first() else onPlay(d.episodes.first().id)
                            },
                            colors = ButtonDefaults.buttonColors(
                                containerColor = Color.White,
                                contentColor = Color.Black,
                            ),
                            shape = RoundedCornerShape(6.dp),
                            modifier = Modifier.fillMaxWidth().padding(top = 14.dp).height(48.dp),
                        ) {
                            Icon(
                                painter = painterResource(
                                    if (firstLocked) R.drawable.ic_lock else R.drawable.ic_play,
                                ),
                                contentDescription = null,
                                tint = Color.Black,
                                modifier = Modifier.size(24.dp),
                            )
                            Spacer(Modifier.size(6.dp))
                            Text(
                                if (firstLocked) "Khusus Premium" else "Putar",
                                fontWeight = FontWeight.Bold,
                                fontSize = 17.sp,
                            )
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
                        Box {
                            var shareMenu by remember { mutableStateOf(false) }
                            DetailAction(
                                icon = R.drawable.ic_share,
                                label = "Bagikan",
                                onClick = { shareMenu = true },
                            )
                            DropdownMenu(
                                expanded = shareMenu,
                                onDismissRequest = { shareMenu = false },
                            ) {
                                DropdownMenuItem(
                                    text = { Text("Bagikan ke Post") },
                                    onClick = {
                                        shareMenu = false
                                        onShareToPost(d)
                                    },
                                )
                                DropdownMenuItem(
                                    text = { Text("Bagikan ke aplikasi lain") },
                                    onClick = {
                                        shareMenu = false
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
            if (d.episodes.isEmpty() &&
                (d.card.type == "anime" || d.card.type == "donghua" || d.card.type == "film")
            ) {
                item {
                    Text(
                        if (epLoading) {
                            "Sedang memuat episode dari sumber…"
                        } else {
                            "Episode belum tersedia di sumber untuk judul ini."
                        },
                        color = Color(0xFFB3B3B3),
                        style = MaterialTheme.typography.bodyMedium,
                        modifier = Modifier.padding(horizontal = 16.dp),
                    )
                }
                if (!epLoading && epFailed) {
                    item {
                        OutlinedButton(
                            onClick = {
                                epFailed = false
                                epRetry++
                            },
                            colors = ButtonDefaults.outlinedButtonColors(contentColor = NetflixRed),
                            modifier = Modifier.padding(horizontal = 16.dp),
                        ) {
                            Text("Coba Lagi", fontWeight = FontWeight.Bold)
                        }
                    }
                }
            }
            // Film tidak butuh daftar episode — tombol Putar di atas
            // sudah cukup (Netflix pun begitu untuk film).
            if (d.episodes.isNotEmpty() && d.card.type != "film") {
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
                            .clickable { if (ep.locked) lockedEp = ep else onPlay(ep.id) },
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
                                if (ep.locked) {
                                    Text(
                                        "Khusus Premium",
                                        style = MaterialTheme.typography.bodySmall,
                                        color = Color(0xFFFFC107),
                                    )
                                }
                            }
                            Box(
                                modifier = Modifier.size(38.dp)
                                    .background(Color.Transparent, CircleShape),
                                contentAlignment = Alignment.Center,
                            ) {
                                Icon(
                                    painter = painterResource(
                                        if (ep.locked) R.drawable.ic_lock else R.drawable.ic_play,
                                    ),
                                    contentDescription = "Putar episode ${ep.episodeNo}",
                                    tint = if (ep.locked) Color(0xFFFFC107) else Color.White,
                                    modifier = Modifier.size(28.dp),
                                )
                            }
                        }
                    }
                }
            }
            if (serupa.isNotEmpty()) {
                item {
                    Text(
                        "Judul Serupa",
                        style = MaterialTheme.typography.titleMedium,
                        fontWeight = FontWeight.Bold,
                        modifier = Modifier.padding(horizontal = 16.dp),
                    )
                }
                items(serupa.chunked(3)) { rowTitles ->
                    Row(
                        modifier = Modifier.fillMaxWidth().padding(horizontal = 16.dp),
                        horizontalArrangement = Arrangement.spacedBy(10.dp),
                    ) {
                        for (t in rowTitles) {
                            Column(
                                modifier = Modifier.weight(1f).clickable { onOpenTitle(t.id) },
                            ) {
                                PosterImage(
                                    url = t.posterUrl,
                                    title = t.title,
                                    modifier = Modifier.fillMaxWidth().aspectRatio(2f / 3f)
                                        .clip(RoundedCornerShape(8.dp)),
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
                        repeat(3 - rowTitles.size) { Spacer(Modifier.weight(1f)) }
                    }
                }
            }
            item { Spacer(Modifier.height(24.dp)) }
        }
    }
}

/**
 * Blok judul + judul lain + baris meta + genre halaman detail.
 * Dipakai detail penuh maupun kepala sementara dari kartu cache
 * (durationMin belum dikenal pada kepala sementara).
 */
@Composable
private fun DetailHeading(card: TitleCard, durationMin: Int?) {
    Text(
        card.title,
        style = MaterialTheme.typography.headlineSmall,
        fontWeight = FontWeight.Black,
        color = Color.White,
    )
    card.titleAlt?.let {
        Text(it, style = MaterialTheme.typography.bodyMedium, color = Color(0xFFB3B3B3))
    }
    Text(
        listOfNotNull(
            card.rating?.let { "★ $it" },
            card.year?.toString(),
            durationMin?.let { m ->
                when {
                    card.type == "film" && m >= 60 -> "${m / 60}j ${m % 60}mnt"
                    card.type == "film" -> "$m mnt"
                    else -> "$m mnt/ep"
                }
            },
            card.type.replaceFirstChar { it.uppercase() },
            when (card.status) {
                "completed" -> "Tamat"
                "ongoing" -> "Berlanjut"
                else -> card.status
            },
        ).joinToString(" · "),
        style = MaterialTheme.typography.bodyMedium,
        color = Color(0xFFDDDDDD),
        modifier = Modifier.padding(top = 6.dp),
    )
    if (card.genres.isNotEmpty()) {
        Text(
            card.genres.joinToString(" • "),
            style = MaterialTheme.typography.bodyMedium,
            color = Color(0xFFB3B3B3),
            modifier = Modifier.padding(top = 4.dp),
        )
    }
}

/**
 * Kepala halaman detail: poster yang sama diburamkan memenuhi latar
 * (menutupi keterbatasan resolusi sumber kecil seperti 225px),
 * poster tajamnya berdiri di tengah setinggi kepala, gradasi ke
 * latar di bawah, dan tombol kembali melayang di kiri atas.
 */
@Composable
private fun DetailBackdrop(url: String?, title: String, onBack: () -> Unit, hiRes: Boolean) {
    Box(modifier = Modifier.fillMaxWidth().height(340.dp)) {
        // Lapisan buram memakai URL poster biasa: sudah terunduh
        // untuk kartu di Beranda (cache Coil), jadi tampil seketika.
        // Hanya poster tajam di tengah yang memakai varian resolusi
        // tinggi bila diatur (dulu keduanya URL hi-res yang sama —
        // poster film terunduh dua kali).
        PosterImage(
            url = url,
            title = title,
            modifier = Modifier.fillMaxSize().blur(30.dp),
        )
        Box(
            modifier = Modifier.fillMaxSize()
                .background(Color(0x59000000)),
        )
        PosterImage(
            url = if (hiRes) hiResPosterUrl(url) else url,
            title = title,
            modifier = Modifier.fillMaxHeight().aspectRatio(2f / 3f)
                .align(Alignment.Center),
        )
        Box(
            modifier = Modifier.fillMaxSize().background(
                Brush.verticalGradient(
                    colors = listOf(Color(0x4D000000), Color.Transparent, MaterialTheme.colorScheme.background),
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
