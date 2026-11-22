package com.subsenpai.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.unit.dp
import coil.compose.AsyncImage
import com.subsenpai.data.CatalogRepository
import com.subsenpai.data.ChapterPage

/** Reader komik: halaman ditampilkan vertikal (mode baca vertikal). */
@Composable
fun ReaderScreen(repo: CatalogRepository, chapterId: String, onBack: () -> Unit) {
    var pages by remember { mutableStateOf<List<ChapterPage>>(emptyList()) }
    var error by remember { mutableStateOf<String?>(null) }

    LaunchedEffect(chapterId) {
        try {
            pages = repo.chapterPages(chapterId)
        } catch (e: Exception) {
            error = e.message
        }
    }

    LazyColumn(modifier = Modifier.fillMaxSize(), verticalArrangement = Arrangement.spacedBy(2.dp)) {
        item {
            TextButton(onClick = onBack, modifier = Modifier.padding(8.dp)) { Text("← Kembali") }
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
        items(pages) { page ->
            AsyncImage(
                model = page.lightUrl ?: page.imageUrl,
                contentDescription = "Halaman ${page.pageNo}",
                contentScale = ContentScale.FillWidth,
                modifier = Modifier.fillMaxWidth(),
            )
        }
    }
}
