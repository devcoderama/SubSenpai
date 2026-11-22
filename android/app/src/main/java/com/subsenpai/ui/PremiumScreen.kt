package com.subsenpai.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.Card
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
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import com.subsenpai.data.CatalogRepository
import com.subsenpai.data.PlanItem
import com.subsenpai.data.PremiumStatus
import com.subsenpai.data.VoucherItem
import kotlinx.coroutines.launch
import java.text.NumberFormat
import java.util.Locale

/** Layar Premium: status, trial, paket + checkout, voucher & hadiah. */
@Composable
fun PremiumScreen(repo: CatalogRepository, onBack: () -> Unit) {
    val scope = rememberCoroutineScope()
    var status by remember { mutableStateOf<PremiumStatus?>(null) }
    var plans by remember { mutableStateOf<List<PlanItem>>(emptyList()) }
    var vouchers by remember { mutableStateOf<List<VoucherItem>>(emptyList()) }
    var info by remember { mutableStateOf<String?>(null) }
    var error by remember { mutableStateOf<String?>(null) }
    var redeemCode by remember { mutableStateOf("") }
    var giftCode by remember { mutableStateOf("") }
    var giftPlan by remember { mutableStateOf<PlanItem?>(null) }
    var refresh by remember { mutableStateOf(0) }

    LaunchedEffect(refresh) {
        try {
            status = repo.premiumStatus()
            plans = repo.plans()
            if (giftPlan == null) giftPlan = plans.firstOrNull()
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
        Text("Pilih paket", style = MaterialTheme.typography.titleMedium)
        plans.forEach { plan ->
            Card(modifier = Modifier.fillMaxWidth()) {
                Column(modifier = Modifier.padding(14.dp)) {
                    Text(plan.label, style = MaterialTheme.typography.titleMedium)
                    Text("Rp" + NumberFormat.getNumberInstance(Locale("id", "ID")).format(plan.price))
                    Spacer(Modifier.height(6.dp))
                    Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        Button(onClick = {
                            act {
                                val o = repo.checkout(plan.id, "gopay", "subscription")
                                val amount = o.optInt("amount")
                                val note = o.optString("note")
                                "Sesi GoPay dibuat. Transfer TEPAT Rp" +
                                    NumberFormat.getNumberInstance(Locale("id", "ID")).format(amount) +
                                    " (nominal unik) sebelum " + o.optString("expires_at").take(16).replace("T", " ") +
                                    ". " + note
                            }
                        }) { Text("GoPay") }
                        OutlinedButton(onClick = {
                            act {
                                val o = repo.checkout(plan.id, "midtrans", "subscription")
                                "Buka tautan pembayaran Midtrans: " + o.optString("redirect_url")
                            }
                        }) { Text("Midtrans") }
                        OutlinedButton(onClick = {
                            act {
                                val o = repo.checkout(plan.id, "xendit", "subscription")
                                "Buka tautan pembayaran Xendit: " + o.optString("invoice_url")
                            }
                        }) { Text("Xendit") }
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
        Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(6.dp)) {
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
                enabled = giftCode.isNotBlank() && giftPlan != null,
                onClick = {
                    val plan = giftPlan!!
                    val code = giftCode.trim()
                    act {
                        val o = repo.checkout(plan.id, "gopay", "voucher", code)
                        "Hadiah dibuat! Kode voucher " + o.optString("voucher_code") +
                            " untuk temanmu — aktif setelah kamu transfer tepat Rp" +
                            NumberFormat.getNumberInstance(Locale("id", "ID")).format(o.optInt("amount")) +
                            " (nominal unik GoPay)."
                    }
                },
            ) { Text("Kirim hadiah") }
        }
        if (info != null) Text(info!!, style = MaterialTheme.typography.bodyMedium)
        if (error != null) Text(error!!, color = MaterialTheme.colorScheme.error)
        Spacer(Modifier.height(16.dp))
    }
}
