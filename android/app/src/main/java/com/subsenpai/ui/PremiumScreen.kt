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
import androidx.compose.foundation.horizontalScroll
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
import com.subsenpai.data.PremiumStatus
import com.subsenpai.data.VoucherItem
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

/** Hasil checkout Tokopay yang ditampilkan sebagai kartu bayar. */
private class PayResult(
    val totalBayar: Int,
    val nomorVa: String,
    val payUrl: String,
    val qrLink: String,
    val panduan: String,
    val voucherCode: String,
)

/**
 * Layar Premium (dirombak 2026-11-22 atas permintaan Rams): pilih
 * kartu paket, lalu pilih metode pembayaran Tokopay — daftar
 * metodenya dari database (migrasi 0015) lengkap dengan ikon, bisa
 * diaktifkan/dinonaktifkan admin. Tombol GoPay/Tokopay per paket
 * yang lama dihapus; satu tombol Bayar di bawah.
 */
@Composable
fun PremiumScreen(repo: CatalogRepository, onBack: () -> Unit) {
    val context = LocalContext.current
    val scope = rememberCoroutineScope()
    var status by remember { mutableStateOf<PremiumStatus?>(null) }
    var plans by remember { mutableStateOf<List<PlanItem>>(emptyList()) }
    var methods by remember { mutableStateOf<List<PaymentMethodItem>>(emptyList()) }
    var vouchers by remember { mutableStateOf<List<VoucherItem>>(emptyList()) }
    var info by remember { mutableStateOf<String?>(null) }
    var error by remember { mutableStateOf<String?>(null) }
    var redeemCode by remember { mutableStateOf("") }
    var giftCode by remember { mutableStateOf("") }
    var giftPlan by remember { mutableStateOf<PlanItem?>(null) }
    var selectedPlan by remember { mutableStateOf<PlanItem?>(null) }
    var selectedMethod by remember { mutableStateOf<PaymentMethodItem?>(null) }
    var payResult by remember { mutableStateOf<PayResult?>(null) }
    var busy by remember { mutableStateOf(false) }
    var refresh by remember { mutableStateOf(0) }

    LaunchedEffect(refresh) {
        try {
            status = repo.premiumStatus()
            plans = repo.plans()
            methods = repo.paymentMethods()
            if (selectedPlan == null) selectedPlan = plans.firstOrNull()
            if (giftPlan == null) giftPlan = plans.firstOrNull()
            if (selectedMethod == null) {
                selectedMethod = methods.firstOrNull { it.code == "QRIS" }
                    ?: methods.firstOrNull()
            }
            vouchers = repo.myVouchers()
        } catch (e: Exception) {
            error = e.message
        }
    }

    fun act(block: suspend () -> String) {
        scope.launch {
            error = null
            info = null
            try {
                info = block()
                refresh++
            } catch (e: Exception) {
                error = e.message
            }
        }
    }

    fun showPayResult(o: JSONObject, voucherCode: String = "") {
        payResult = PayResult(
            totalBayar = o.optInt("total_bayar", o.optInt("amount")),
            nomorVa = o.optString("nomor_va"),
            payUrl = o.optString("pay_url"),
            qrLink = o.optString("qr_link"),
            panduan = o.optString("panduan"),
            voucherCode = voucherCode,
        )
    }

    Column(
        modifier = Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        TextButton(onClick = onBack) { Text("← Kembali") }
        Text("Premium", style = MaterialTheme.typography.headlineSmall)
        status?.let { s ->
            Card(modifier = Modifier.fillMaxWidth()) {
                Column(modifier = Modifier.padding(14.dp)) {
                    Text(
                        when {
                            s.isPremium -> "Premium aktif"
                            else -> "Akun gratis"
                        },
                        style = MaterialTheme.typography.titleMedium,
                    )
                    s.premiumUntil?.let { Text("Berlaku sampai: ${it.take(10)}") }
                    if (s.trialAvailable) {
                        Spacer(Modifier.height(6.dp))
                        Button(onClick = {
                            act { "Trial aktif! Premium 3 hari. Sampai: " + (repo.claimTrial()?.take(10) ?: "") }
                        }) { Text("Klaim trial gratis 3 hari") }
                    }
                }
            }
        }

        Text("1 · Pilih paket", style = MaterialTheme.typography.titleMedium)
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

        Text("2 · Metode pembayaran (Tokopay)", style = MaterialTheme.typography.titleMedium)
        if (methods.isEmpty()) {
            Text(
                "Metode pembayaran belum tersedia. Coba lagi sebentar lagi.",
                color = Color(0xFFB3B3B3),
            )
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
                payResult = null
                act {
                    try {
                        val o = repo.checkout(plan.id, "tokopay", "subscription", channel = method.code)
                        showPayResult(o)
                        "Order ${method.name} dibuat — selesaikan pembayaran sebelum kedaluwarsa. Premium aktif otomatis setelah pembayaran terkonfirmasi."
                    } finally {
                        busy = false
                    }
                }
            },
            modifier = Modifier.fillMaxWidth(),
        ) {
            Text(
                if (selectedPlan != null && selectedMethod != null) {
                    "Bayar ${rp(selectedPlan!!.price)} via ${selectedMethod!!.name}"
                } else {
                    "Bayar"
                },
            )
        }

        payResult?.let { pr ->
            Card(modifier = Modifier.fillMaxWidth()) {
                Column(modifier = Modifier.padding(14.dp)) {
                    Text("Selesaikan pembayaran", style = MaterialTheme.typography.titleMedium)
                    if (pr.voucherCode.isNotBlank()) {
                        Text("Kode voucher hadiah: ${pr.voucherCode} (aktif setelah lunas)")
                    }
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
        }

        Text("Voucher", style = MaterialTheme.typography.titleMedium)
        if (vouchers.isNotEmpty()) {
            Text("Voucher untukmu:", style = MaterialTheme.typography.bodyMedium)
            vouchers.forEach { v ->
                Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
                    Text("${v.code} · ${v.days} hari (${v.source})")
                    TextButton(onClick = {
                        act {
                            val o = repo.redeemVoucher(v.code)
                            "Voucher ditukar! Premium sampai " + o.optString("premium_until").take(10)
                        }
                    }) { Text("Tukar") }
                }
            }
        }
        Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
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
                    act {
                        val o = repo.redeemVoucher(code)
                        redeemCode = ""
                        "Voucher ditukar! Premium sampai " + o.optString("premium_until").take(10)
                    }
                },
            ) { Text("Tukar") }
        }
        Text("Hadiahkan premium ke teman", style = MaterialTheme.typography.titleMedium)
        Row(modifier = Modifier.fillMaxWidth().horizontalScroll(rememberScrollState()), horizontalArrangement = Arrangement.spacedBy(6.dp)) {
            plans.forEach { plan ->
                OutlinedButton(onClick = { giftPlan = plan }) {
                    Text((if (giftPlan?.id == plan.id) "✓ " else "") + plan.days + " hari")
                }
            }
        }
        Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            OutlinedTextField(
                value = giftCode,
                onValueChange = { giftCode = it },
                label = { Text("Kode teman penerima") },
                modifier = Modifier.weight(1f),
                singleLine = true,
            )
            Button(
                enabled = giftCode.isNotBlank() && giftPlan != null && selectedMethod != null && !busy,
                onClick = {
                    val plan = giftPlan!!
                    val method = selectedMethod!!
                    val code = giftCode.trim()
                    busy = true
                    payResult = null
                    act {
                        try {
                            val o = repo.checkout(plan.id, "tokopay", "voucher", code, channel = method.code)
                            showPayResult(o, o.optString("voucher_code"))
                            "Hadiah dibuat via ${method.name}! Kode voucher di atas aktif setelah pembayaran lunas."
                        } finally {
                            busy = false
                        }
                    }
                },
            ) { Text("Kirim hadiah") }
        }
        if (info != null) Text(info!!, style = MaterialTheme.typography.bodyMedium)
        if (error != null) Text(error!!, color = MaterialTheme.colorScheme.error)
        Spacer(Modifier.height(16.dp))
    }
}
