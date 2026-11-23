package com.subsenpai.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import com.subsenpai.R
import com.subsenpai.data.ApiException
import java.io.IOException
import java.net.ConnectException
import java.net.SocketTimeoutException
import java.net.UnknownHostException
import java.security.cert.CertPathValidatorException
import java.security.cert.CertificateException
import javax.net.ssl.SSLException

/**
 * Halaman peringatan khusus untuk kegagalan muat yang fatal
 * (permintaan Rams 2026-11-23): pesan teknis mentah TIDAK boleh
 * tampil di halaman utama — pengguna mendapat halaman peringatan
 * yang rapi ala peringatan keamanan browser, dengan penjelasan
 * bahasa awam, saran tindakan, tombol Coba Lagi, dan detail teknis
 * yang tersembunyi di balik pengalih.
 */
enum class AppErrorKind { SECURITY, NETWORK, SERVER, UNKNOWN }

/** Klasifikasikan pengecualian jaringan/API menjadi jenis halaman. */
fun classifyError(e: Throwable): AppErrorKind {
    if (e is ApiException) {
        return if (e.code >= 500) AppErrorKind.SERVER else AppErrorKind.UNKNOWN
    }
    var cur: Throwable? = e
    while (cur != null) {
        if (cur is SSLException || cur is CertificateException || cur is CertPathValidatorException) {
            return AppErrorKind.SECURITY
        }
        cur = cur.cause
    }
    val msg = e.message.orEmpty()
    if (msg.contains("Cert", ignoreCase = true) ||
        msg.contains("Trust anchor", ignoreCase = true) ||
        msg.contains("SSL", ignoreCase = true)
    ) {
        return AppErrorKind.SECURITY
    }
    if (e is UnknownHostException || e is ConnectException ||
        e is SocketTimeoutException || e is IOException
    ) {
        return AppErrorKind.NETWORK
    }
    return AppErrorKind.UNKNOWN
}

/** Pesan singkat ramah untuk kegagalan non-fatal (di dalam halaman). */
fun friendlyInlineError(e: Throwable): String = when (classifyError(e)) {
    AppErrorKind.SECURITY ->
        "Koneksi tidak aman terdeteksi. Matikan VPN atau aplikasi perekam lalu lintas, lalu coba lagi."
    AppErrorKind.NETWORK ->
        "Tidak dapat terhubung. Periksa koneksi internetmu, lalu coba lagi."
    AppErrorKind.SERVER ->
        "Server sedang bermasalah. Silakan coba lagi beberapa saat lagi."
    AppErrorKind.UNKNOWN ->
        "Gagal memuat. Silakan coba lagi."
}

private data class ErrorCopy(
    val icon: Int,
    val tint: Color,
    val title: String,
    val description: String,
    val suggestions: List<String>,
)

private fun errorCopy(kind: AppErrorKind): ErrorCopy = when (kind) {
    AppErrorKind.SECURITY -> ErrorCopy(
        icon = R.drawable.ic_shield,
        tint = Color(0xFFFFB300),
        title = "Koneksi Tidak Aman",
        description = "Sertifikat keamanan server tidak dapat diverifikasi. " +
            "Lalu lintas jaringanmu kemungkinan sedang dialihkan atau disadap — " +
            "misalnya oleh VPN atau aplikasi perekam lalu lintas yang aktif di perangkat ini. " +
            "Demi keamanan akunmu, aplikasi menghentikan koneksi.",
        suggestions = listOf(
            "Matikan VPN atau aplikasi perekam lalu lintas (mis. HttpCanary), lalu coba lagi",
            "Gunakan jaringan yang kamu percaya, seperti data seluler atau Wi-Fi rumah",
            "Jangan memasukkan akun di jaringan yang tidak kamu kenali",
        ),
    )
    AppErrorKind.NETWORK -> ErrorCopy(
        icon = R.drawable.ic_cloud_off,
        tint = Color(0xFFB3B3B3),
        title = "Tidak Dapat Terhubung",
        description = "Perangkatmu tidak dapat menjangkau server SubSenpai saat ini. " +
            "Ini biasanya soal koneksi internet, bukan akun atau aplikasimu.",
        suggestions = listOf(
            "Periksa Wi-Fi atau data seluler dan pastikan mode pesawat mati",
            "Pindah ke jaringan lain bila sinyal lemah",
            "Ketuk Coba Lagi setelah koneksi stabil",
        ),
    )
    AppErrorKind.SERVER -> ErrorCopy(
        icon = R.drawable.ic_warning,
        tint = Color(0xFFE87C7C),
        title = "Server Sedang Bermasalah",
        description = "Server SubSenpai mengembalikan kesalahan saat memproses permintaan. " +
            "Tim kami biasanya memperbaikinya dalam beberapa menit.",
        suggestions = listOf(
            "Tunggu sebentar, lalu ketuk Coba Lagi",
            "Bila berlanjut, coba lagi nanti — datamu aman",
        ),
    )
    AppErrorKind.UNKNOWN -> ErrorCopy(
        icon = R.drawable.ic_warning,
        tint = Color(0xFFE87C7C),
        title = "Terjadi Kesalahan",
        description = "Ada yang tidak beres saat memuat halaman ini.",
        suggestions = listOf(
            "Ketuk Coba Lagi",
            "Tutup dan buka kembali aplikasi bila masalah berlanjut",
        ),
    )
}

/** Halaman peringatan layar penuh dengan tombol coba lagi. */
@Composable
fun ErrorPage(kind: AppErrorKind, technical: String?, onRetry: () -> Unit) {
    val copy = errorCopy(kind)
    var showTech by remember { mutableStateOf(false) }
    Column(
        modifier = Modifier.fillMaxSize().padding(horizontal = 28.dp),
        verticalArrangement = Arrangement.Center,
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Box(
            modifier = Modifier.size(96.dp).clip(CircleShape).background(Color(0xFF1F1F1F)),
            contentAlignment = Alignment.Center,
        ) {
            Icon(
                painter = painterResource(copy.icon),
                contentDescription = null,
                tint = copy.tint,
                modifier = Modifier.size(48.dp),
            )
        }
        Text(
            copy.title,
            style = MaterialTheme.typography.headlineSmall,
            fontWeight = FontWeight.Bold,
            textAlign = TextAlign.Center,
            modifier = Modifier.padding(top = 24.dp),
        )
        Text(
            copy.description,
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
            textAlign = TextAlign.Center,
            modifier = Modifier.padding(top = 12.dp),
        )
        Column(
            modifier = Modifier.fillMaxWidth().padding(top = 24.dp),
            verticalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            Text(
                "Yang bisa kamu lakukan:",
                style = MaterialTheme.typography.bodyMedium,
                fontWeight = FontWeight.Bold,
            )
            copy.suggestions.forEach { s ->
                Text(
                    "•  $s",
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
        }
        Button(
            onClick = onRetry,
            colors = ButtonDefaults.buttonColors(containerColor = NetflixRed),
            modifier = Modifier.fillMaxWidth().padding(top = 28.dp),
        ) {
            Text("Coba Lagi")
        }
        if (!technical.isNullOrBlank()) {
            Text(
                if (showTech) "Sembunyikan detail teknis" else "Detail teknis",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.clickable { showTech = !showTech }.padding(top = 16.dp),
            )
            if (showTech) {
                Text(
                    technical,
                    style = MaterialTheme.typography.bodySmall,
                    fontFamily = FontFamily.Monospace,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.padding(top = 8.dp),
                )
            }
        }
    }
}
