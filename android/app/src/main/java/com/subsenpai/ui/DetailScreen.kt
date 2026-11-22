package com.subsenpai.ui

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.Card
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
import com.subsenpai.data.TitleDetail

@Composable
fun DetailScreen(
    repo: CatalogRepository,
    titleId: String,
    onPlay: (String) -> Unit,
    onRead: (String) -> Unit,
    onBack: () -> Unit,
) {
    var detail by remember { mutableStateOf<TitleDetail?>(null) }
    var error by remember { mutableStateOf<String?>(null) }

    LaunchedEffect(titleId) {
        try {
            detail = repo.titleDetail(titleId)
        } catch (e: Exception) {
            error = e.message
        }
    }

    LazyColumn(modifier = Modifier.fillMaxSize().padding(16.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
        item { TextButton(onClick = onBack) { Text("← Kembali") } }
        if (error != null) {
            item { Text("Gagal memuat: $error", color = MaterialTheme.colorScheme.error) }
        }
        val d = detail
        if (d != null) {
            item {
                Row {
                    AsyncImage(
                        model = d.card.posterUrl,
                        contentDescription = d.card.title,
                        contentScale = ContentScale.Crop,
                        modifier = Modifier.width(120.dp).aspectRatio(3f / 4f),
                    )
                    Spacer(Modifier.width(14.dp))
                    Column {
                        Text(d.card.title, style = MaterialTheme.typography.headlineSmall)
                        d.card.titleAlt?.let { Text(it, style = MaterialTheme.typography.bodyMedium) }
                        Text(
                            listOfNotNull(
                                d.card.type.replaceFirstChar { it.uppercase() },
                                d.card.year?.toString(),
                                d.card.status,
                                d.card.rating?.let { "★ $it" },
                            ).joinToString(" · "),
                            style = MaterialTheme.typography.bodySmall,
                        )
                        if (d.card.genres.isNotEmpty()) {
                            Text(d.card.genres.joinToString(", "), style = MaterialTheme.typography.bodySmall)
                        }
                    }
                }
                d.synopsis?.let {
                    Spacer(Modifier.height(10.dp))
                    Text(it, style = MaterialTheme.typography.bodyMedium)
                }
            }
            if (d.episodes.isNotEmpty()) {
                item { Text("Episode", style = MaterialTheme.typography.titleMedium) }
                items(d.episodes) { ep ->
                    Card(modifier = Modifier.fillMaxWidth().clickable { onPlay(ep.id) }) {
                        Text(
                            "Episode ${ep.episodeNo}" + (ep.title?.let { " — $it" } ?: ""),
                            modifier = Modifier.padding(14.dp),
                        )
                    }
                }
            }
            if (d.chapters.isNotEmpty()) {
                item { Text("Chapter", style = MaterialTheme.typography.titleMedium) }
                items(d.chapters) { ch ->
                    Card(modifier = Modifier.fillMaxWidth().clickable { onRead(ch.id) }) {
                        Text(
                            "Chapter ${formatChapterNo(ch.chapterNo)}" + (ch.title?.let { " — $it" } ?: "") +
                                " · ${ch.pageCount} halaman",
                            modifier = Modifier.padding(14.dp),
                        )
                    }
                }
            }
        }
    }
}

private fun formatChapterNo(n: Double): String =
    if (n == n.toLong().toDouble()) n.toLong().toString() else n.toString()
