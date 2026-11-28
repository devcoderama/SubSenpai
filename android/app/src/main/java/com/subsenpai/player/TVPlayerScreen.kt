package com.subsenpai.player

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
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
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.viewinterop.AndroidView
import androidx.media3.common.MediaItem
import androidx.media3.common.PlaybackException
import androidx.media3.common.Player
import androidx.media3.datasource.DefaultHttpDataSource
import androidx.media3.exoplayer.ExoPlayer
import androidx.media3.exoplayer.source.DefaultMediaSourceFactory
import androidx.media3.ui.PlayerView
import com.subsenpai.data.CatalogRepository
import com.subsenpai.ui.StabilityBadge

/**
 * Pemutar TV Langsung (IPTV, permintaan Rams 2026-11-28): stream
 * HLS (.m3u8) kanal diputar LANGSUNG dengan ExoPlayer dari
 * perangkat ke sumber — tanpa WebView, tanpa proxy VPS. Beberapa
 * kanal mewajibkan header Referer / User-Agent (mis. ANTV,
 * CNN/CNBC, BeritaSatu): header itu dikirim lewat
 * DefaultHttpDataSource.Factory untuk semua permintaan segmen.
 * Kanal 'tidak stabil' diberi catatan supaya pengguna tidak
 * mengira aplikasinya yang rusak bila stream mati.
 */
@Composable
fun TVPlayerScreen(
    repo: CatalogRepository,
    slug: String,
    onBack: () -> Unit,
) {
    val context = LocalContext.current
    var channel by remember { mutableStateOf<CatalogRepository.TVChannel?>(null) }
    var error by remember { mutableStateOf<String?>(null) }
    var playError by remember { mutableStateOf<String?>(null) }
    var buffering by remember { mutableStateOf(true) }
    var fullscreen by remember { mutableStateOf(false) }

    LaunchedEffect(slug) {
        try {
            channel = repo.tvChannel(slug)
        } catch (e: Exception) {
            error = e.message
        }
    }

    // Pabrik sumber HTTP per kanal: header wajib kanal (bila ada)
    // dipakai untuk playlist + semua segmen HLS.
    val dataSourceFactory = remember(channel) {
        DefaultHttpDataSource.Factory().apply {
            val props = mutableMapOf<String, String>()
            channel?.httpReferer?.let { props["Referer"] = it }
            channel?.httpUserAgent?.let { props["User-Agent"] = it }
            if (props.isNotEmpty()) setDefaultRequestProperties(props)
        }
    }
    val player = remember(dataSourceFactory) {
        ExoPlayer.Builder(context)
            .setMediaSourceFactory(DefaultMediaSourceFactory(dataSourceFactory))
            .build()
            .apply { playWhenReady = true }
    }
    DisposableEffect(Unit) {
        val listener = object : Player.Listener {
            override fun onPlaybackStateChanged(state: Int) {
                buffering = state == Player.STATE_BUFFERING
                if (state == Player.STATE_READY) playError = null
            }

            override fun onPlayerError(err: PlaybackException) {
                playError = err.message ?: "Stream gagal diputar."
            }
        }
        player.addListener(listener)
        onDispose {
            player.removeListener(listener)
            player.release()
        }
    }
    LaunchedEffect(channel) {
        val ch = channel ?: return@LaunchedEffect
        playError = null
        player.setMediaItem(MediaItem.fromUri(ch.streamUrl))
        player.prepare()
    }

    // Layar penuh: putar ke lanskap + sembunyikan bar sistem
    // (mode imersif ala YouTube) selama aktif — pola yang sama
    // dengan PlayerScreen.
    val activity = context as? android.app.Activity
    LaunchedEffect(fullscreen) {
        activity?.requestedOrientation = if (fullscreen) {
            android.content.pm.ActivityInfo.SCREEN_ORIENTATION_LANDSCAPE
        } else {
            android.content.pm.ActivityInfo.SCREEN_ORIENTATION_UNSPECIFIED
        }
        val window = activity?.window
        if (window != null) {
            val controller = androidx.core.view.WindowInsetsControllerCompat(window, window.decorView)
            if (fullscreen) {
                controller.hide(androidx.core.view.WindowInsetsCompat.Type.systemBars())
                controller.systemBarsBehavior =
                    androidx.core.view.WindowInsetsControllerCompat.BEHAVIOR_SHOW_TRANSIENT_BARS_BY_SWIPE
            } else {
                controller.show(androidx.core.view.WindowInsetsCompat.Type.systemBars())
            }
        }
    }
    DisposableEffect(Unit) {
        onDispose {
            activity?.requestedOrientation =
                android.content.pm.ActivityInfo.SCREEN_ORIENTATION_UNSPECIFIED
        }
    }

    Column(modifier = Modifier.fillMaxSize().background(Color(0xFF0D0D0F))) {
        if (!fullscreen) {
            Row(
                verticalAlignment = Alignment.CenterVertically,
                modifier = Modifier.fillMaxWidth().padding(top = 4.dp, end = 8.dp),
            ) {
                TextButton(onClick = onBack) { Text("← Kembali") }
                Text(
                    channel?.name ?: "TV Langsung",
                    style = MaterialTheme.typography.titleMedium,
                    fontWeight = FontWeight.Bold,
                    color = Color.White,
                    modifier = Modifier.weight(1f),
                )
                Box(
                    modifier = Modifier
                        .clip(RoundedCornerShape(4.dp))
                        .background(Color(0xFFE50914))
                        .padding(horizontal = 8.dp, vertical = 3.dp),
                ) {
                    Text(
                        "● LIVE",
                        color = Color.White,
                        style = MaterialTheme.typography.labelSmall,
                        fontWeight = FontWeight.Bold,
                    )
                }
            }
        }
        Box(
            modifier = if (fullscreen) {
                Modifier.fillMaxSize()
            } else {
                Modifier.fillMaxWidth().aspectRatio(16f / 9f)
            },
        ) {
            AndroidView(
                factory = {
                    PlayerView(it).apply {
                        this.player = player
                        useController = true
                        // Tombol layar penuh bawaan PlayerView baru
                        // tampil bila listener ini dipasang.
                        setFullscreenButtonClickListener { fullscreen = !fullscreen }
                        layoutParams = android.view.ViewGroup.LayoutParams(
                            android.view.ViewGroup.LayoutParams.MATCH_PARENT,
                            android.view.ViewGroup.LayoutParams.MATCH_PARENT,
                        )
                    }
                },
                modifier = Modifier.fillMaxSize(),
            )
            if (buffering && playError == null && channel != null) {
                Box(
                    modifier = Modifier.fillMaxSize(),
                    contentAlignment = Alignment.Center,
                ) {
                    CircularProgressIndicator(color = Color(0xFFE50914))
                }
            }
            if (playError != null) {
                Column(
                    modifier = Modifier.fillMaxSize().background(Color(0xCC000000))
                        .padding(16.dp),
                    horizontalAlignment = Alignment.CenterHorizontally,
                    verticalArrangement = Arrangement.Center,
                ) {
                    Text(
                        "Stream gagal diputar.",
                        color = Color.White,
                        fontWeight = FontWeight.Bold,
                    )
                    Text(
                        playError!!,
                        color = Color(0xFFB3B3B3),
                        style = MaterialTheme.typography.bodySmall,
                        modifier = Modifier.padding(top = 4.dp),
                    )
                    Spacer(Modifier.height(12.dp))
                    Button(onClick = {
                        playError = null
                        channel?.let {
                            player.setMediaItem(MediaItem.fromUri(it.streamUrl))
                            player.prepare()
                        }
                    }) { Text("Coba lagi") }
                }
            }
        }
        if (!fullscreen) {
            val ch = channel
            Column(modifier = Modifier.padding(horizontal = 16.dp, vertical = 12.dp)) {
                if (error != null) {
                    Text(
                        "Gagal memuat kanal: $error",
                        color = MaterialTheme.colorScheme.error,
                    )
                }
                if (ch != null) {
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        Text(
                            ch.name,
                            style = MaterialTheme.typography.titleLarge,
                            fontWeight = FontWeight.Bold,
                            color = Color.White,
                        )
                        Spacer(Modifier.width(8.dp))
                        StabilityBadge(ch.stability)
                    }
                    Text(
                        listOfNotNull(
                            ch.category,
                            ch.country,
                            ch.quality,
                        ).joinToString(" • "),
                        style = MaterialTheme.typography.bodyMedium,
                        color = Color(0xFFB3B3B3),
                        modifier = Modifier.padding(top = 4.dp),
                    )
                    if (ch.stability == "tidak stabil") {
                        Text(
                            "Stream ini tidak stabil — kadang mati. Bila berhenti, " +
                                "tutup lalu buka lagi kanalnya.",
                            style = MaterialTheme.typography.bodySmall,
                            color = Color(0xFFFFC107),
                            modifier = Modifier.padding(top = 8.dp),
                        )
                    }
                    Text(
                        "Stream diputar langsung dari sumbernya — tanpa perantara.",
                        style = MaterialTheme.typography.bodySmall,
                        color = Color(0xFF808080),
                        modifier = Modifier.padding(top = 8.dp),
                    )
                }
            }
        }
    }
}
