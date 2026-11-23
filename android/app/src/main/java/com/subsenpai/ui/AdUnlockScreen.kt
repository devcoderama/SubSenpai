package com.subsenpai.ui

import android.webkit.WebResourceError
import android.webkit.WebResourceRequest
import android.webkit.WebView
import android.webkit.WebViewClient
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.CircularProgressIndicator
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
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Shadow
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.viewinterop.AndroidView
import com.subsenpai.data.CatalogRepository
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

/**
 * Direct Link Monetag milik Rams (zone subsenpai.netlify.app) —
 * dibuka layar penuh di layar unlock; tayangan iklannya tercatat
 * di Monetag dari kunjungan ini.
 */
private const val AD_UNLOCK_URL = "https://uplcm.com/4/12330317"

/**
 * Layar "Buka dengan Iklan" (permintaan Rams 2026-11-23): halaman
 * iklan Monetag tampil di WebView sementara hitung mundur berjalan;
 * lama tonton ditegakkan SERVER lewat token sesi (start/complete) —
 * tombol Buka Episode baru aktif setelah hitung mundur selesai dan
 * server menerima penyelesaian sesinya. Berhasil -> langsung putar.
 */
@Composable
fun AdUnlockScreen(
    repo: CatalogRepository,
    episodeId: String,
    onUnlocked: (String) -> Unit,
    onBack: () -> Unit,
) {
    var token by remember { mutableStateOf<String?>(null) }
    var left by remember { mutableStateOf<Int?>(null) }
    var busy by remember { mutableStateOf(false) }
    var error by remember { mutableStateOf<String?>(null) }
    var attempt by remember { mutableStateOf(0) }
    // Ketahanan iklan (laporan Rams 2026-11-23 malam: iklan kadang
    // tidak muncul): status gagal muat halaman iklan + jumlah
    // percobaan muat ulang + pegangan WebView untuk muat ulang
    // manual dari pemberitahuan.
    var adFailed by remember { mutableStateOf(false) }
    var adRetries by remember { mutableStateOf(0) }
    var webViewRef by remember { mutableStateOf<WebView?>(null) }
    val scope = rememberCoroutineScope()

    LaunchedEffect(episodeId, attempt) {
        token = null
        left = null
        error = null
        try {
            val (t, w) = repo.adUnlockStart(episodeId)
            token = t
            left = w
        } catch (e: Exception) {
            error = "Fitur buka dengan iklan belum tersedia saat ini. " +
                "Coba lagi nanti, atau buka episode ini dengan Premium."
        }
    }
    LaunchedEffect(left) {
        val l = left ?: return@LaunchedEffect
        if (l > 0) {
            delay(1000)
            left = l - 1
        }
    }

    // Tata letak (permintaan Rams 2026-11-23): TANPA teks kepala di
    // atas — iklan memenuhi layar; teks hitung mundur + tombol di
    // bawah MELAYANG dengan latar TRANSPARAN di atas iklannya
    // (bayangan teks menjaga keterbacaan). Keluar lewat tombol
    // Kembali sistem.
    Box(
        modifier = Modifier.fillMaxSize()
            .background(MaterialTheme.colorScheme.background),
    ) {
        if (error == null) {
            AndroidView(
                factory = { ctx ->
                    WebView(ctx).apply {
                        webViewRef = this
                        setBackgroundColor(0xFF0D0D0F.toInt())
                        settings.javaScriptEnabled = true
                        settings.domStorageEnabled = true
                        settings.mediaPlaybackRequiresUserGesture = false
                        webViewClient = object : WebViewClient() {
                            override fun shouldOverrideUrlLoading(
                                view: WebView?,
                                request: WebResourceRequest?,
                            ): Boolean {
                                val u = request?.url?.toString().orEmpty()
                                // Skema non-http (intent://, market://,
                                // tel:) jangan melempar user keluar app.
                                return !(u.startsWith("http://") || u.startsWith("https://"))
                            }

                            override fun onReceivedError(
                                view: WebView?,
                                request: WebResourceRequest?,
                                error: WebResourceError?,
                            ) {
                                // Rantai alihan Direct Link kadang
                                // mendarat di domain pengiklan yang
                                // menolak koneksi (ERR_CONNECTION_
                                // REFUSED) — muat ulang tautan aslinya
                                // sampai 3x supaya dapat alihan yang
                                // sehat; sesudah itu tampilkan
                                // pemberitahuan ramah (hitung mundur
                                // tetap berjalan).
                                if (request?.isForMainFrame == true) {
                                    if (adRetries < 3) {
                                        adRetries++
                                        view?.postDelayed(
                                            { view.loadUrl(AD_UNLOCK_URL) },
                                            1200,
                                        )
                                    } else {
                                        adFailed = true
                                    }
                                }
                            }
                        }
                        loadUrl(AD_UNLOCK_URL)
                    }
                },
                modifier = Modifier.fillMaxSize(),
            )
            if (adFailed) {
                Column(
                    modifier = Modifier.align(Alignment.Center).padding(24.dp),
                    horizontalAlignment = Alignment.CenterHorizontally,
                ) {
                    Text(
                        "Iklan lagi tidak tersedia dari penyedianya 🙏",
                        fontWeight = FontWeight.Bold,
                        color = Color.White,
                        textAlign = TextAlign.Center,
                        style = MaterialTheme.typography.titleMedium,
                    )
                    Spacer(Modifier.height(8.dp))
                    Text(
                        "Hitung mundur tetap berjalan — episode bisa dibuka begitu hitung mundur selesai.",
                        color = Color.White.copy(alpha = 0.85f),
                        textAlign = TextAlign.Center,
                        style = MaterialTheme.typography.bodyMedium,
                    )
                    Spacer(Modifier.height(14.dp))
                    OutlinedButton(onClick = {
                        adRetries = 0
                        adFailed = false
                        webViewRef?.loadUrl(AD_UNLOCK_URL)
                    }) { Text("Muat Ulang Iklan", color = Color.White) }
                }
            }
            Column(
                modifier = Modifier.align(Alignment.BottomCenter)
                    .fillMaxWidth().padding(16.dp),
                horizontalAlignment = Alignment.CenterHorizontally,
            ) {
                when {
                    left == null -> {
                        CircularProgressIndicator(color = NetflixRed)
                        Spacer(Modifier.height(8.dp))
                        Text(
                            "Menyiapkan iklan…",
                            color = Color.White,
                            style = MaterialTheme.typography.bodyMedium.copy(
                                shadow = Shadow(
                                    color = Color.Black,
                                    offset = Offset(0f, 2f),
                                    blurRadius = 8f,
                                ),
                            ),
                        )
                    }
                    left!! > 0 -> {
                        Text(
                            "Episode terbuka dalam ${left} detik…",
                            style = MaterialTheme.typography.titleMedium.copy(
                                shadow = Shadow(
                                    color = Color.Black,
                                    offset = Offset(0f, 2f),
                                    blurRadius = 8f,
                                ),
                            ),
                            fontWeight = FontWeight.Bold,
                            color = Color.White,
                        )
                        Spacer(Modifier.height(10.dp))
                        Button(
                            onClick = {},
                            enabled = false,
                            modifier = Modifier.fillMaxWidth().height(48.dp),
                        ) { Text("Buka Episode") }
                    }
                    else -> {
                        Button(
                            onClick = {
                                val t = token ?: return@Button
                                busy = true
                                scope.launch {
                                    try {
                                        repo.adUnlockComplete(episodeId, t)
                                        onUnlocked(episodeId)
                                    } catch (e: Exception) {
                                        busy = false
                                        error = "Gagal membuka episode. Periksa koneksimu lalu coba lagi."
                                    }
                                }
                            },
                            enabled = !busy,
                            colors = ButtonDefaults.buttonColors(containerColor = NetflixRed),
                            modifier = Modifier.fillMaxWidth().height(48.dp),
                        ) {
                            Text(
                                if (busy) "Membuka…" else "Buka Episode",
                                fontWeight = FontWeight.Bold,
                                color = Color.White,
                            )
                        }
                    }
                }
            }
        } else {
            Column(
                modifier = Modifier.align(Alignment.Center).padding(24.dp),
                horizontalAlignment = Alignment.CenterHorizontally,
            ) {
                Text(error!!, color = NetflixRed)
                Spacer(Modifier.height(12.dp))
                OutlinedButton(onClick = { attempt++ }) { Text("Coba Lagi") }
                TextButton(onClick = onBack) { Text("← Kembali") }
            }
        }
    }
}
