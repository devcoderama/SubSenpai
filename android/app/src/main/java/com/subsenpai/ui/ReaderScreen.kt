package com.subsenpai.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.material3.Button
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
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import coil.compose.AsyncImage
import com.subsenpai.data.BubbleOcr
import com.subsenpai.data.CatalogRepository
import com.subsenpai.data.ChapterPage
import com.subsenpai.data.OcrBubble
import com.subsenpai.data.ReaderNav
import kotlinx.coroutines.launch
import java.util.Locale

/**
 * Reader komik (Fase 5): halaman vertikal, lapor progres baca, navigasi
 * chapter sebelum/berikut, mode hemat data (varian ringan O16), dan
 * terjemahan balon teks di perangkat (F31: OCR ML Kit + gtx).
 */
@Composable
fun ReaderScreen(
    repo: CatalogRepository,
    chapterId: String,
    onBack: () -> Unit,
    onOpenChapter: (String) -> Unit = {},
) {
    val context = LocalContext.current
    val scope = rememberCoroutineScope()
    var pages by remember { mutableStateOf<List<ChapterPage>>(emptyList()) }
    var error by remember { mutableStateOf<String?>(null) }
    var dataSaver by remember { mutableStateOf(false) }
    var ocrPage by remember { mutableStateOf(-1) }
    var bubbles by remember { mutableStateOf<List<OcrBubble>>(emptyList()) }
    var ocrBusy by remember { mutableStateOf(false) }
    var pageSizes by remember { mutableStateOf<Map<Int, Size>>(emptyMap()) }
    val listState = rememberLazyListState()

    LaunchedEffect(chapterId) {
        pages = emptyList()
        ocrPage = -1
        bubbles = emptyList()
        pageSizes = emptyMap()
        try {
            pages = repo.chapterPages(chapterId)
        } catch (e: Exception) {
            error = e.message
        }
    }

    // Halaman yang sedang terlihat (item 0 = kepala kendali).
    val currentPage = (listState.firstVisibleItemIndex - 1).coerceIn(0, (pages.size - 1).coerceAtLeast(0))
    LaunchedEffect(currentPage, pages.size) {
        if (pages.isNotEmpty()) repo.reportRead(chapterId, currentPage + 1)
    }

    val chapters = ReaderNav.chapters
    val chapterIdx = chapters.indexOfFirst { it.id == chapterId }
    val prev = chapters.getOrNull(chapterIdx - 1)
    val next = chapters.getOrNull(chapterIdx + 1)
    val script = when (ReaderNav.titleType) {
        "manhua" -> "zh"
        "manhwa" -> "ko"
        "manga" -> "ja"
        else -> "latin"
    }
    val targetLang = Locale.getDefault().language.ifBlank { "id" }

    LazyColumn(
        state = listState,
        modifier = Modifier.fillMaxSize(),
        verticalArrangement = Arrangement.spacedBy(2.dp),
    ) {
        item {
            Row(
                modifier = Modifier.fillMaxWidth().padding(horizontal = 8.dp),
                horizontalArrangement = Arrangement.SpaceBetween,
            ) {
                TextButton(onClick = onBack) { Text("← Kembali") }
                Text(
                    if (pages.isEmpty()) "" else "Hal. ${currentPage + 1}/${pages.size}",
                    modifier = Modifier.padding(top = 12.dp),
                    style = MaterialTheme.typography.bodySmall,
                )
            }
            Row(
                modifier = Modifier.fillMaxWidth().padding(horizontal = 8.dp),
                horizontalArrangement = Arrangement.spacedBy(6.dp),
            ) {
                OutlinedButton(onClick = { dataSaver = !dataSaver }) {
                    Text(if (dataSaver) "✓ Hemat data" else "Hemat data")
                }
                Button(
                    enabled = pages.isNotEmpty() && !ocrBusy,
                    onClick = {
                        val idx = currentPage
                        val page = pages.getOrNull(idx) ?: return@Button
                        ocrBusy = true
                        scope.launch {
                            bubbles = BubbleOcr.translatePage(context, page.imageUrl, script, targetLang)
                            ocrPage = idx
                            ocrBusy = false
                        }
                    },
                ) { Text(if (ocrBusy) "Menerjemahkan…" else "Terjemahkan Balon") }
            }
            if (ocrPage >= 0) {
                Text(
                    if (bubbles.isEmpty()) {
                        "Tidak ada teks terbaca di halaman ini."
                    } else {
                        "Terjemahan balon tampil menimpa halaman ${ocrPage + 1}."
                    },
                    style = MaterialTheme.typography.bodySmall,
                    modifier = Modifier.padding(horizontal = 16.dp, vertical = 4.dp),
                )
            }
            Row(
                modifier = Modifier.fillMaxWidth().padding(horizontal = 8.dp),
                horizontalArrangement = Arrangement.SpaceBetween,
            ) {
                if (prev != null) {
                    TextButton(onClick = { onOpenChapter(prev.id) }) {
                        Text("← Chapter ${prev.chapterNo.toLong()}")
                    }
                }
                if (next != null) {
                    TextButton(onClick = { onOpenChapter(next.id) }) {
                        Text("Chapter ${next.chapterNo.toLong()} →")
                    }
                }
            }
        }
        if (error != null) {
            item {
                Text(
                    "Gagal memuat: $error",
                    color = MaterialTheme.colorScheme.error,
                    modifier = Modifier.padding(16.dp),
                )
            }
        }
        itemsIndexed(pages) { idx, page ->
            val model = if (dataSaver) page.lightUrl ?: page.imageUrl else page.imageUrl
            val size = pageSizes[idx]
            BoxWithConstraints(modifier = Modifier.fillMaxWidth()) {
                val boxW = maxWidth
                AsyncImage(
                    model = model,
                    contentDescription = "Halaman ${page.pageNo}",
                    contentScale = ContentScale.FillWidth,
                    modifier = Modifier.fillMaxWidth(),
                    onSuccess = { st ->
                        pageSizes = pageSizes + (idx to st.painter.intrinsicSize)
                    },
                )
                if (idx == ocrPage && size != null && size.width > 0 && bubbles.isNotEmpty()) {
                    val boxH = boxW * (size.height / size.width)
                    Box(modifier = Modifier.fillMaxWidth().height(boxH)) {
                        bubbles.forEach { b ->
                            val bw = (boxW * (b.right - b.left)).coerceAtLeast(56.dp)
                            val bh = (boxH * (b.bottom - b.top)).coerceAtLeast(18.dp)
                            Box(
                                modifier = Modifier
                                    .offset(x = boxW * b.left, y = boxH * b.top)
                                    .width(bw)
                                    .height(bh)
                                    .background(
                                        MaterialTheme.colorScheme.surface.copy(alpha = 0.88f),
                                    )
                                    .padding(2.dp),
                            ) {
                                Text(b.translated, fontSize = 10.sp, lineHeight = 12.sp)
                            }
                        }
                    }
                }
            }
        }
        if (next != null || prev != null) {
            item {
                Row(
                    modifier = Modifier.fillMaxWidth().padding(12.dp),
                    horizontalArrangement = Arrangement.SpaceBetween,
                ) {
                    if (prev != null) {
                        OutlinedButton(onClick = { onOpenChapter(prev.id) }) {
                            Text("← Chapter sebelumnya")
                        }
                    }
                    if (next != null) {
                        Button(onClick = { onOpenChapter(next.id) }) {
                            Text("Chapter berikutnya →")
                        }
                    }
                }
            }
        }
    }
}
