package com.subsenpai.player

import android.net.Uri
import android.view.ViewGroup
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
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
import com.subsenpai.data.Playback

/**
 * Player HLS (Media3/ExoPlayer).
 * - Sumber dicoba berurutan; kegagalan putar berpindah otomatis ke sumber
 *   cadangan berikutnya (failover O9, daftar sehat dari backend).
 * - Subtitle bahasa asal yang sudah jadi dilampirkan sebagai trek teks.
 * - Terjemahan subtitle dilakukan di perangkat (fase berikutnya, pola gtx).
 */
@Composable
fun PlayerScreen(repo: CatalogRepository, episodeId: String, onBack: () -> Unit) {
    val context = LocalContext.current
    var playback by remember { mutableStateOf<Playback?>(null) }
    var error by remember { mutableStateOf<String?>(null) }
    var sourceIdx by remember { mutableStateOf(0) }

    val player = remember {
        ExoPlayer.Builder(context).build().apply { playWhenReady = true }
    }

    LaunchedEffect(episodeId) {
        try {
            playback = repo.playback(episodeId)
        } catch (e: Exception) {
            error = e.message
        }
    }

    LaunchedEffect(playback, sourceIdx) {
        val pb = playback ?: return@LaunchedEffect
        val src = pb.sources.getOrNull(sourceIdx) ?: run {
            error = "Semua sumber video gagal diputar."
            return@LaunchedEffect
        }
        val builder = MediaItem.Builder().setUri(Uri.parse(src.url))
        pb.subtitles.firstOrNull { it.status == "done" && it.fileUrl != null }?.let { sub ->
            builder.setSubtitleConfigurations(
                listOf(
                    MediaItem.SubtitleConfiguration.Builder(Uri.parse(sub.fileUrl))
                        .setMimeType(if (sub.format == "ass") "text/x-ssa" else MimeTypes.APPLICATION_SUBRIP)
                        .setLanguage(sub.lang)
                        .setSelectionFlags(1) // C.SELECTION_FLAG_DEFAULT
                        .build()
                )
            )
        }
        player.setMediaItem(builder.build())
        player.prepare()
    }

    DisposableEffect(player, playback) {
        val listener = object : Player.Listener {
            override fun onPlayerError(e: PlaybackException) {
                val pb = playback
                if (pb != null && sourceIdx < pb.sources.size - 1) {
                    sourceIdx++ // failover ke sumber cadangan
                } else {
                    error = "Video gagal diputar: ${e.errorCodeName}"
                }
            }
        }
        player.addListener(listener)
        onDispose {
            player.removeListener(listener)
        }
    }
    DisposableEffect(Unit) {
        onDispose { player.release() }
    }

    Column(modifier = Modifier.fillMaxSize().background(Color.Black)) {
        TextButton(onClick = onBack) { Text("← Kembali", color = Color.White) }
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
