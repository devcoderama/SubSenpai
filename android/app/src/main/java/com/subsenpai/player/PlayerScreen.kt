package com.subsenpai.player

import android.net.Uri
import android.view.ViewGroup
import androidx.compose.foundation.background
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.material3.FilterChip
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import androidx.compose.ui.viewinterop.AndroidView
import androidx.media3.common.MediaItem
import androidx.media3.common.MimeTypes
import androidx.media3.common.PlaybackException
import androidx.media3.common.Player
import androidx.media3.exoplayer.ExoPlayer
import androidx.media3.ui.PlayerView
import com.subsenpai.data.CatalogRepository
import com.subsenpai.data.GtxTranslator
import com.subsenpai.data.Playback
import com.subsenpai.data.SubtitleItem
import kotlinx.coroutines.launch
import java.util.Locale

/** Pilihan trek teks aktif di player. */
private sealed interface SubChoice {
    data class Remote(val sub: SubtitleItem) : SubChoice
    data class Local(val uri: Uri, val lang: String) : SubChoice
    data object None : SubChoice
}

private fun mimeOf(format: String): String = when (format) {
    "vtt" -> MimeTypes.TEXT_VTT
    "ass" -> "text/x-ssa"
    else -> MimeTypes.APPLICATION_SUBRIP
}

/**
 * Player HLS (Media3/ExoPlayer) dengan bahasa F40:
 * - Sumber softsub: trek subtitle siap pakai dari sumber bisa dipilih.
 * - Sumber hardsub: tiap bahasa adalah alirannya sendiri — memilih bahasa
 *   berarti berpindah aliran video.
 * - Bila bahasa perangkat belum punya trek siap pakai, chip "Terjemahkan"
 *   menjalankan cadangan gtx di perangkat (hasilnya di-cache lokal).
 * - Kegagalan putar berpindah otomatis ke sumber berikutnya (failover O9).
 */
@Composable
fun PlayerScreen(repo: CatalogRepository, episodeId: String, onBack: () -> Unit) {
    val context = LocalContext.current
    val scope = rememberCoroutineScope()
    val translator = remember { GtxTranslator() }
    val deviceLang = remember { Locale.getDefault().language.ifBlank { "id" } }

    var playback by remember { mutableStateOf<Playback?>(null) }
    var error by remember { mutableStateOf<String?>(null) }
    var sourceIdx by remember { mutableStateOf(0) }
    var subChoice by remember { mutableStateOf<SubChoice>(SubChoice.None) }
    var translating by remember { mutableStateOf(false) }
    var note by remember { mutableStateOf<String?>(null) }

    val player = remember {
        ExoPlayer.Builder(context).build().apply { playWhenReady = true }
    }

    LaunchedEffect(episodeId) {
        try {
            val pb = repo.playback(episodeId)
            playback = pb
            pb.subtitles.firstOrNull { it.status == "done" && it.fileUrl != null }?.let {
                subChoice = SubChoice.Remote(it)
            }
        } catch (e: Exception) {
            error = e.message
        }
    }

    LaunchedEffect(playback, sourceIdx, subChoice) {
        val pb = playback ?: return@LaunchedEffect
        val src = pb.sources.getOrNull(sourceIdx) ?: run {
            error = "Semua sumber video gagal diputar."
            return@LaunchedEffect
        }
        val builder = MediaItem.Builder().setUri(Uri.parse(src.url))
        when (val sc = subChoice) {
            is SubChoice.Remote -> builder.setSubtitleConfigurations(
                listOf(
                    MediaItem.SubtitleConfiguration.Builder(Uri.parse(sc.sub.fileUrl))
                        .setMimeType(mimeOf(sc.sub.format))
                        .setLanguage(sc.sub.lang)
                        .setSelectionFlags(1)
                        .build()
                )
            )
            is SubChoice.Local -> builder.setSubtitleConfigurations(
                listOf(
                    MediaItem.SubtitleConfiguration.Builder(sc.uri)
                        .setMimeType(MimeTypes.APPLICATION_SUBRIP)
                        .setLanguage(sc.lang)
                        .setSelectionFlags(1)
                        .build()
                )
            )
            SubChoice.None -> Unit
        }
        player.setMediaItem(builder.build())
        player.prepare()
    }

    DisposableEffect(player, playback) {
        val listener = object : Player.Listener {
            override fun onPlayerError(e: PlaybackException) {
                val pb = playback
                if (pb != null && sourceIdx < pb.sources.size - 1) {
                    sourceIdx++
                } else {
                    error = "Video gagal diputar: ${e.errorCodeName}"
                }
            }
        }
        player.addListener(listener)
        onDispose { player.removeListener(listener) }
    }
    DisposableEffect(Unit) {
        onDispose { player.release() }
    }

    val pb = playback
    Column(modifier = Modifier.fillMaxSize().background(Color.Black)) {
        TextButton(onClick = onBack) { Text("← Kembali", color = Color.White) }
        if (pb != null) {
            val hardLangs = pb.sources.mapNotNull { it.lang }.distinct()
            val readySubs = pb.subtitles.filter { it.status == "done" && it.fileUrl != null }
            val canTranslate = readySubs.isNotEmpty() &&
                readySubs.none { it.lang == deviceLang } && !translating
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .horizontalScroll(rememberScrollState())
                    .padding(horizontal = 8.dp),
            ) {
                hardLangs.forEach { lang ->
                    val idx = pb.sources.indexOfFirst { it.lang == lang }
                    FilterChip(
                        selected = sourceIdx == idx,
                        onClick = { sourceIdx = idx },
                        label = { Text("Video ${lang.uppercase()}") },
                        modifier = Modifier.padding(end = 6.dp),
                    )
                }
                readySubs.forEach { sub ->
                    FilterChip(
                        selected = (subChoice as? SubChoice.Remote)?.sub == sub,
                        onClick = { subChoice = SubChoice.Remote(sub) },
                        label = { Text("Sub ${sub.lang.uppercase()}") },
                        modifier = Modifier.padding(end = 6.dp),
                    )
                }
                if (readySubs.isNotEmpty() || hardLangs.isNotEmpty()) {
                    FilterChip(
                        selected = subChoice == SubChoice.None,
                        onClick = { subChoice = SubChoice.None },
                        label = { Text("Tanpa Sub") },
                        modifier = Modifier.padding(end = 6.dp),
                    )
                }
                if (canTranslate) {
                    FilterChip(
                        selected = subChoice is SubChoice.Local,
                        onClick = {
                            val base = readySubs.firstOrNull { it.lang == "en" } ?: readySubs.first()
                            translating = true
                            note = "Menerjemahkan subtitle ke ${deviceLang.uppercase()}…"
                            scope.launch {
                                val uri = translator.translateSubtitle(context, episodeId, base, deviceLang)
                                translating = false
                                if (uri != null) {
                                    subChoice = SubChoice.Local(uri, deviceLang)
                                    note = null
                                } else {
                                    note = "Terjemahan gagal — menampilkan subtitle asli."
                                }
                            }
                        },
                        label = { Text("Terjemahkan ${deviceLang.uppercase()}") },
                        modifier = Modifier.padding(end = 6.dp),
                    )
                }
            }
        }
        if (note != null) {
            Text(note!!, color = Color.LightGray, modifier = Modifier.padding(horizontal = 12.dp, vertical = 4.dp))
        }
        Box(modifier = Modifier.fillMaxSize()) {
            AndroidView(
                factory = {
                    PlayerView(it).apply {
                        this.player = player
                        layoutParams = ViewGroup.LayoutParams(
                            ViewGroup.LayoutParams.MATCH_PARENT,
                            ViewGroup.LayoutParams.MATCH_PARENT,
                        )
                    }
                },
                modifier = Modifier.fillMaxSize(),
            )
            if (error != null) {
                Text(
                    error!!,
                    color = MaterialTheme.colorScheme.error,
                    modifier = Modifier.padding(16.dp),
                )
            }
        }
    }
}
