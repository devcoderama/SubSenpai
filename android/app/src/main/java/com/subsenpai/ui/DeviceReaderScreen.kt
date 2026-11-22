package com.subsenpai.ui

import android.annotation.SuppressLint
import android.webkit.CookieManager
import android.webkit.WebView
import android.webkit.WebViewClient
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.material3.Button
import androidx.compose.material3.Card
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
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.unit.dp
import androidx.compose.ui.viewinterop.AndroidView
import coil.compose.AsyncImage
import coil.request.ImageRequest
import androidx.compose.ui.platform.LocalContext
import com.subsenpai.data.CatalogRepository
import com.subsenpai.data.DeviceSourceParser
import com.subsenpai.data.ExtChapter
import com.subsenpai.data.ReaderNav
import com.subsenpai.data.TitleDetail
import kotlinx.coroutines.launch
import org.json.JSONObject

/**
 * F39 — baca dari sumber ber-Cloudflare langsung di perangkat: WebView
 * melewati verifikasi sumber sekali, lalu daftar chapter & halaman
 * diuraikan dari HTML halaman yang sudah terbuka (pola Mihon). Halaman
 * mengalir langsung dari sumber ke perangkat; progres dicatat sebagai
 * riwayat eksternal di server.
 */
@SuppressLint("SetJavaScriptEnabled")
@Composable
fun DeviceReaderScreen(
    repo: CatalogRepository,
    titleId: String,
    onBack: () -> Unit,
    onNeedLogin: () -> Unit,
) {
    val context = LocalContext.current
    val scope = rememberCoroutineScope()
    var detail by remember { mutableStateOf<TitleDetail?>(ReaderNav.detail?.takeIf { it.card.id == titleId }) }
    var webView by remember { mutableStateOf<WebView?>(null) }
    var pendingExtract by remember { mutableStateOf<((String) -> Unit)?>(null) }
    var chapters by remember { mutableStateOf<List<ExtChapter>>(emptyList()) }
    var status by remember { mutableStateOf("Memuat sumber… lewati verifikasi bila muncul, lalu ambil daftar chapter.") }
    var reading by remember { mutableStateOf<ExtChapter?>(null) }
    var pages by remember { mutableStateOf<List<String>>(emptyList()) }
    var busy by remember { mutableStateOf(false) }
    val opened = remember { mutableSetOf<String>() }
    val listState = rememberLazyListState()

    LaunchedEffect(titleId) {
        if (detail == null) {
            try {
                detail = repo.titleDetail(titleId)
            } catch (e: Exception) {
                status = "Gagal memuat judul: ${e.message}"
            }
        }
    }

    val seriesUrl = remember(detail) {
        val d = detail
        when {
            d == null -> ""
            d.sourceUrlTemplate != null ->
                d.sourceUrlTemplate.replace("{slug}", d.card.slug)
            d.sourceDomain != null -> "https://${d.sourceDomain}/"
            else -> ""
        }
    }

    fun unescape(raw: String?): String {
        if (raw == null || raw == "null") return ""
        return try {
            JSONObject("{\"x\":$raw}").getString("x")
        } catch (e: Exception) {
            raw.trim('"')
        }
    }

    fun extractCurrent(cb: (String) -> Unit) {
        webView?.evaluateJavascript("document.documentElement.outerHTML") { raw -> cb(unescape(raw)) }
    }

    fun fetchAndExtract(url: String, cb: (String) -> Unit) {
        pendingExtract = cb
        webView?.loadUrl(url)
    }

    fun openChapter(ch: ExtChapter) {
        if (!repo.isLoggedIn && opened.isNotEmpty() && ch.url !in opened) {
            status = "Tamu hanya boleh membuka 1 chapter per judul — masuk untuk melanjutkan."
            return
        }
        busy = true
        status = "Memuat ${ch.label}…"
        fetchAndExtract(ch.url) { html ->
            val found = DeviceSourceParser.parsePages(html)
            busy = false
            if (found.isEmpty()) {
                status = "Halaman belum terbaca — tunggu halaman termuat penuh, gulir sedikit, lalu ketuk chapter lagi."
            } else {
                opened.add(ch.url)
                pages = found
                reading = ch
                status = ""
                scope.launch { listState.scrollToItem(0) }
            }
        }
    }

    // Lapor progres baca eksternal saat halaman berubah.
    val currentPage = (listState.firstVisibleItemIndex).coerceAtLeast(0)
    LaunchedEffect(currentPage, reading) {
        val ch = reading ?: return@LaunchedEffect
        if (pages.isNotEmpty()) {
            repo.reportReadExternal(titleId, ch.label, ch.no, currentPage + 1)
        }
    }

    Column(modifier = Modifier.fillMaxSize()) {
        Row(modifier = Modifier.fillMaxWidth().padding(horizontal = 8.dp)) {
            TextButton(onClick = {
                if (reading != null) {
                    reading = null
                    pages = emptyList()
                } else {
                    onBack()
                }
            }) { Text(if (reading != null) "← Daftar chapter" else "← Kembali") }
            Text(
                detail?.card?.title ?: "",
                style = MaterialTheme.typography.titleMedium,
                modifier = Modifier.padding(top = 12.dp),
            )
        }
        if (status.isNotBlank()) {
            Text(status, style = MaterialTheme.typography.bodySmall, modifier = Modifier.padding(16.dp))
        }

        if (reading == null) {
            if (seriesUrl.isNotBlank()) {
                AndroidView(
                    factory = { ctx ->
                        WebView(ctx).apply {
                            settings.javaScriptEnabled = true
                            settings.domStorageEnabled = true
                            settings.loadWithOverviewMode = true
                            settings.useWideViewPort = true
                            webViewClient = object : WebViewClient() {
                                override fun onPageFinished(view: WebView?, url: String?) {
                                    val cb = pendingExtract ?: return
                                    pendingExtract = null
                                    view?.postDelayed({
                                        view.evaluateJavascript(
                                            "document.documentElement.outerHTML",
                                        ) { raw -> cb(unescape(raw)) }
                                    }, 900)
                                }
                            }
                            webView = this
                            loadUrl(seriesUrl)
                        }
                    },
                    modifier = Modifier.fillMaxWidth().height(300.dp),
                )
            }
            Row(
                modifier = Modifier.fillMaxWidth().padding(8.dp),
                horizontalArrangement = Arrangement.spacedBy(6.dp),
            ) {
                Button(
                    enabled = !busy,
                    onClick = {
                        busy = true
                        status = "Mengurai daftar chapter…"
                        extractCurrent { html ->
                            chapters = DeviceSourceParser.parseChapters(html)
                            busy = false
                            status = if (chapters.isEmpty()) {
                                "Chapter belum terbaca — pastikan halaman seri sudah termuat (bukan halaman verifikasi), lalu coba lagi."
                            } else {
                                "${chapters.size} chapter ditemukan."
                            }
                        }
                    },
                ) { Text("Ambil Daftar Chapter") }
                if (!repo.isLoggedIn) {
                    OutlinedButton(onClick = onNeedLogin) { Text("Masuk") }
                }
            }
            LazyColumn(modifier = Modifier.fillMaxSize()) {
                items(chapters) { ch ->
                    Card(
                        modifier = Modifier.fillMaxWidth().padding(horizontal = 12.dp, vertical = 3.dp)
                            .clickable { openChapter(ch) },
                    ) {
                        Text(ch.label, modifier = Modifier.padding(12.dp))
                    }
                }
            }
        } else {
            val ch = reading!!
            val idx = chapters.indexOfFirst { it.url == ch.url }
            val prev = chapters.getOrNull(idx - 1)
            val next = chapters.getOrNull(idx + 1)
            Row(
                modifier = Modifier.fillMaxWidth().padding(horizontal = 8.dp),
                horizontalArrangement = Arrangement.SpaceBetween,
            ) {
                Text(
                    "${ch.label} · Hal. ${(currentPage + 1).coerceAtMost(pages.size)}/${pages.size}",
                    style = MaterialTheme.typography.bodySmall,
                    modifier = Modifier.padding(top = 10.dp),
                )
                Row {
                    if (prev != null) TextButton(onClick = { openChapter(prev) }) { Text("← Prev") }
                    if (next != null) TextButton(onClick = { openChapter(next) }) { Text("Next →") }
                }
            }
            val cookie = remember(ch.url) { CookieManager.getInstance().getCookie(ch.url) ?: "" }
            val ua = remember { webView?.settings?.userAgentString ?: "" }
            LazyColumn(state = listState, modifier = Modifier.fillMaxSize()) {
                itemsIndexed(pages) { i, url ->
                    AsyncImage(
                        model = ImageRequest.Builder(context)
                            .data(url)
                            .addHeader("Cookie", cookie)
                            .addHeader("User-Agent", ua)
                            .addHeader("Referer", ch.url)
                            .build(),
                        contentDescription = "Halaman ${i + 1}",
                        contentScale = ContentScale.FillWidth,
                        modifier = Modifier.fillMaxWidth(),
                    )
                }
                item {
                    Row(
                        modifier = Modifier.fillMaxWidth().padding(12.dp),
                        horizontalArrangement = Arrangement.SpaceBetween,
                    ) {
                        if (prev != null) {
                            OutlinedButton(onClick = { openChapter(prev) }) { Text("← Chapter sebelumnya") }
                        }
                        if (next != null) {
                            Button(onClick = { openChapter(next) }) { Text("Chapter berikutnya →") }
                        }
                    }
                }
            }
        }
    }
}
