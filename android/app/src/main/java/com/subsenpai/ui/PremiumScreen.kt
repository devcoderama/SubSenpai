package com.subsenpai.ui

import android.content.Intent
import android.net.Uri
import androidx.compose.foundation.background
import androidx.compose.foundation.border
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
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
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
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import coil.compose.AsyncImage
import com.subsenpai.data.CatalogRepository
import com.subsenpai.data.PaymentMethodItem
import com.subsenpai.data.PlanItem
import kotlinx.coroutines.launch
import org.json.JSONObject
import java.text.NumberFormat
import java.util.Locale

private fun rp(n: Int): String =
    "Rp" + NumberFormat.getNumberInstance(Locale("id", "ID")).format(n)

private val methodGroups = listOf(
    "va" to "Virtual Account",
    "ewallet" to "E-Wallet",
    "qris" to "QRIS",
    "retail" to "Retail",
    "pulsa" to "Pulsa",
)

/** Hasil checkout yang ditampilkan di langkah pembayaran. */
private class PayResult(
    val sessionId: String,
    val totalBayar: Int,
    val nomorVa: String,
    val payUrl: String,
    val qrLink: String,
    val panduan: String,
    val voucherCodes: List<String>,
)

/**
 * Layar Premium (dirombak lagi 2026-11-22 atas permintaan Rams
 * agar tidak kepanjangan): dua kategori — Langganan Premium
 * langsung & Beli Voucher (dengan kuota jumlah orang) — lalu alur
 * bertahap: 1 pilih paket → Lanjut → 2 metode pembayaran → Bayar →
 * 3 detail pembayaran + tombol Cek Pembayaran (server juga
 * mengaktifkan otomatis lewat callback Tokopay). Masa aktif
 * premium dicek di halaman profil; riwayat & voucher ada di
 * Lainnya › Riwayat Transaksi / Voucher Saya.
 */
@Composable
fun PremiumScreen(repo: CatalogRepository, onBack: () -> Unit) {
    val context = LocalContext.current
    val scope = rememberCoroutineScope()
    var tab by remember { mutableStateOf(0) } // 0 langganan, 1 voucher
    var step by remember { mutableStateOf(1) }
    var plans by remember { mutableStateOf<List<PlanItem>>(emptyList()) }
    var methods by remember { mutableStateOf<List<PaymentMethodItem>>(emptyList()) }
    var selectedPlan by remember { mutableStateOf<PlanItem?>(null) }
    var selectedMethod by remember { mutableStateOf<PaymentMethodItem?>(null) }
    var qty by remember { mutableStateOf(1) }
    var payResult by remember { mutableStateOf<PayResult?>(null) }
    var payPaid by remember { mutableStateOf(false) }
    var checking by remember { mutableStateOf(false) }
    var busy by remember { mutableStateOf(false) }
    var info by remember { mutableStateOf<String?>(null) }
    var error by remember { mutableStateOf<String?>(null) }
    var redeemCode by remember { mutableStateOf("") }

    LaunchedEffect(Unit) {
        try {
            plans = repo.plans()
            methods = repo.paymentMethods()
            if (selectedPlan == null) selectedPlan = plans.firstOrNull()
            if (selectedMethod == null) {
                selectedMethod = methods.firstOrNull { it.code == "QRIS" } ?: methods.firstOrNull()
            }
        } catch (e: Exception) {
            error = e.message
        }
    }

    fun checkPayment() {
        val pr = payResult ?: return
        checking = true
        scope.launch {
            try {
                val o = repo.paymentStatus(pr.sessionId)
                if (o.optString("status") == "paid") {
                    payPaid = true
                    info = null
                } else {
                    info = "Pembayaran belum terdeteksi lunas. Selesaikan pembayaran, lalu cek lagi."
                }
            } catch (e: Exception) {
                error = e.message
            } finally {
                checking = false
            }
        }
    }

    // Cek sekali otomatis saat masuk langkah pembayaran.
    LaunchedEffect(payResult?.sessionId) {
        if (payResult != null && !payPaid) checkPayment()
    }

    val isVoucher = tab == 1
    val total = (selectedPlan?.price ?: 0) * (if (isVoucher) qty else 1)

    Column(
        modifier = Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        TextButton(onClick = { if (step > 1) step-- else onBack() }) { Text("← Kembali") }
        Text("Premium", style = MaterialTheme.typography.headlineSmall)

        // Kategori: langganan langsung / beli voucher.
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            listOf("Langganan Premium", "Beli Voucher").forEachIndexed { i, label ->
                Card(
                    modifier = Modifier.weight(1f)
                        .border(
                            width = if (tab == i) 2.dp else 0.dp,
                            color = if (tab == i) FeedPurple else Color.Transparent,
                            shape = RoundedCornerShape(12.dp),
                        )
                        .clickable { tab = i; step = 1; payResult = null; payPaid = false },
                    colors = CardDefaults.cardColors(
                        containerColor = if (tab == i) Color(0xFF262033) else Color(0xFF1E1B26),
                    ),
                ) {
                    Text(
                        label,
                        color = Color.White,
                        style = MaterialTheme.typography.titleSmall,
                        modifier = Modifier.fillMaxWidth().padding(vertical = 12.dp),
                        textAlign = androidx.compose.ui.text.style.TextAlign.Center,
                    )
                }
            }
        }

        Text(
            "Langkah $step dari 3 · " + when (step) {
                1 -> "Pilih paket"
                2 -> "Metode pembayaran"
                else -> "Pembayaran"
            },
            style = MaterialTheme.typography.bodySmall,
            color = Color(0xFFB3B3B3),
        )

        when (step) {
            1 -> {
                plans.forEach { plan ->
                    val selected = selectedPlan?.id == plan.id
                    Card(
                        modifier = Modifier.fillMaxWidth()
                            .border(
                                width = if (selected) 2.dp else 0.dp,
                                color = if (selected) FeedPurple else Color.Transparent,
                                shape = RoundedCornerShape(12.dp),
                            )
                            .clickable { selectedPlan = plan },
                        colors = CardDefaults.cardColors(
                            containerColor = if (selected) Color(0xFF262033) else Color(0xFF1E1B26),
                        ),
                    ) {
                        Row(
                            modifier = Modifier.fillMaxWidth().padding(14.dp),
                            verticalAlignment = Alignment.CenterVertically,
                        ) {
                            Column(modifier = Modifier.weight(1f)) {
                                Text(plan.label, style = MaterialTheme.typography.titleMedium, color = Color.White)
                                Text(rp(plan.price), color = Color(0xFFB3B3B3))
                                if (plan.days > 30) {
                                    Text(
                                        "≈ " + rp(plan.price * 30 / plan.days) + "/bulan",
                                        style = MaterialTheme.typography.bodySmall,
                                        color = FeedPurple,
                                    )
                                }
                            }
                            if (selected) {
                                Box(
                                    modifier = Modifier.size(24.dp).background(FeedPurple, CircleShape),
                                    contentAlignment = Alignment.Center,
                                ) {
                                    Text("✓", color = Color.White, style = MaterialTheme.typography.bodyMedium)
                                }
                            }
                        }
                    }
                }
                if (isVoucher) {
                    Card(modifier = Modifier.fillMaxWidth()) {
                        Column(modifier = Modifier.padding(14.dp)) {
                            Text("Mau dibagikan ke berapa orang?", style = MaterialTheme.typography.titleSmall)
                            Spacer(Modifier.height(8.dp))
                            Row(verticalAlignment = Alignment.CenterVertically) {
                                OutlinedButton(onClick = { if (qty > 1) qty-- }) { Text("−") }
                                Text(
                                    "$qty",
                                    style = MaterialTheme.typography.titleLarge,
                                    modifier = Modifier.padding(horizontal = 18.dp),
                                )
                                OutlinedButton(onClick = { if (qty < 10) qty++ }) { Text("+") }
                                Spacer(Modifier.width(10.dp))
                                Text("kode voucher", color = Color(0xFFB3B3B3))
                            }
                            Spacer(Modifier.height(6.dp))
                            Text(
                                "Total: ${rp(total)} — $qty kode × ${selectedPlan?.days ?: 0} hari premium. " +
                                    "Kode bisa dibagikan ke siapa saja dan dipakai siapa pun yang menukarkannya.",
                                style = MaterialTheme.typography.bodySmall,
                                color = Color(0xFFB3B3B3),
                            )
                        }
                    }
                }
                Button(
                    enabled = selectedPlan != null,
                    onClick = { step = 2 },
                    modifier = Modifier.fillMaxWidth(),
                ) { Text("Lanjut") }
                if (isVoucher) {
                    Spacer(Modifier.height(4.dp))
                    Text("Punya kode voucher?", style = MaterialTheme.typography.titleSmall)
                    Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        OutlinedTextField(
                            value = redeemCode,
                            onValueChange = { redeemCode = it },
                            label = { Text("Kode voucher") },
                            modifier = Modifier.weight(1f),
                            singleLine = true,
                        )
                        Button(
                            enabled = redeemCode.isNotBlank(),
                            onClick = {
                                val code = redeemCode.trim()
                                scope.launch {
                                    error = null
                                    try {
                                        val o = repo.redeemVoucher(code)
                                        redeemCode = ""
                                        info = "Voucher ditukar! Premium sampai " +
                                            o.optString("premium_until").take(10)
                                    } catch (e: Exception) {
                                        error = e.message
                                    }
                                }
                            },
                        ) { Text("Tukar") }
                    }
                }
            }

            2 -> {
                selectedPlan?.let { plan ->
                    Text(
                        (if (isVoucher) "Voucher " else "Langganan ") + plan.label +
                            (if (isVoucher) " × $qty kode" else "") + " — " + rp(total),
                        style = MaterialTheme.typography.titleSmall,
                        color = Color.White,
                    )
                }
                if (methods.isEmpty()) {
                    Text("Metode pembayaran belum tersedia. Coba lagi sebentar lagi.", color = Color(0xFFB3B3B3))
                }
                methodGroups.forEach { (group, title) ->
                    val inGroup = methods.filter { it.group == group }
                    if (inGroup.isNotEmpty()) {
                        Text(title, style = MaterialTheme.typography.titleSmall, color = Color(0xFFB3B3B3))
                        inGroup.forEach { m ->
                            val selected = selectedMethod?.code == m.code
                            Card(
                                modifier = Modifier.fillMaxWidth()
                                    .border(
                                        width = if (selected) 2.dp else 0.dp,
                                        color = if (selected) FeedPurple else Color.Transparent,
                                        shape = RoundedCornerShape(12.dp),
                                    )
                                    .clickable { selectedMethod = m },
                                colors = CardDefaults.cardColors(
                                    containerColor = if (selected) Color(0xFF262033) else Color(0xFF1E1B26),
                                ),
                            ) {
                                Row(
                                    modifier = Modifier.fillMaxWidth().padding(horizontal = 12.dp, vertical = 10.dp),
                                    verticalAlignment = Alignment.CenterVertically,
                                ) {
                                    if (m.iconUrl.isNotBlank()) {
                                        AsyncImage(
                                            model = m.iconUrl,
                                            contentDescription = m.name,
                                            modifier = Modifier.size(30.dp)
                                                .clip(RoundedCornerShape(6.dp))
                                                .background(Color.White)
                                                .padding(3.dp),
                                        )
                                    } else {
                                        Box(
                                            modifier = Modifier.size(30.dp).background(Color(0xFF3A3A44), CircleShape),
                                            contentAlignment = Alignment.Center,
                                        ) {
                                            Text(
                                                m.name.take(1),
                                                color = Color.White,
                                                fontWeight = FontWeight.Bold,
                                                style = MaterialTheme.typography.bodyMedium,
                                            )
                                        }
                                    }
                                    Spacer(Modifier.width(12.dp))
                                    Text(m.name, color = Color.White, modifier = Modifier.weight(1f))
                                    if (selected) {
                                        Text("✓", color = FeedPurple, style = MaterialTheme.typography.titleMedium)
                                    }
                                }
                            }
                        }
                    }
                }
                Button(
                    enabled = selectedPlan != null && selectedMethod != null && !busy,
                    onClick = {
                        val plan = selectedPlan!!
                        val method = selectedMethod!!
                        busy = true
                        error = null
                        info = null
                        scope.launch {
                            try {
                                val o = repo.checkout(
                                    plan.id, "tokopay",
                                    if (isVoucher) "voucher" else "subscription",
                                    channel = method.code,
                                    quantity = if (isVoucher) qty else 1,
                                )
                                val codes = mutableListOf<String>()
                                val arr = o.optJSONArray("voucher_codes")
                                if (arr != null) for (i in 0 until arr.length()) codes.add(arr.getString(i))
                                payResult = PayResult(
                                    sessionId = o.optString("session_id"),
                                    totalBayar = o.optInt("total_bayar", o.optInt("amount")),
                                    nomorVa = o.optString("nomor_va"),
                                    payUrl = o.optString("pay_url"),
                                    qrLink = o.optString("qr_link"),
                                    panduan = o.optString("panduan"),
                                    voucherCodes = codes,
                                )
                                payPaid = false
                                step = 3
                            } catch (e: Exception) {
                                error = e.message
                            } finally {
                                busy = false
                            }
                        }
                    },
                    modifier = Modifier.fillMaxWidth(),
                ) { Text("Bayar ${rp(total)}") }
            }

            else -> {
                val pr = payResult
                if (pr == null) {
                    Text("Belum ada pembayaran yang dibuat.", color = Color(0xFFB3B3B3))
                } else {
                    Card(modifier = Modifier.fillMaxWidth()) {
                        Column(modifier = Modifier.padding(14.dp)) {
                            Text("Selesaikan pembayaran", style = MaterialTheme.typography.titleMedium)
                            Text("Total bayar: ${rp(pr.totalBayar)}")
                            if (pr.nomorVa.isNotBlank()) {
                                Text(
                                    "Nomor VA / kode bayar: ${pr.nomorVa}",
                                    style = MaterialTheme.typography.titleMedium,
                                    fontWeight = FontWeight.Bold,
                                )
                            }
                            if (pr.panduan.isNotBlank()) {
                                Text(pr.panduan, style = MaterialTheme.typography.bodySmall)
                            }
                            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                                if (pr.payUrl.isNotBlank()) {
                                    Button(onClick = {
                                        context.startActivity(Intent(Intent.ACTION_VIEW, Uri.parse(pr.payUrl)))
                                    }) { Text("Buka halaman bayar") }
                                }
                                if (pr.qrLink.isNotBlank()) {
                                    OutlinedButton(onClick = {
                                        context.startActivity(Intent(Intent.ACTION_VIEW, Uri.parse(pr.qrLink)))
                                    }) { Text("Lihat QR") }
                                }
                            }
                        }
                    }
                    if (payPaid) {
                        Card(
                            modifier = Modifier.fillMaxWidth(),
                            colors = CardDefaults.cardColors(containerColor = Color(0xFF14331F)),
                        ) {
                            Column(modifier = Modifier.padding(14.dp)) {
                                Text("✓ Pembayaran berhasil!", color = Color.White, fontWeight = FontWeight.Bold)
                                Text(
                                    if (isVoucher) {
                                        "Pembayaran lunas — ${pr.voucherCodes.size} kode voucher aktif. " +
                                            "Lihat & bagikan di Lainnya › Voucher Saya."
                                    } else {
                                        "Premium kamu sudah aktif. Cek masa aktifnya di halaman profil " +
                                            "(Lainnya › kartu akun)."
                                    },
                                    color = Color(0xFFB3B3B3),
                                    style = MaterialTheme.typography.bodySmall,
                                )
                            }
                        }
                    } else {
                        OutlinedButton(
                            enabled = !checking,
                            onClick = { checkPayment() },
                            modifier = Modifier.fillMaxWidth(),
                        ) { Text(if (checking) "Mengecek…" else "Sudah bayar? Cek Pembayaran") }
                        Text(
                            "Status juga tercatat di Lainnya › Riwayat Transaksi. " +
                                "Server mengaktifkan otomatis lewat callback Tokopay begitu pembayaran masuk.",
                            style = MaterialTheme.typography.bodySmall,
                            color = Color(0xFFB3B3B3),
                        )
                    }
                    Button(onClick = onBack, modifier = Modifier.fillMaxWidth()) { Text("Selesai") }
                }
            }
        }

        if (info != null) Text(info!!, style = MaterialTheme.typography.bodyMedium)
        if (error != null) Text(error!!, color = MaterialTheme.colorScheme.error)
        Spacer(Modifier.height(16.dp))
    }
}
