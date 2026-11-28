package com.subsenpai.ui

import android.content.Intent
import androidx.compose.foundation.layout.Arrangement
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
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
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
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalClipboardManager
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import com.subsenpai.data.CatalogRepository
import com.subsenpai.data.VoucherItem
import kotlinx.coroutines.launch

/**
 * Halaman Undang Teman (referral, cermin /undang-teman di web):
 * kode teman = kode referral. Backend memberi voucher 3 hari ke
 * pengundang + pendaftar saat akun baru login dengan referral_code
 * (auth.go); aplikasi hanya menyediakan tautan undangan, statistik,
 * dan penukaran voucher. Statistik dihitung dari
 * GET /api/v1/vouchers/mine (source 'referral'): satu voucher yang
 * dibuat orang lain untukku = satu teman yang berhasil daftar lewat
 * kodeku.
 */
@Composable
fun ReferralScreen(
    repo: CatalogRepository,
    onBack: () -> Unit,
    onNeedLogin: () -> Unit,
) {
    val context = LocalContext.current
    val clipboard = LocalClipboardManager.current
    val scope = rememberCoroutineScope()
    var vouchers by remember { mutableStateOf<List<VoucherItem>>(emptyList()) }
    var info by remember { mutableStateOf<String?>(null) }
    var error by remember { mutableStateOf<String?>(null) }
    var refresh by remember { mutableStateOf(0) }
    var redeemBusy by remember { mutableStateOf<String?>(null) }

    val loggedIn = repo.isLoggedIn
    val friendCode = repo.auth?.friendCode.orEmpty()
    val inviteLink =
        if (friendCode.isNotEmpty()) "https://subsenpai.web.id/masuk?ref=$friendCode" else ""

    LaunchedEffect(loggedIn, refresh) {
        if (!loggedIn) return@LaunchedEffect
        try {
            vouchers = repo.myVouchers()
        } catch (e: Exception) {
            error = e.message
        }
    }

    val referralVouchers = vouchers.filter { it.source == "referral" }
    val joinedCount = referralVouchers.count { !it.createdByMe && it.forMe }
    val daysEarned = referralVouchers
        .filter { !it.createdByMe && it.forMe && it.status == "redeemed" }
        .sumOf { it.days }

    fun copy(text: String, label: String) {
        clipboard.setText(AnnotatedString(text))
        info = "$label disalin."
    }

    fun shareInvite() {
        val send = Intent(Intent.ACTION_SEND).apply {
            type = "text/plain"
            putExtra(
                Intent.EXTRA_TEXT,
                "Nonton anime, donghua & film subtitle Indonesia di SubSenpai — " +
                    "daftar pakai tautanku dan kita berdua dapat Premium 3 hari gratis! $inviteLink",
            )
        }
        context.startActivity(Intent.createChooser(send, "Bagikan tautan undangan"))
    }

    LazyColumn(
        modifier = Modifier.fillMaxSize().padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        item { TextButton(onClick = onBack) { Text("← Kembali") } }
        item { Text("Undang Teman", style = MaterialTheme.typography.headlineSmall) }
        if (info != null) item { Text(info!!) }
        if (error != null) item { Text(error!!, color = MaterialTheme.colorScheme.error) }
        if (!loggedIn) {
            item {
                Text("Masuk dulu untuk melihat kode referral kamu.")
                Spacer(Modifier.height(8.dp))
                Button(onClick = onNeedLogin, modifier = Modifier.fillMaxWidth()) {
                    Text("Masuk dengan Google")
                }
            }
        } else {
            item {
                Card(
                    modifier = Modifier.fillMaxWidth(),
                    colors = CardDefaults.cardColors(containerColor = Color(0xFF1F1F1F)),
                ) {
                    Column(modifier = Modifier.padding(16.dp)) {
                        Text(
                            "Undang teman, berdua dapat Premium 3 hari",
                            style = MaterialTheme.typography.titleMedium,
                            fontWeight = FontWeight.Bold,
                        )
                        Spacer(Modifier.height(6.dp))
                        Text(
                            "Bagikan tautan undanganmu. Teman daftar pakai Google lewat tautan itu, " +
                                "kalian berdua langsung dapat voucher Premium 3 hari.",
                            style = MaterialTheme.typography.bodyMedium,
                            color = Color(0xFFB3B3B3),
                        )
                    }
                }
            }
            if (friendCode.isNotEmpty()) {
                item {
                    Card(
                        modifier = Modifier.fillMaxWidth(),
                        colors = CardDefaults.cardColors(containerColor = Color(0xFF1F1F1F)),
                    ) {
                        Column(modifier = Modifier.padding(16.dp)) {
                            Text("Kode referral kamu", style = MaterialTheme.typography.bodySmall, color = Color(0xFFB3B3B3))
                            Text(friendCode, style = MaterialTheme.typography.headlineMedium, fontWeight = FontWeight.Bold)
                            Spacer(Modifier.height(8.dp))
                            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                                Button(onClick = { copy(friendCode, "Kode referral") }) { Text("Salin Kode") }
                                Button(onClick = { copy(inviteLink, "Tautan undangan") }) { Text("Salin Tautan") }
                            }
                            Spacer(Modifier.height(8.dp))
                            Text(inviteLink, style = MaterialTheme.typography.bodySmall, color = Color(0xFFB3B3B3))
                            Spacer(Modifier.height(8.dp))
                            Button(onClick = { shareInvite() }, modifier = Modifier.fillMaxWidth()) {
                                Text("Bagikan")
                            }
                        }
                    }
                }
                item {
                    Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                        Card(
                            modifier = Modifier.weight(1f),
                            colors = CardDefaults.cardColors(containerColor = Color(0xFF1F1F1F)),
                        ) {
                            Column(
                                modifier = Modifier.padding(16.dp),
                                horizontalAlignment = androidx.compose.ui.Alignment.CenterHorizontally,
                            ) {
                                Text("$joinedCount", style = MaterialTheme.typography.headlineMedium, fontWeight = FontWeight.Bold)
                                Text("teman bergabung", style = MaterialTheme.typography.bodySmall, color = Color(0xFFB3B3B3))
                            }
                        }
                        Card(
                            modifier = Modifier.weight(1f),
                            colors = CardDefaults.cardColors(containerColor = Color(0xFF1F1F1F)),
                        ) {
                            Column(
                                modifier = Modifier.padding(16.dp),
                                horizontalAlignment = androidx.compose.ui.Alignment.CenterHorizontally,
                            ) {
                                Text("$daysEarned hari", style = MaterialTheme.typography.headlineMedium, fontWeight = FontWeight.Bold)
                                Text("Premium dari undangan", style = MaterialTheme.typography.bodySmall, color = Color(0xFFB3B3B3))
                            }
                        }
                    }
                }
            }
            item {
                Text("Voucher referral kamu", style = MaterialTheme.typography.titleMedium)
            }
            if (referralVouchers.isEmpty()) {
                item {
                    Text(
                        "Belum ada voucher referral. Bagikan tautanmu — tiap teman yang daftar " +
                            "lewat tautan itu memberimu satu voucher Premium 3 hari.",
                        color = Color(0xFFB3B3B3),
                    )
                }
            }
            items(referralVouchers) { v ->
                val statusLabel = when (v.status) {
                    "active" -> "Aktif"
                    "redeemed" -> "Ditukar"
                    "expired" -> "Kedaluwarsa"
                    else -> v.status
                }
                Card(
                    modifier = Modifier.fillMaxWidth(),
                    colors = CardDefaults.cardColors(containerColor = Color(0xFF1F1F1F)),
                ) {
                    Column(modifier = Modifier.padding(14.dp)) {
                        Text(v.code, style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.Bold)
                        Text(
                            "Premium ${v.days} hari • " +
                                if (v.createdByMe && v.forMe) {
                                    "Voucher sambutanmu (kamu daftar pakai kode teman)"
                                } else {
                                    "Dari teman yang daftar lewat tautanmu"
                                },
                            color = Color(0xFFB3B3B3),
                            style = MaterialTheme.typography.bodySmall,
                        )
                        Text(statusLabel, style = MaterialTheme.typography.bodySmall)
                        if (v.status == "active" && v.forMe) {
                            Spacer(Modifier.height(6.dp))
                            Button(
                                enabled = redeemBusy == null,
                                onClick = {
                                    scope.launch {
                                        redeemBusy = v.code
                                        try {
                                            val o = repo.redeemVoucher(v.code)
                                            info = "Voucher ditukar! Premium sampai " +
                                                o.optString("premium_until").take(10)
                                            refresh++
                                        } catch (e: Exception) {
                                            error = e.message
                                        } finally {
                                            redeemBusy = null
                                        }
                                    }
                                },
                            ) { Text(if (redeemBusy == v.code) "Menukar…" else "Tukar") }
                        }
                    }
                }
            }
            item {
                Text(
                    "Voucher referral juga bisa ditukar di halaman Voucher Saya. " +
                        "Satu teman = satu voucher; kode tidak bisa dipakai untuk akun sendiri.",
                    style = MaterialTheme.typography.bodySmall,
                    color = Color(0xFFB3B3B3),
                )
            }
        }
    }
}
