package com.subsenpai.ui

import android.annotation.SuppressLint
import android.webkit.CookieManager
import android.webkit.WebView
import android.webkit.WebViewClient
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
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
import com.subsenpai.data.CatalogRepository
import com.subsenpai.data.ComicChapter
import com.subsenpai.data.ComicNav
import com.subsenpai.data.ComicSeries
import com.subsenpai.data.DeviceSourceParser
import com.subsenpai.data.ManhuarmtlApi
import kotlinx.coroutines.launch
import org.json.JSONObject

/**
 * Komik langsung dari perangkat (pola ekstensi sumber ala Mihon):
 * daftar seri, pencarian, dan detail diambil dari API publik
 * manhuarmtl oleh aplikasi sendiri — tanpa server SubSenpai. Membaca
 * chapter memakai WebView sumber (F39) karena endpoint chapter API
 * situs mensyaratkan otorisasi aplikasi mereka.
 */

private val comicTypes = listOf(
    "" to "Semua", "manga" to "Manga", "manhua" to "Manhua", "manhwa" to "Manhwa",
)

@Composable
fun ComicBrowseScreen(
    onBack: () -> Unit,
    onOpenSeries: (ComicSeries) -> Unit,
) {
    val scope = rememberCoroutineScope()
    var query by remember { mutableStateOf("") }
    var typeFilter by remember { mutableStateOf("") }
    var raw by remember { mutableStateOf<List<ComicSeries>>(emptyList()) }
    var page by remember { mutableStateOf(0) }
    var totalPages by remember { mutableStateOf(1) }
    var searching by remember { mutableStateOf<String?>(null) }
    var loading by remember { mutableStateOf(false) }
    var error by remember { mutableStateOf<String?>(null) }

    val visible = raw.filter { typeFilter.isEmpty() || it.type == typeFilter }

    suspend fun fetchMore() {
        if (loading) return
        loading = true
        error = null
        try {
            var fetched = 0
            var grew = false
            val before = raw.count { typeFilter.isEmpty() || it.type == typeFilter }
            while (fetched < 6 && page < totalPages) {
                val next = page + 1
                val (items, total) = if (searching.isNullOrBlank()) {
                    ManhuarmtlApi.latest(next)
                } else {
                    ManhuarmtlApi.search(searching!!, next)
                }
                totalPages = total
                page = next
                fetched++
                val known = raw.map { it.slug }.toSet()
                raw = raw + items.filter { it.slug !in known }
                val now = raw.count { typeFilter.isEmpty() || it.type == typeFilter }
                if (now > before) {
                    grew = true
                    break
                }
            }
            if (!grew && page >= totalPages && visible.isEmpty()) {
                error = "Tidak ada seri ditemukan."
            }
        } catch (e: Exception) {
            error = "Gagal memuat dari sumber: ${e.message}"
        } finally {
            loading = false
        }
    }

    fun resetAndLoad() {
        raw = emptyList()
        page = 0
        totalPages = 1
        scope.launch { fetchMore() }
    }

    LaunchedEffect(Unit) { fetchMore() }

    Column(modifier = Modifier.fillMaxSize().padding(horizontal = 12.dp)) {
        Row(modifier = Modifier.fillMaxWidth()) {
            TextButton(onClick = onBack) { Text("← Kembali", color = Color.White) }
            Text(
                "Komik (Sumber Langsung)",
                style = MaterialTheme.typography.titleMedium,
                fontWeight = FontWeight.Bold,
                modifier = Modifier.padding(top = 10.dp),
            )
        }
        OutlinedTextField(
            value = query,
            onValueChange = { query = it },
            placeholder = { Text("Cari manga, manhua, manhwa…") },
            singleLine = true,
            modifier = Modifier.fillMaxWidth(),
        )
        Row(
            modifier = Modifier.fillMaxWidth().padding(vertical = 6.dp),
            horizontalArrangement = Arrangement.spacedBy(6.dp),
        ) {
            Button(
                enabled = !loading,
                onClick = {
                    searching = query.trim().ifBlank { null }
                    resetAndLoad()
                },
            ) { Text("Cari") }
            if (searching != null) {
                OutlinedButton(
                    enabled = !loading,
                    onClick = {
                        searching = null
                        query = ""
                        resetAndLoad()
                    },
                ) { Text("Terbaru") }
            }
        }
        Row(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
            comicTypes.forEach { (value, label) ->
                FilterChip(
                    selected = typeFilter == value,
                    onClick = {
                        typeFilter = value
                        if (visible.none { value.isEmpty() || it.type == value }) {
                            scope.launch { fetchMore() }
                        }
                    },
                    label = { Text(label) },
                )
            }
        }
        if (error != null) {
            Text(error!!, color = MaterialTheme.colorScheme.error, modifier = Modifier.padding(8.dp))
        }
        LazyVerticalGrid(
            columns = GridCells.Fixed(3),
            horizontalArrangement = Arrangement.spacedBy(8.dp),
            verticalArrangement = Arrangement.spacedBy(10.dp),
            modifier = Modifier.weight(1f).padding(top = 8.dp),
        ) {
            items(visible) { s ->
                Column(modifier = Modifier.clickable { onOpenSeries(s) }) {
                    PosterImage(s.poster, s.title, Modifier.fillMaxWidth().height(150.dp))
                    Text(
                        s.title,
                        style = MaterialTheme.typography.bodySmall,
                        maxLines = 2,
                        modifier = Modifier.padding(top = 4.dp),
                    )
                    Text(
                        s.type.replaceFirstChar { it.uppercase() },
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }
            }
        }
        if (page < totalPages) {
            Button(
                enabled = !loading,
                onClick = { scope.launch { fetchMore() } },
                modifier = Modifier.fillMaxWidth().padding(vertical = 8.dp),
            ) { Text(if (loading) "Memuat…" else "Muat Lebih Banyak") }
        } else if (loading) {
            Text("Memuat…", modifier = Modifier.padding(8.dp))
        }
    }
}

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
    var webView by remember { mutableStateOf<WebView?>(null) }
    var pendingExtract by remember { mutableStateOf<((String) -> Unit)?>(null) }
    var reading by remember { mutableStateOf<ComicChapter?>(null) }
    var pages by remember { mutableStateOf<List<String>>(emptyList()) }
    var busy by remember { mutableStateOf(false) }
    var status by remember {
        mutableStateOf("Memuat sumber… lewati verifikasi bila muncul di bawah — chapter terbuka otomatis.")
    }
    val opened = remember { mutableSetOf<String>() }
    val listState = rememberLazyListState()

    fun unescape(raw: String?): String {
        if (raw == null || raw == "null") return ""
        return try {
            JSONObject("{\"x\":$raw}").getString("x")
        } catch (e: Exception) {
            raw.trim('"')
        }
    }

    fun openChapter(ch: ComicChapter) {
        if (!repo.isLoggedIn && opened.isNotEmpty() && ch.url !in opened) {
            status = "Tamu hanya boleh membuka 1 chapter per judul — masuk untuk melanjutkan."
            return
        }
        busy = true
        status = "Memuat ${ch.label}…"
        pendingExtract = { html ->
            val found = DeviceSourceParser.parsePages(html)
            busy = false
            if (found.isEmpty()) {
                status = "Halaman belum terbaca — tunggu panel sumber di bawah termuat penuh, " +
                    "gulir sedikit, lalu ketuk Coba Lagi."
            } else {
                opened.add(ch.url)
                pages = found
                reading = ch
                status = ""
            }
        }
        webView?.loadUrl(ch.url)
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
        // Panel WebView sumber: untuk verifikasi Cloudflare + kuki baca.
        // Menyusut saat membaca agar halaman komik leluasa.
        AndroidView(
            factory = { ctx ->
                WebView(ctx).apply {
                    settings.javaScriptEnabled = true
                    settings.domStorageEnabled = true
                    settings.loadWithOverviewMode = true
                    settings.useWideViewPort = true
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
            modifier = Modifier.fillMaxWidth().height(if (reading == null) 280.dp else 1.dp),
        )
        val ch = reading
        if (ch == null) {
            Row(
                modifier = Modifier.fillMaxWidth().padding(8.dp),
                horizontalArrangement = Arrangement.spacedBy(6.dp),
            ) {
                Button(
                    enabled = !busy,
                    onClick = { ComicNav.startChapter?.let { openChapter(it) } },
                ) { Text("Coba Lagi") }
                if (!repo.isLoggedIn) {
                    OutlinedButton(onClick = onNeedLogin) { Text("Masuk") }
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
