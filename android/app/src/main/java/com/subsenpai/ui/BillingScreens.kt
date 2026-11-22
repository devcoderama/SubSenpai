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
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import com.subsenpai.data.CatalogRepository
import com.subsenpai.data.TransactionItem
import com.subsenpai.data.VoucherItem
import kotlinx.coroutines.launch
import java.text.NumberFormat
import java.util.Locale

private fun rp(n: Int): String =
    "Rp" + NumberFormat.getNumberInstance(Locale("id", "ID")).format(n)

private fun statusLabel(status: String): String = when (status) {
    "paid" -> "Lunas"
    "pending" -> "Menunggu pembayaran"
    "expired" -> "Kedaluwarsa"
    else -> status
}

/**
 * Riwayat Transaksi (Lainnya): sesi pembayaran user + tombol Cek
 * Pembayaran untuk yang masih pending (server menanyakan gateway
 * dan mengaktifkan sesi bila ternyata sudah lunas).
 */
@Composable
fun TransactionsScreen(repo: CatalogRepository, onBack: () -> Unit) {
    val scope = rememberCoroutineScope()
    var items by remember { mutableStateOf<List<TransactionItem>>(emptyList()) }
    var info by remember { mutableStateOf<String?>(null) }
    var error by remember { mutableStateOf<String?>(null) }
    var refresh by remember { mutableStateOf(0) }
    var checkingId by remember { mutableStateOf<String?>(null) }

    LaunchedEffect(refresh) {
        try {
            items = repo.transactions()
        } catch (e: Exception) {
            error = e.message
        }
    }

    LazyColumn(
        modifier = Modifier.fillMaxSize().padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        item {
            TextButton(onClick = onBack) { Text("← Kembali") }
            Text("Riwayat Transaksi", style = MaterialTheme.typography.headlineSmall)
        }
        if (info != null) item { Text(info!!, style = MaterialTheme.typography.bodyMedium) }
        if (error != null) item { Text(error!!, color = MaterialTheme.colorScheme.error) }
        if (items.isEmpty()) {
            item {
                Text(
                    "Belum ada transaksi. Beli premium atau voucher lewat tab Premium.",
                    color = Color(0xFFB3B3B3),
                )
            }
        }
        items(items) { t ->
            Card(modifier = Modifier.fillMaxWidth()) {
                Column(modifier = Modifier.padding(14.dp)) {
                    Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
                        Text(
                            t.planLabel.ifBlank { t.purpose },
                            style = MaterialTheme.typography.titleSmall,
                            color = Color.White,
                        )
                        Text(
                            statusLabel(t.status),
                            color = when (t.status) {
                                "paid" -> Color(0xFF4CAF50)
                                "pending" -> Color(0xFFFFC107)
                                else -> Color(0xFFB3B3B3)
                            },
                            style = MaterialTheme.typography.bodySmall,
                            fontWeight = FontWeight.Bold,
                        )
                    }
                    Text(
                        (if (t.purpose == "voucher") "Beli voucher" else "Langganan") +
                            " · " + t.provider.replaceFirstChar { it.uppercase() } +
                            " · " + rp(t.amount),
                        color = Color(0xFFB3B3B3),
                        style = MaterialTheme.typography.bodySmall,
                    )
                    Text(
                        t.createdAt.take(16).replace("T", " "),
                        color = Color(0xFF808080),
                        style = MaterialTheme.typography.bodySmall,
                    )
                    if (t.status == "pending") {
                        Spacer(Modifier.height(6.dp))
                        OutlinedButton(
                            enabled = checkingId == null,
                            onClick = {
                                checkingId = t.id
                                scope.launch {
                                    try {
                                        val o = repo.paymentStatus(t.id)
                                        info = if (o.optString("status") == "paid") {
                                            "Pembayaran ${t.planLabel} terkonfirmasi lunas."
                                        } else {
                                            "Belum terdeteksi lunas — selesaikan pembayaran dulu."
                                        }
                                        refresh++
                                    } catch (e: Exception) {
                                        error = e.message
                                    } finally {
                                        checkingId = null
                                    }
                                }
                            },
                        ) { Text(if (checkingId == t.id) "Mengecek…" else "Cek Pembayaran") }
                    }
                }
            }
        }
        item { Spacer(Modifier.height(16.dp)) }
    }
}

/**
 * Voucher Saya (Lainnya): kode yang dibeli user untuk dibagikan
 * (dengan status pembayaran & penukar), dan kode yang ditujukan ke
 * user untuk ditukar sendiri.
 */
@Composable
fun MyVouchersScreen(repo: CatalogRepository, onBack: () -> Unit) {
    val context = LocalContext.current
    val scope = rememberCoroutineScope()
    var vouchers by remember { mutableStateOf<List<VoucherItem>>(emptyList()) }
    var info by remember { mutableStateOf<String?>(null) }
    var error by remember { mutableStateOf<String?>(null) }
    var refresh by remember { mutableStateOf(0) }

    LaunchedEffect(refresh) {
        try {
            vouchers = repo.myVouchers()
        } catch (e: Exception) {
            error = e.message
        }
    }

    val shareable = vouchers.filter { it.createdByMe && it.status == "active" }
    val redeemed = vouchers.filter { it.createdByMe && it.status == "redeemed" }
    val forMe = vouchers.filter { it.forMe && it.status == "active" }

    LazyColumn(
        modifier = Modifier.fillMaxSize().padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        item {
            TextButton(onClick = onBack) { Text("← Kembali") }
            Text("Voucher Saya", style = MaterialTheme.typography.headlineSmall)
        }
        if (info != null) item { Text(info!!, style = MaterialTheme.typography.bodyMedium) }
        if (error != null) item { Text(error!!, color = MaterialTheme.colorScheme.error) }
        if (vouchers.isEmpty()) {
            item {
                Text(
                    "Belum ada voucher. Beli lewat tab Premium › Beli Voucher untuk dibagikan ke teman.",
                    color = Color(0xFFB3B3B3),
                )
            }
        }
        if (shareable.isNotEmpty()) {
            item {
                Text("Untuk dibagikan", style = MaterialTheme.typography.titleMedium)
            }
            items(shareable) { v ->
                val paidOk = v.paymentStatus == null || v.paymentStatus == "paid"
                Card(modifier = Modifier.fillMaxWidth()) {
                    Column(modifier = Modifier.padding(14.dp)) {
                        Text(v.code, style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.Bold)
                        Text(
                            "Premium ${v.days} hari" +
                                if (paidOk) "" else " · menunggu pembayaran (belum bisa ditukar)",
                            color = if (paidOk) Color(0xFFB3B3B3) else Color(0xFFFFC107),
                            style = MaterialTheme.typography.bodySmall,
                        )
                        if (paidOk) {
                            Spacer(Modifier.height(6.dp))
                            Button(onClick = {
                                val send = Intent(Intent.ACTION_SEND).apply {
                                    type = "text/plain"
                                    putExtra(
                                        Intent.EXTRA_TEXT,
                                        "Kode voucher SubSenpai (${v.days} hari premium): ${v.code} — " +
                                            "tukarkan di aplikasi SubSenpai lewat Premium › Beli Voucher.",
                                    )
                                }
                                context.startActivity(Intent.createChooser(send, "Bagikan voucher"))
                            }) { Text("Bagikan") }
                        }
                    }
                }
            }
        }
        if (forMe.isNotEmpty()) {
            item {
                Text("Untuk saya", style = MaterialTheme.typography.titleMedium)
            }
            items(forMe) { v ->
                Card(modifier = Modifier.fillMaxWidth()) {
                    Column(modifier = Modifier.padding(14.dp)) {
                        Text(v.code, style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.Bold)
                        Text(
                            "Premium ${v.days} hari · dari ${v.source}",
                            color = Color(0xFFB3B3B3),
                            style = MaterialTheme.typography.bodySmall,
                        )
                        Spacer(Modifier.height(6.dp))
                        Button(onClick = {
                            scope.launch {
                                try {
                                    val o = repo.redeemVoucher(v.code)
                                    info = "Voucher ditukar! Premium sampai " +
                                        o.optString("premium_until").take(10)
                                    refresh++
                                } catch (e: Exception) {
                                    error = e.message
                                }
                            }
                        }) { Text("Tukar sekarang") }
                    }
                }
            }
        }
        if (redeemed.isNotEmpty()) {
            item {
                Text("Sudah ditukar", style = MaterialTheme.typography.titleMedium)
            }
            items(redeemed) { v ->
                Card(
                    modifier = Modifier.fillMaxWidth(),
                    colors = CardDefaults.cardColors(containerColor = Color(0xFF1A1A1A)),
                ) {
                    Column(modifier = Modifier.padding(14.dp)) {
                        Text(v.code, style = MaterialTheme.typography.titleSmall, color = Color(0xFFB3B3B3))
                        Text(
                            "Premium ${v.days} hari · ditukar oleh ${v.redeemedByName ?: "seseorang"}",
                            color = Color(0xFF808080),
                            style = MaterialTheme.typography.bodySmall,
                        )
                    }
                }
            }
        }
        item { Spacer(Modifier.height(16.dp)) }
    }
}
