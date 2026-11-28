package com.subsenpai.ui

import android.content.Intent
import android.net.Uri
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
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.RadioButton
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
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import coil.Coil
import com.subsenpai.R
import com.subsenpai.data.AuthStore
import com.subsenpai.data.CatalogRepository
import com.subsenpai.data.SettingsStore
import com.subsenpai.data.WatchHistoryItem
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

/**
 * Layar "Lainnya" (pengganti tombol Akun di bilah bawah, permintaan
 * Rams 2026-11-22): kartu akun di atas, lalu sub-menu — Riwayat
 * Tontonan, Daftar Saya, Premium, Teman, Grup Chat, Pengaturan.
 */
@Composable
fun MoreScreen(
    onOpenLogin: () -> Unit,
    onOpenHistory: () -> Unit,
    onOpenWatchlist: () -> Unit,
    onOpenDevices: () -> Unit,
    onOpenPremium: () -> Unit,
    onOpenFriends: () -> Unit,
    onOpenGroups: () -> Unit,
    onOpenSettings: () -> Unit,
    onOpenTransactions: () -> Unit,
    onOpenMyVouchers: () -> Unit,
    onOpenReferral: () -> Unit,
) {
    val context = LocalContext.current
    val auth = remember { AuthStore(context) }
    val loggedIn = auth.token != null
    val version = remember {
        try {
            context.packageManager.getPackageInfo(context.packageName, 0).versionName ?: ""
        } catch (e: Exception) {
            ""
        }
    }
    LazyColumn(modifier = Modifier.fillMaxSize()) {
        item {
            Text(
                "Lainnya",
                style = MaterialTheme.typography.headlineSmall,
                fontWeight = FontWeight.Bold,
                modifier = Modifier.padding(16.dp),
            )
        }
        item {
            Card(
                modifier = Modifier.fillMaxWidth().padding(horizontal = 16.dp)
                    .clickable { onOpenLogin() },
                colors = CardDefaults.cardColors(containerColor = Color(0xFF1F1F1F)),
                shape = RoundedCornerShape(10.dp),
            ) {
                Row(
                    modifier = Modifier.padding(16.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Box(
                        modifier = Modifier.size(48.dp)
                            .background(NetflixRed, CircleShape),
                        contentAlignment = Alignment.Center,
                    ) {
                        Text(
                            (auth.displayName ?: "S").take(1).uppercase(),
                            color = Color.White,
                            fontWeight = FontWeight.Black,
                            style = MaterialTheme.typography.titleLarge,
                        )
                    }
                    Spacer(Modifier.width(14.dp))
                    Column(modifier = Modifier.weight(1f)) {
                        Text(
                            if (loggedIn) (auth.displayName ?: "Pengguna SubSenpai") else "Belum masuk",
                            style = MaterialTheme.typography.titleMedium,
                            fontWeight = FontWeight.Bold,
                            color = Color.White,
                        )
                        Text(
                            if (loggedIn) (auth.email ?: "Kelola akun & sesi masuk") else "Masuk untuk riwayat, watchlist & premium",
                            style = MaterialTheme.typography.bodySmall,
                            color = Color(0xFFB3B3B3),
                        )
                    }
                    Icon(
                        painter = painterResource(R.drawable.ic_chevron_right),
                        contentDescription = null,
                        tint = Color(0xFFB3B3B3),
                    )
                }
            }
        }
        item { MoreSectionLabel("Menu") }
        item {
            MoreRow(R.drawable.ic_history, "Riwayat Tontonan", "Judul yang pernah kamu tonton", onOpenHistory)
        }
        item {
            MoreRow(R.drawable.ic_bookmark, "Daftar Saya", "Watchlist tersimpan", onOpenWatchlist)
        }
        item {
            MoreRow(R.drawable.ic_phone, "Perangkat", "Perangkat yang login ke akun ini", onOpenDevices)
        }
        item {
            MoreRow(R.drawable.ic_star, "Premium", "Paket & langganan", onOpenPremium)
        }
        item {
            MoreRow(R.drawable.ic_receipt, "Riwayat Transaksi", "Status pembayaran premium & voucher", onOpenTransactions)
        }
        item {
            MoreRow(R.drawable.ic_ticket, "Voucher Saya", "Kode voucher untuk dibagikan & ditukar", onOpenMyVouchers)
        }
        item {
            MoreRow(R.drawable.ic_share, "Undang Teman", "Undang teman, berdua dapat Premium 3 hari", onOpenReferral)
        }
        item {
            MoreRow(R.drawable.ic_person, "Teman", "Kode teman & daftar teman", onOpenFriends)
        }
        item {
            MoreRow(R.drawable.ic_chat, "Grup Chat", "Grup & obrolan global", onOpenGroups)
        }
        item { MoreSectionLabel("Komunitas") }
        item {
            MoreRow(R.drawable.ic_discord, "Discord", "Gabung server Discord SubSenpai") {
                context.startActivity(Intent(Intent.ACTION_VIEW, Uri.parse("https://discord.gg/yJebKEGNh")))
            }
        }
        item {
            MoreRow(R.drawable.ic_telegram, "Telegram", "Gabung channel Telegram SubSenpai") {
                context.startActivity(Intent(Intent.ACTION_VIEW, Uri.parse("https://t.me/subsenpaianime")))
            }
        }
        item { MoreSectionLabel("Umum") }
        item {
            MoreRow(R.drawable.ic_settings, "Pengaturan", "Kualitas poster, penyimpanan, tentang", onOpenSettings)
        }
        item {
            Spacer(Modifier.height(28.dp))
            Text(
                "SubSenpai v$version",
                style = MaterialTheme.typography.bodySmall,
                color = Color(0xFF808080),
                modifier = Modifier.fillMaxWidth().padding(bottom = 20.dp),
                textAlign = androidx.compose.ui.text.style.TextAlign.Center,
            )
        }
    }
}

@Composable
private fun MoreSectionLabel(text: String) {
    Text(
        text.uppercase(),
        style = MaterialTheme.typography.labelMedium,
        color = Color(0xFF808080),
        modifier = Modifier.padding(start = 16.dp, end = 16.dp, top = 22.dp, bottom = 6.dp),
    )
}

@Composable
private fun MoreRow(icon: Int, label: String, subtitle: String, onClick: () -> Unit) {
    Row(
        modifier = Modifier.fillMaxWidth().clickable { onClick() }
            .padding(horizontal = 16.dp, vertical = 12.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Icon(
            painter = painterResource(icon),
            contentDescription = null,
            tint = Color.White,
            modifier = Modifier.size(24.dp),
        )
        Spacer(Modifier.width(16.dp))
        Column(modifier = Modifier.weight(1f)) {
            Text(label, style = MaterialTheme.typography.bodyLarge, color = Color.White)
            Text(subtitle, style = MaterialTheme.typography.bodySmall, color = Color(0xFFB3B3B3))
        }
        Icon(
            painter = painterResource(R.drawable.ic_chevron_right),
            contentDescription = null,
            tint = Color(0xFF808080),
        )
    }
}

/** Halaman Riwayat Tontonan penuh (dari menu Lainnya). */
@Composable
fun HistoryScreen(
    repo: CatalogRepository,
    onOpenEpisode: (String) -> Unit,
    onNeedLogin: () -> Unit,
) {
    var items by remember { mutableStateOf<List<WatchHistoryItem>>(emptyList()) }
    var loaded by remember { mutableStateOf(false) }
    var error by remember { mutableStateOf<String?>(null) }
    LaunchedEffect(Unit) {
        if (repo.isLoggedIn) {
            try {
                items = repo.watchHistory(50)
            } catch (e: Exception) {
                error = e.message
            }
        }
        loaded = true
    }
    LazyColumn(
        modifier = Modifier.fillMaxSize(),
        verticalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        item {
            Row(
                modifier = Modifier.fillMaxWidth().padding(horizontal = 4.dp, vertical = 8.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text(
                    "Riwayat Tontonan",
                    style = MaterialTheme.typography.titleLarge,
                    fontWeight = FontWeight.Bold,
                )
            }
        }
        if (!repo.isLoggedIn) {
            item {
                Column(modifier = Modifier.padding(horizontal = 16.dp)) {
                    Text("Masuk dulu untuk melihat riwayat tontonanmu.")
                    Spacer(Modifier.height(10.dp))
                    Button(onClick = onNeedLogin) { Text("Masuk") }
                }
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
        if (loaded && repo.isLoggedIn && items.isEmpty() && error == null) {
            item {
                Text(
                    "Belum ada riwayat. Judul yang kamu tonton akan muncul di sini.",
                    modifier = Modifier.padding(horizontal = 16.dp),
                )
            }
        }
        items(items) { h ->
            Card(
                modifier = Modifier.fillMaxWidth().padding(horizontal = 16.dp)
                    .clickable { onOpenEpisode(h.episodeId) },
                colors = CardDefaults.cardColors(containerColor = Color(0xFF1F1F1F)),
                shape = RoundedCornerShape(8.dp),
            ) {
                Row(modifier = Modifier.padding(10.dp), verticalAlignment = Alignment.CenterVertically) {
                    PosterImage(
                        url = h.posterUrl,
                        title = h.title,
                        modifier = Modifier.width(52.dp).aspectRatio(2f / 3f)
                            .clip(RoundedCornerShape(6.dp)),
                    )
                    Spacer(Modifier.width(12.dp))
                    Column(modifier = Modifier.weight(1f)) {
                        Text(
                            h.title,
                            style = MaterialTheme.typography.bodyLarge,
                            color = Color.White,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                        )
                        Text(
                            "Episode ${h.episodeNo} · terakhir di ${h.positionSec / 60}:${(h.positionSec % 60).toString().padStart(2, '0')}",
                            style = MaterialTheme.typography.bodySmall,
                            color = Color(0xFFB3B3B3),
                        )
                    }
                    Icon(
                        painter = painterResource(R.drawable.ic_play),
                        contentDescription = "Lanjutkan",
                        tint = Color.White,
                        modifier = Modifier.size(26.dp),
                    )
                }
            }
        }
        item { Spacer(Modifier.height(20.dp)) }
    }
}

/** Pengaturan: kualitas poster, penyimpanan (cache), tentang. */
@Composable
fun SettingsScreen(onBack: () -> Unit) {
    val context = LocalContext.current
    val settings = remember { SettingsStore(context) }
    var hiRes by remember { mutableStateOf(settings.posterHiRes) }
    var cacheMsg by remember { mutableStateOf<String?>(null) }
    val scope = rememberCoroutineScope()
    val version = remember {
        try {
            context.packageManager.getPackageInfo(context.packageName, 0).versionName ?: ""
        } catch (e: Exception) {
            ""
        }
    }
    LazyColumn(modifier = Modifier.fillMaxSize()) {
        item {
            Row(
                modifier = Modifier.fillMaxWidth().padding(horizontal = 4.dp, vertical = 8.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                TextButton(onClick = onBack) { Text("←", color = Color.White) }
                Text(
                    "Pengaturan",
                    style = MaterialTheme.typography.titleLarge,
                    fontWeight = FontWeight.Bold,
                )
            }
        }
        item { MoreSectionLabel("Kualitas Poster") }
        item {
            SettingRadioRow(
                selected = hiRes,
                title = "Tajam",
                subtitle = "Poster 780px di hero & halaman detail — lebih jernih, unduhan sedikit lebih besar",
                onClick = {
                    hiRes = true
                    settings.posterHiRes = true
                },
            )
        }
        item {
            SettingRadioRow(
                selected = !hiRes,
                title = "Hemat data",
                subtitle = "Poster 500px di semua tempat — lebih ringan",
                onClick = {
                    hiRes = false
                    settings.posterHiRes = false
                },
            )
        }
        item { MoreSectionLabel("Penyimpanan") }
        item {
            Row(
                modifier = Modifier.fillMaxWidth().clickable {
                    scope.launch {
                        val loader = Coil.imageLoader(context)
                        loader.memoryCache?.clear()
                        withContext(Dispatchers.IO) { loader.diskCache?.clear() }
                        cacheMsg = "Cache gambar dibersihkan."
                    }
                }.padding(horizontal = 16.dp, vertical = 12.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Icon(
                    painter = painterResource(R.drawable.ic_delete),
                    contentDescription = null,
                    tint = Color.White,
                    modifier = Modifier.size(24.dp),
                )
                Spacer(Modifier.width(16.dp))
                Column {
                    Text("Hapus cache gambar", style = MaterialTheme.typography.bodyLarge, color = Color.White)
                    Text(
                        cacheMsg ?: "Poster yang tersimpan di perangkat akan diunduh ulang saat dibutuhkan",
                        style = MaterialTheme.typography.bodySmall,
                        color = Color(0xFFB3B3B3),
                    )
                }
            }
        }
        item { MoreSectionLabel("Tentang") }
        item {
            MoreRow(R.drawable.ic_info, "Versi aplikasi", "SubSenpai v$version") {}
        }
        item { Spacer(Modifier.height(20.dp)) }
    }
}

@Composable
private fun SettingRadioRow(selected: Boolean, title: String, subtitle: String, onClick: () -> Unit) {
    Row(
        modifier = Modifier.fillMaxWidth().clickable { onClick() }
            .padding(horizontal = 16.dp, vertical = 10.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        RadioButton(selected = selected, onClick = onClick)
        Spacer(Modifier.width(8.dp))
        Column {
            Text(title, style = MaterialTheme.typography.bodyLarge, color = Color.White)
            Text(subtitle, style = MaterialTheme.typography.bodySmall, color = Color(0xFFB3B3B3))
        }
    }
}
