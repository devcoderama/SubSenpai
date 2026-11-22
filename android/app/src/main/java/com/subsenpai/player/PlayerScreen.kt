package com.subsenpai.player

import android.net.Uri
import android.view.ViewGroup
import android.webkit.WebView
import android.webkit.WebViewClient
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
 * Sumber donghua (Anichin) dan sebagian sumber lain adalah halaman
 * embed (ok.ru/videoembed, Rumble, pemutar pihak ketiga) — bukan
 * berkas media langsung, jadi ExoPlayer tidak bisa memutarnya;
 * sumber seperti itu dirender dengan WebView. Berkas media langsung
 * (m3u8/mp4/mkv/webm) tetap di ExoPlayer.
 */
private fun isEmbedUrl(url: String): Boolean {
    val path = url.substringBefore('?').lowercase()
    return !(path.endsWith(".m3u8") || path.endsWith(".mp4") || path.endsWith(".mkv") ||
        path.endsWith(".webm") || path.endsWith(".mov") || path.contains(".m3u8"))
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
fun PlayerScreen(
    repo: CatalogRepository,
    episodeId: String,
    onBack: () -> Unit,
    onNeedLogin: () -> Unit = {},
) {
    val context = LocalContext.current
    val scope = rememberCoroutineScope()
    val translator = remember { GtxTranslator() }
    val deviceLang = remember { Locale.getDefault().language.ifBlank { "id" } }

    var playback by remember { mutableStateOf<Playback?>(null) }
    var error by remember { mutableStateOf<String?>(null) }
    var locked by remember { mutableStateOf<String?>(null) }
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
        } catch (e: com.subsenpai.data.ApiException) {
            if (e.code == 403) {
                locked = e.message // gerbang freemium: batas tamu tercapai
            } else {
                error = e.message
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
        if (isEmbedUrl(src.url)) {
            // Sumber embed diputar WebView (lihat area video di bawah);
            // kosongkan ExoPlayer agar tidak ikut berbunyi.
            player.clearMediaItems()
            player.stop()
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

    // Lapor progres tonton berkala (Fase 3) agar "Lanjutkan" akurat.
    LaunchedEffect(episodeId) {
        while (true) {
            kotlinx.coroutines.delay(15_000)
            val pos = (player.currentPosition / 1000).toInt()
            if (pos > 0) repo.reportWatch(episodeId, pos)
        }
    }
    DisposableEffect(episodeId) {
        onDispose {
            val pos = (player.currentPosition / 1000).toInt()
            if (pos > 0) {
                scope.launch { repo.reportWatch(episodeId, pos) }
            }
        }
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

    // ---------- Nonton bareng (Fase 4) ----------
    var partyCode by remember { mutableStateOf<String?>(null) }
    var partyJoinCode by remember { mutableStateOf("") }
    val partySocket = remember { mutableStateOf<com.subsenpai.data.ChatSocket?>(null) }
    DisposableEffect(repo.auth?.token) {
        val token = repo.auth?.token
        val s = if (token != null) com.subsenpai.data.ChatSocket(token) else null
        partySocket.value = s
        if (s != null) {
            s.onEvent = { ev ->
                when (ev.optString("type")) {
                    "party_created", "party_joined" -> partyCode = ev.optString("code")
                    "party_sync" -> {
                        val pos = ev.optInt("position_sec") * 1000L
                        when (ev.optString("action")) {
                            "play" -> {
                                if (kotlin.math.abs(player.currentPosition - pos) > 2000) {
                                    player.seekTo(pos)
                                }
                                player.play()
                            }
                            "pause" -> player.pause()
                            "seek" -> player.seekTo(pos)
                        }
                    }
                }
            }
            s.connect()
        }
        onDispose { s?.close() }
    }
    // Teruskan aksi putar lokal ke ruang party.
    DisposableEffect(player, partyCode) {
        val code = partyCode
        val listener = object : Player.Listener {
            override fun onIsPlayingChanged(isPlaying: Boolean) {
                if (code != null) {
                    partySocket.value?.partySync(
                        code,
                        if (isPlaying) "play" else "pause",
                        (player.currentPosition / 1000).toInt(),
                    )
                }
            }

            override fun onPositionDiscontinuity(
                oldPosition: Player.PositionInfo,
                newPosition: Player.PositionInfo,
                reason: Int,
            ) {
                if (code != null && reason == Player.DISCONTINUITY_REASON_SEEK) {
                    partySocket.value?.partySync(code, "seek", (player.currentPosition / 1000).toInt())
                }
            }
        }
        if (code != null) player.addListener(listener)
        onDispose { player.removeListener(listener) }
    }

    val pb = playback
    Column(modifier = Modifier.fillMaxSize().background(Color.Black)) {
        TextButton(onClick = onBack) { Text("← Kembali", color = Color.White) }
        if (locked != null) {
            Text(
                "🔒 $locked",
                color = Color.White,
                style = MaterialTheme.typography.titleMedium,
                modifier = Modifier.padding(horizontal = 12.dp),
            )
            Text(
                "Masuk gratis untuk membuka semua episode — atau Premium untuk tanpa iklan.",
                color = Color.LightGray,
                modifier = Modifier.padding(horizontal = 12.dp, vertical = 4.dp),
            )
            androidx.compose.material3.Button(
                onClick = onNeedLogin,
                modifier = Modifier.padding(horizontal = 12.dp),
            ) { Text("Masuk") }
        }
        if (repo.isLoggedIn && pb != null) {
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .horizontalScroll(rememberScrollState())
                    .padding(horizontal = 8.dp, vertical = 4.dp),
            ) {
                if (partyCode == null) {
                    FilterChip(
                        selected = false,
                        onClick = { partySocket.value?.partyCreate(episodeId) },
                        label = { Text("Buat Nonton Bareng") },
                        modifier = Modifier.padding(end = 6.dp),
                    )
                    androidx.compose.material3.OutlinedTextField(
                        value = partyJoinCode,
                        onValueChange = { partyJoinCode = it.uppercase().take(6) },
                        label = { Text("Kode party") },
                        singleLine = true,
                        modifier = Modifier.padding(end = 6.dp),
                    )
                    FilterChip(
                        selected = false,
                        enabled = partyJoinCode.length == 6,
                        onClick = { partySocket.value?.partyJoin(partyJoinCode) },
                        label = { Text("Gabung") },
                    )
                } else {
                    FilterChip(
                        selected = true,
                        onClick = { },
                        label = { Text("Party: $partyCode (bagikan kodenya)") },
                    )
                }
            }
        }
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
        if (pb != null && pb.sources.size > 1) {
            // Pemilih server — penting untuk sumber embed donghua
            // (Okru, Rumble, Dailymotion, …): tiap chip satu server.
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .horizontalScroll(rememberScrollState())
                    .padding(horizontal = 8.dp, vertical = 4.dp),
            ) {
                pb.sources.forEachIndexed { i, s ->
                    FilterChip(
                        selected = sourceIdx == i,
                        onClick = { sourceIdx = i },
                        label = { Text(s.label ?: "Server ${i + 1}") },
                        modifier = Modifier.padding(end = 6.dp),
                    )
                }
            }
        }
        val curSrc = pb?.sources?.getOrNull(sourceIdx)
        Box(modifier = Modifier.fillMaxSize()) {
            if (curSrc != null && isEmbedUrl(curSrc.url)) {
                androidx.compose.runtime.key(curSrc.url) {
                    AndroidView(
                        factory = {
                            WebView(it).apply {
                                settings.javaScriptEnabled = true
                                settings.domStorageEnabled = true
                                settings.mediaPlaybackRequiresUserGesture = false
                                settings.userAgentString =
                                    "Mozilla/5.0 (Linux; Android 13) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Mobile Safari/537.36"
                                webViewClient = WebViewClient()
                                layoutParams = ViewGroup.LayoutParams(
                                    ViewGroup.LayoutParams.MATCH_PARENT,
                                    ViewGroup.LayoutParams.MATCH_PARENT,
                                )
                                loadUrl(curSrc.url)
                            }
                        },
                        modifier = Modifier.fillMaxSize(),
                    )
                }
            } else {
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
            }
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
