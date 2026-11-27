package com.subsenpai.ui

import android.widget.Toast
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
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
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import com.subsenpai.R
import com.subsenpai.data.CatalogRepository
import com.subsenpai.data.DeviceInfo
import kotlinx.coroutines.launch

/** "Terakhir aktif" relatif dari ISO-8601 backend (UTC). */
private fun lastSeenText(iso: String?): String {
    if (iso.isNullOrBlank()) return "Tidak diketahui"
    return try {
        val fmt = java.text.SimpleDateFormat("yyyy-MM-dd'T'HH:mm:ss", java.util.Locale.US)
        fmt.timeZone = java.util.TimeZone.getTimeZone("UTC")
        val t = fmt.parse(iso.take(19))?.time ?: return "Tidak diketahui"
        android.text.format.DateUtils.getRelativeTimeSpanString(
            t,
            System.currentTimeMillis(),
            android.text.format.DateUtils.MINUTE_IN_MILLIS,
        ).toString()
    } catch (e: Exception) {
        "Tidak diketahui"
    }
}

/**
 * Halaman Perangkat (batas login, cerminan /perangkat di web):
 * daftar perangkat yang login ke akun ini, perangkat ini ditandai,
 * perangkat lain bisa dicabut (sesinya berakhir). Tamu diarahkan
 * masuk dulu. Batas: Premium maks 2 perangkat, gratis maks 1 —
 * login di perangkat lain mengeluarkan perangkat yang paling lama
 * tidak aktif secara otomatis.
 */
@Composable
fun DevicesScreen(
    repo: CatalogRepository,
    onBack: () -> Unit,
    onNeedLogin: () -> Unit,
) {
    val context = LocalContext.current
    val scope = rememberCoroutineScope()
    val loggedIn = repo.isLoggedIn
    val myDeviceId = repo.auth?.deviceId
    var devices by remember { mutableStateOf<List<DeviceInfo>>(emptyList()) }
    var error by remember { mutableStateOf<String?>(null) }
    var loaded by remember { mutableStateOf(false) }
    var reloadKey by remember { mutableStateOf(0) }
    var confirmDevice by remember { mutableStateOf<DeviceInfo?>(null) }
    var revoking by remember { mutableStateOf(false) }

    LaunchedEffect(reloadKey) {
        if (!loggedIn) {
            loaded = true
            return@LaunchedEffect
        }
        loaded = false
        error = null
        try {
            devices = repo.devices()
        } catch (e: Exception) {
            error = friendlyInlineError(e)
        } finally {
            loaded = true
        }
    }

    LazyColumn(
        modifier = Modifier.fillMaxSize().padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        item { TextButton(onClick = onBack) { Text("← Kembali") } }
        item { Text("Perangkat", style = MaterialTheme.typography.headlineSmall) }
        if (!loggedIn) {
            item {
                Text("Masuk untuk melihat perangkat yang login ke akun ini.")
            }
            item {
                Button(
                    onClick = onNeedLogin,
                    colors = ButtonDefaults.buttonColors(containerColor = NetflixRed),
                ) { Text("Masuk") }
            }
            return@LazyColumn
        }
        item {
            Text(
                "Premium: maks 2 perangkat • Gratis: maks 1 perangkat. " +
                    "Kalau login di perangkat lain, perangkat yang paling lama " +
                    "tidak aktif dikeluarkan otomatis.",
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
        if (!loaded) {
            item {
                Row(
                    modifier = Modifier.fillMaxWidth().padding(vertical = 24.dp),
                    horizontalArrangement = Arrangement.Center,
                ) { CircularProgressIndicator(color = NetflixRed) }
            }
        }
        if (error != null) {
            item { Text("Gagal memuat: $error", color = MaterialTheme.colorScheme.error) }
            item {
                OutlinedButton(onClick = { reloadKey++ }) { Text("Coba lagi") }
            }
        }
        if (loaded && devices.isEmpty() && error == null) {
            item { Text("Belum ada perangkat tercatat — perangkat ini tercatat saat kamu masuk.") }
        }
        items(devices) { d ->
            val isMine = myDeviceId != null && d.deviceId == myDeviceId
            val name = d.deviceName ?: "Perangkat ${d.deviceId.take(8)}"
            Card(
                modifier = Modifier.fillMaxWidth(),
                colors = CardDefaults.cardColors(containerColor = Color(0xFF1F1F1F)),
                shape = RoundedCornerShape(10.dp),
            ) {
                Row(
                    modifier = Modifier.padding(16.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Icon(
                        painter = painterResource(R.drawable.ic_phone),
                        contentDescription = null,
                        tint = Color(0xFFB3B3B3),
                        modifier = Modifier.size(28.dp),
                    )
                    Spacer(Modifier.width(14.dp))
                    Column(modifier = Modifier.weight(1f)) {
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            Text(
                                name,
                                style = MaterialTheme.typography.titleMedium,
                                fontWeight = FontWeight.Bold,
                                color = Color.White,
                            )
                            if (isMine) {
                                Spacer(Modifier.width(8.dp))
                                Text(
                                    "Perangkat ini",
                                    style = MaterialTheme.typography.labelSmall,
                                    color = NetflixRed,
                                )
                            }
                        }
                        Spacer(Modifier.height(2.dp))
                        Text(
                            "Terakhir aktif: ${lastSeenText(d.lastSeenAt)}",
                            style = MaterialTheme.typography.bodySmall,
                            color = Color(0xFFB3B3B3),
                        )
                    }
                    if (!isMine) {
                        OutlinedButton(
                            onClick = { confirmDevice = d },
                            colors = ButtonDefaults.outlinedButtonColors(contentColor = Color(0xFFE50914)),
                        ) { Text("Cabut") }
                    }
                }
            }
        }
    }

    val target = confirmDevice
    if (target != null) {
        val targetName = target.deviceName ?: "Perangkat ${target.deviceId.take(8)}"
        AlertDialog(
            onDismissRequest = { if (!revoking) confirmDevice = null },
            title = { Text("Cabut perangkat?") },
            text = {
                Text("$targetName akan dikeluarkan — akun di perangkat itu harus masuk lagi.")
            },
            confirmButton = {
                TextButton(
                    onClick = {
                        revoking = true
                        scope.launch {
                            try {
                                repo.revokeDevice(target.deviceId)
                                Toast.makeText(context, "Perangkat dicabut", Toast.LENGTH_SHORT).show()
                                reloadKey++
                            } catch (e: Exception) {
                                Toast.makeText(
                                    context,
                                    friendlyInlineError(e),
                                    Toast.LENGTH_LONG,
                                ).show()
                            } finally {
                                revoking = false
                                confirmDevice = null
                            }
                        }
                    },
                    enabled = !revoking,
                ) { Text("Cabut", color = Color(0xFFE50914)) }
            },
            dismissButton = {
                TextButton(
                    onClick = { confirmDevice = null },
                    enabled = !revoking,
                ) { Text("Batal") }
            },
        )
    }
}
