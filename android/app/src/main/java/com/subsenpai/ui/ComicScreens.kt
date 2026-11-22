package com.subsenpai.ui

import android.annotation.SuppressLint
import android.webkit.CookieManager
import android.webkit.WebView
import android.webkit.WebViewClient
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
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.FilterChip
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
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.viewinterop.AndroidView
import coil.compose.AsyncImage
import coil.request.ImageRequest
import androidx.compose.foundation.Image
import androidx.compose.runtime.mutableStateMapOf
import androidx.compose.ui.graphics.asImageBitmap
import com.subsenpai.data.CatalogRepository
import com.subsenpai.data.ComicChapter
import com.subsenpai.data.ComicNav
import com.subsenpai.data.ComicSeries
import com.subsenpai.data.DeviceSourceParser
import com.subsenpai.data.GtxTranslator
import com.subsenpai.data.ManhuarmPage
import com.subsenpai.data.ManhuarmReader
import com.subsenpai.data.ManhuarmSession
import com.subsenpai.data.ManhuarmtlApi
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.sync.Semaphore
import kotlinx.coroutines.sync.withPermit
import org.json.JSONObject
import java.util.Locale

/**
 * Komik langsung dari perangkat (pola ekstensi sumber ala Mihon):
 * daftar seri, pencarian, dan detail diambil dari API publik
 * manhuarmtl oleh aplikasi sendiri — tanpa server SubSenpai. Membaca
 * chapter memakai WebView sumber (F39) karena endpoint chapter API
 * situs mensyaratkan otorisasi aplikasi mereka.
 */

@Composable
fun ComicDetailScreen(
    onBack: () -> Unit,
    onRead: () -> Unit,
) {
    val base = ComicNav.series
    var series by remember { mutableStateOf(base) }
    var chapters by remember { mutableStateOf<List<ComicChapter>>(emptyList()) }
    var status by remember { mutableStateOf("Memuat detail & daftar chapter…") }

    LaunchedEffect(base?.url) {
        val b = base ?: return@LaunchedEffect
        try {
            val (s, ch) = ManhuarmtlApi.detail(b.url)
            series = s
            chapters = ch
            ComicNav.series = s
            ComicNav.chapters = ch
            status = if (ch.isEmpty()) "Daftar chapter kosong dari sumber." else ""
        } catch (e: Exception) {
            status = "Gagal memuat detail: ${e.message}"
        }
    }

    val s = series
    Column(modifier = Modifier.fillMaxSize()) {
        TextButton(onClick = onBack) { Text("← Kembali", color = Color.White) }
        if (s == null) {
            Text("Seri tidak ditemukan.", modifier = Modifier.padding(16.dp))
            return@Column
        }
        LazyColumn(modifier = Modifier.fillMaxSize()) {
            item {
                Column(modifier = Modifier.padding(horizontal = 16.dp)) {
                    PosterImage(s.poster, s.title, Modifier.fillMaxWidth().height(300.dp))
                    Spacer(Modifier.height(10.dp))
                    Text(s.title, style = MaterialTheme.typography.headlineSmall, fontWeight = FontWeight.Bold)
                    val meta = listOfNotNull(
                        s.type.replaceFirstChar { it.uppercase() },
                        s.year?.toString(),
                        when (s.status) {
                            "end", "completed" -> "Tamat"
                            "ongoing" -> "Berlanjut"
                            else -> s.status
                        },
                        s.genres.takeIf { it.isNotEmpty() }?.joinToString(", "),
                    ).joinToString(" · ")
                    if (meta.isNotBlank()) {
                        Text(meta, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    }
                    s.author?.let {
                        Text("Penulis: $it", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    }
                    s.artist?.let {
                        Text("Ilustrator: $it", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    }
                    s.rating?.let {
                        Text("Rating sumber: $it", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    }
                    s.synopsis?.let {
                        Spacer(Modifier.height(8.dp))
                        Text(it, style = MaterialTheme.typography.bodyMedium)
                    }
                    if (status.isNotBlank()) {
                        Spacer(Modifier.height(8.dp))
                        Text(status, style = MaterialTheme.typography.bodySmall)
                    }
                    Spacer(Modifier.height(10.dp))
                    Text("Chapter (${chapters.size})", style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.Bold)
                    Spacer(Modifier.height(6.dp))
                }
            }
            items(chapters.asReversed()) { ch ->
                Card(
                    modifier = Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 3.dp)
                        .clickable {
                            ComicNav.startChapter = ch
                            onRead()
                        },
                ) {
                    Text(ch.label, modifier = Modifier.padding(12.dp))
                }
            }
        }
    }
}

@SuppressLint("SetJavaScriptEnabled")
@Composable
fun ComicReaderScreen(
    repo: CatalogRepository,
    onBack: () -> Unit,
    onNeedLogin: () -> Unit,
) {
    val series = ComicNav.series
    val chapters = ComicNav.chapters
    val context = LocalContext.current
    val scope = rememberCoroutineScope()
    var webView by remember { mutableStateOf<WebView?>(null) }
    var pendingExtract by remember { mutableStateOf<((String) -> Unit)?>(null) }
    var reading by remember { mutableStateOf<ComicChapter?>(null) }
    var pages by remember { mutableStateOf<List<String>>(emptyList()) }
    var busy by remember { mutableStateOf(false) }
    var webExpanded by remember { mutableStateOf(false) }
    var status by remember { mutableStateOf("Menyiapkan bacaan…") }
    val opened = remember { mutableSetOf<String>() }
    val listState = rememberLazyListState()

    // --- Sesi baca v9 (pola ekstensi Manhuarm di Mihon) ---
    var session by remember { mutableStateOf<ManhuarmSession?>(null) }
    val rawPages = remember { mutableStateMapOf<Int, ByteArray>() }
    val bitmaps = remember { mutableStateMapOf<Int, android.graphics.Bitmap>() }
    var translated by remember { mutableStateOf(false) }
    var translating by remember { mutableStateOf(false) }
    var translations by remember { mutableStateOf<Map<String, String>>(emptyMap()) }
    val deviceLang = remember {
        when (val l = Locale.getDefault().language) {
            "in" -> "id"
            else -> l.ifBlank { "id" }
        }
    }
    val translator = remember { GtxTranslator() }

    fun composeFor(sess: ManhuarmSession, page: ManhuarmPage, bytes: ByteArray, useTr: Boolean): android.graphics.Bitmap? {
        val boxes = sess.dialogsOf(page).map { d ->
            d to (if (useTr) translations[d.text] ?: d.text else d.text)
        }
        return ManhuarmReader.composePage(bytes, boxes)
    }

    fun unescape(raw: String?): String {
        if (raw == null || raw == "null") return ""
        return try {
            JSONObject("{\"x\":$raw}").getString("x")
        } catch (e: Exception) {
            raw.trim('"')
        }
    }

    fun isCfChallenge(html: String): Boolean =
        html.contains("challenges.cloudflare") || html.contains("cf-chl") ||
            html.contains("Melakukan verifikasi") || html.contains("Just a moment") ||
            html.contains("Verifying you are human") || html.contains("Attention Required")

    /** Jalur cadangan lama: ekstrak URL gambar dari HTML di WebView. */
    fun openChapterLegacy(ch: ComicChapter) {
        var cfWaits = 0
        lateinit var cb: (String) -> Unit
        cb = { html ->
            if (isCfChallenge(html) && cfWaits < 10) {
                // Halaman perantara Cloudflare — ia memuat ulang sendiri
                // sesudah pemeriksaan; pasang lagi pengekstraknya dan tunggu.
                cfWaits++
                status = "Menunggu verifikasi keamanan selesai…"
                pendingExtract = cb
            } else {
                val found = DeviceSourceParser.parsePages(html)
                busy = false
                if (found.isEmpty()) {
                    status = if (isCfChallenge(html)) {
                        "Verifikasi belum selesai — ketuk Verifikasi Manual di bawah, " +
                            "selesaikan pemeriksaannya, lalu ketuk Coba Lagi."
                    } else {
                        "Halaman terbuka tapi gambar belum terbaca — ketuk Coba Lagi."
                    }
                } else {
                    opened.add(ch.url)
                    pages = found
                    reading = ch
                    status = ""
                }
            }
        }
        pendingExtract = cb
        webView?.loadUrl(ch.url)
    }

    /** Tunggu kuki cf_clearance muncul dari WebView (verifikasi di latar). */
    suspend fun waitForClearance(url: String, timeoutMs: Long) {
        val step = 1200L
        var waited = 0L
        while (waited < timeoutMs) {
            val cookie = CookieManager.getInstance().getCookie(url) ?: ""
            if (cookie.contains("cf_clearance")) return
            delay(step)
            waited += step
        }
    }

    fun openChapter(ch: ComicChapter) {
        if (!repo.isLoggedIn && opened.isNotEmpty() && ch.url !in opened) {
            status = "Tamu hanya boleh membuka 1 chapter per judul — masuk untuk melanjutkan."
            return
        }
        busy = true
        status = "Menyiapkan ${ch.label}…"
        session = null
        rawPages.clear()
        bitmaps.clear()
        translations = emptyMap()
        translated = false
        scope.launch {
            val ua = webView?.settings?.userAgentString ?: ""
            var sess: ManhuarmSession? = null
            var lastErr: Exception? = null
            for (round in 0..2) {
                try {
                    val html = ManhuarmReader.fetchChapterHtml(ch.url, ua)
                    if (isCfChallenge(html)) throw IllegalStateException("halaman masih tertahan verifikasi")
                    sess = ManhuarmReader.openChapter(html, ch.url, ua)
                    break
                } catch (e: Exception) {
                    lastErr = e
                    sess = null
                    if (round < 2) {
                        // Verifikasi Cloudflare berjalan di WebView latar —
                        // tunggu kukinya, lalu ambil HTML segar & coba lagi.
                        status = "Menunggu verifikasi keamanan selesai…"
                        webView?.reload()
                        waitForClearance(ch.url, 15_000)
                    }
                }
            }
            val openedSess = sess
            if (openedSess == null) {
                status = "Sesi baca gagal (${lastErr?.message ?: "tidak diketahui"}) — mencoba cara lama…"
                openChapterLegacy(ch)
                return@launch
            }
            session = openedSess
            opened.add(ch.url)
            reading = ch
            busy = false
            status = ""
            val sem = Semaphore(3)
            openedSess.pages.forEach { page ->
                launch {
                    sem.withPermit {
                        try {
                            val bytes = ManhuarmReader.fetchImage(openedSess, page, ua)
                            rawPages[page.index] = bytes
                            composeFor(openedSess, page, bytes, false)?.let { bitmaps[page.index] = it }
                        } catch (e: Exception) {
                            // Halaman gagal: biarkan penampung kosong.
                        }
                    }
                }
            }
        }
    }

    fun toggleTranslate() {
        val sess = session ?: return
        if (translated) {
            translated = false
            sess.pages.forEach { page ->
                rawPages[page.index]?.let { bytes ->
                    composeFor(sess, page, bytes, false)?.let { bitmaps[page.index] = it }
                }
            }
            return
        }
        if (translations.isNotEmpty()) {
            translated = true
            sess.pages.forEach { page ->
                rawPages[page.index]?.let { bytes ->
                    composeFor(sess, page, bytes, true)?.let { bitmaps[page.index] = it }
                }
            }
            return
        }
        translating = true
        scope.launch {
            val texts = sess.dialogues.values.flatten().map { it.text }
                .filter { it.isNotBlank() }.distinct()
            val hasil = translator.translateTexts(texts, deviceLang)
            translating = false
            if (hasil == null || hasil.size != texts.size) {
                status = "Terjemahan gagal — menampilkan teks asli."
                return@launch
            }
            translations = texts.zip(hasil).toMap()
            translated = true
            sess.pages.forEach { page ->
                rawPages[page.index]?.let { bytes ->
                    composeFor(sess, page, bytes, true)?.let { bitmaps[page.index] = it }
                }
            }
        }
    }

    if (series == null) {
        Column(modifier = Modifier.fillMaxSize().padding(16.dp)) {
            TextButton(onClick = onBack) { Text("← Kembali", color = Color.White) }
            Text("Seri tidak ditemukan.")
        }
        return
    }

    Column(modifier = Modifier.fillMaxSize()) {
        Row(modifier = Modifier.fillMaxWidth().padding(horizontal = 8.dp)) {
            TextButton(onClick = onBack) { Text("← Detail", color = Color.White) }
            Text(
                series.title,
                style = MaterialTheme.typography.titleMedium,
                modifier = Modifier.padding(top = 12.dp),
            )
        }
        if (status.isNotBlank()) {
            Text(status, style = MaterialTheme.typography.bodySmall, modifier = Modifier.padding(horizontal = 16.dp))
        }
        // WebView sumber berjalan DI LATAR (1dp, tak terlihat): hanya untuk
        // melewati verifikasi Cloudflare + menyimpan kukinya. Panel hanya
        // membesar bila user memilih verifikasi manual.
        AndroidView(
            factory = { ctx ->
                WebView(ctx).apply {
                    settings.javaScriptEnabled = true
                    settings.domStorageEnabled = true
                    settings.loadWithOverviewMode = true
                    settings.useWideViewPort = true
                    CookieManager.getInstance().setAcceptThirdPartyCookies(this, true)
                    webViewClient = object : WebViewClient() {
                        override fun onPageFinished(view: WebView?, url: String?) {
                            val cb = pendingExtract ?: run {
                                // Halaman seri selesai dimuat pertama kali —
                                // langsung buka chapter tujuan.
                                val start = ComicNav.startChapter
                                if (start != null && reading == null && !busy) {
                                    openChapter(start)
                                }
                                return
                            }
                            pendingExtract = null
                            view?.postDelayed({
                                view.evaluateJavascript(
                                    "document.documentElement.outerHTML",
                                ) { raw -> cb(unescape(raw)) }
                            }, 900)
                        }
                    }
                    webView = this
                    loadUrl(series.url)
                }
            },
            modifier = Modifier.fillMaxWidth().height(if (webExpanded) 420.dp else 1.dp),
        )
        val ch = reading
        val sess = session
        if (ch == null) {
            Row(
                modifier = Modifier.fillMaxWidth().padding(8.dp),
                horizontalArrangement = Arrangement.spacedBy(6.dp),
            ) {
                Button(
                    enabled = !busy,
                    onClick = { ComicNav.startChapter?.let { openChapter(it) } },
                ) { Text("Coba Lagi") }
                TextButton(onClick = { webExpanded = !webExpanded }) {
                    Text(if (webExpanded) "Tutup Verifikasi" else "Verifikasi Manual")
                }
                if (!repo.isLoggedIn) {
                    OutlinedButton(onClick = onNeedLogin) { Text("Masuk") }
                }
            }
            if (busy) {
                Box(
                    modifier = Modifier.fillMaxWidth().padding(top = 24.dp),
                    contentAlignment = androidx.compose.ui.Alignment.Center,
                ) { BrandLoading() }
            }
        } else if (sess != null) {
            val idx = chapters.indexOfFirst { it.url == ch.url }
            val prev = chapters.getOrNull(idx - 1)
            val next = chapters.getOrNull(idx + 1)
            val currentPage = (listState.firstVisibleItemIndex).coerceAtLeast(0)
            Row(
                modifier = Modifier.fillMaxWidth().padding(horizontal = 8.dp),
                horizontalArrangement = Arrangement.SpaceBetween,
            ) {
                Text(
                    "${ch.label} · Hal. ${(currentPage + 1).coerceAtMost(sess.pages.size)}/${sess.pages.size}",
                    style = MaterialTheme.typography.bodySmall,
                    modifier = Modifier.padding(top = 10.dp),
                )
                Row {
                    TextButton(enabled = !translating, onClick = { toggleTranslate() }) {
                        Text(
                            when {
                                translating -> "Menerjemahkan…"
                                translated -> "Teks Asli (EN)"
                                else -> "Terjemahkan (${deviceLang.uppercase()})"
                            },
                        )
                    }
                    if (prev != null) TextButton(onClick = { openChapter(prev) }) { Text("← Prev") }
                    if (next != null) TextButton(onClick = { openChapter(next) }) { Text("Next →") }
                }
            }
            LazyColumn(state = listState, modifier = Modifier.fillMaxSize()) {
                items(sess.pages, key = { it.index }) { page ->
                    val bmp = bitmaps[page.index]
                    if (bmp != null) {
                        Image(
                            bitmap = bmp.asImageBitmap(),
                            contentDescription = "Halaman ${page.index + 1}",
                            contentScale = ContentScale.FillWidth,
                            modifier = Modifier.fillMaxWidth(),
                        )
                    } else {
                        Box(
                            modifier = Modifier.fillMaxWidth().height(240.dp),
                            contentAlignment = androidx.compose.ui.Alignment.Center,
                        ) {
                            BrandLoading(compact = true)
                        }
                    }
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
        } else {
            val idx = chapters.indexOfFirst { it.url == ch.url }
            val prev = chapters.getOrNull(idx - 1)
            val next = chapters.getOrNull(idx + 1)
            val currentPage = (listState.firstVisibleItemIndex).coerceAtLeast(0)
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
