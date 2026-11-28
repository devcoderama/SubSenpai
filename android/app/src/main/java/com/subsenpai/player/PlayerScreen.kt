package com.subsenpai.player

import android.app.Activity
import android.content.Context
import android.content.ContextWrapper
import android.content.Intent
import android.content.pm.ActivityInfo
import android.net.Uri
import android.view.KeyEvent
import android.view.MotionEvent
import android.view.View
import android.view.ViewGroup
import android.webkit.WebView
import android.webkit.WebChromeClient
import android.webkit.WebViewClient
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.itemsIndexed
import androidx.compose.foundation.lazy.grid.rememberLazyGridState
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import android.widget.Toast
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.RadioButton
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.key
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.viewinterop.AndroidView
import androidx.media3.common.MediaItem
import androidx.media3.common.MimeTypes
import androidx.media3.common.PlaybackException
import androidx.media3.common.Player
import androidx.media3.exoplayer.ExoPlayer
import androidx.media3.ui.AspectRatioFrameLayout
import androidx.media3.ui.PlayerView
import com.subsenpai.data.CatalogRepository
import com.subsenpai.data.EpisodeItem
import com.subsenpai.data.GtxTranslator
import com.subsenpai.data.Playback
import com.subsenpai.data.PlaybackSource
import com.subsenpai.data.SubtitleItem
import com.subsenpai.data.TitleCard
import com.subsenpai.data.TitleDetail
import com.subsenpai.ui.PosterImage
import kotlinx.coroutines.launch
import java.util.Locale

/** Palet halaman nonton (redesign ala YouTube mobile, aksen ungu). */
private val WatchBg = Color(0xFF0D0D0F)
private val WatchSurface = Color(0xFF19191D)
private val WatchAccent = Color(0xFF8B5CF6)
private val WatchText = Color(0xFFF4F4F5)
private val WatchMuted = Color(0xFFA1A1AA)

/** Pilihan trek teks aktif di player. */
private sealed interface SubChoice {
    data class Remote(val sub: SubtitleItem) : SubChoice
    data class Local(val uri: Uri, val lang: String) : SubChoice
    data object None : SubChoice
}

private enum class SheetKind { Server, Subtitle, Party }

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
 * Halaman nonton (redesign total v0.7.6): tata letak ala YouTube
 * mobile — header, player 16:9, info judul/episode, tombol aksi
 * (server/subtitle/terjemahan/party lewat BottomSheet), navigasi
 * episode, sinopsis, dan rekomendasi — semuanya bisa di-scroll
 * (LazyColumn) di bawah player yang tetap menempel di atas.
 *
 * Seluruh mekanisme lama dipertahankan: ExoPlayer untuk berkas
 * langsung + WebView untuk embed, failover server, subtitle siap
 * pakai & terjemahan gtx di perangkat, gerbang freemium, laporan
 * progres, dan sinkronisasi nonton bareng.
 */
@Composable
fun PlayerScreen(
    repo: CatalogRepository,
    episodeId: String,
    onBack: () -> Unit,
    onNeedLogin: () -> Unit = {},
    onOpenTitle: (String) -> Unit = {},
    onShareToPost: (TitleDetail) -> Unit = {},
    onOpenPremium: () -> Unit = {},
    onOpenAdUnlock: (String) -> Unit = {},
) {
    val context = LocalContext.current
    val scope = rememberCoroutineScope()
    val translator = remember { GtxTranslator() }
    val deviceLang = remember { Locale.getDefault().language.ifBlank { "id" } }

    // Episode yang sedang diputar bisa berganti dari dalam halaman
    // (tombol sebelumnya/berikutnya & daftar episode).
    var epId by remember(episodeId) { mutableStateOf(episodeId) }
    var playback by remember { mutableStateOf<Playback?>(null) }
    var detail by remember { mutableStateOf<TitleDetail?>(null) }
    var recommendations by remember { mutableStateOf<List<TitleCard>>(emptyList()) }
    var error by remember { mutableStateOf<String?>(null) }
    var locked by remember { mutableStateOf<String?>(null) }
    var sourceIdx by remember { mutableStateOf(0) }
    var subChoice by remember { mutableStateOf<SubChoice>(SubChoice.None) }
    var translating by remember { mutableStateOf(false) }
    var note by remember { mutableStateOf<String?>(null) }
    var sheet by remember { mutableStateOf<SheetKind?>(null) }
    var fullscreen by remember { mutableStateOf(false) }
    var descExpanded by remember { mutableStateOf(false) }
    var showReport by remember { mutableStateOf(false) }
    var reportReason by remember { mutableStateOf("semua_server_gagal") }
    var reportNote by remember { mutableStateOf("") }
    var reportSending by remember { mutableStateOf(false) }

    val player = remember {
        ExoPlayer.Builder(context).build().apply { playWhenReady = true }
    }

    LaunchedEffect(epId) {
        playback = null
        detail = null
        recommendations = emptyList()
        sourceIdx = 0
        subChoice = SubChoice.None
        error = null
        locked = null
        descExpanded = false
        try {
            // Hasil prefetch dari halaman detail dipakai bila ada:
            // Putar mulai tanpa menunggu jaringan lagi.
            val pb = repo.takeCachedPlayback(epId) ?: repo.playback(epId)
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

    // Info judul + daftar episode dari endpoint detail yang sudah ada.
    LaunchedEffect(playback?.titleId) {
        val tid = playback?.titleId
        if (tid.isNullOrBlank()) return@LaunchedEffect
        try {
            val d = repo.titleDetail(tid)
            detail = d
            try {
                recommendations = repo.titles(d.card.type, null)
                    .filter { it.id != d.card.id }
                    .take(12)
            } catch (e: Exception) {
                // Rekomendasi gagal diam-diam saja.
            }
        } catch (e: Exception) {
            // Info tambahan gagal: player tetap jalan dengan data putar.
        }
    }

    LaunchedEffect(playback, sourceIdx, subChoice) {
        val pb = playback ?: return@LaunchedEffect
        val src = pb.sources.getOrNull(sourceIdx) ?: run {
            error = "Semua sumber video gagal diputar."
            return@LaunchedEffect
        }
        if (isEmbedUrl(src.url)) {
            // Sumber embed diputar WebView (lihat kontainer player);
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

    // Lapor progres tonton berkala (Fase 3) agar "Lanjutkan" akurat
    // dan riwayat terisi. Sumber embed (WebView — film & donghua)
    // tidak memutar ExoPlayer sehingga posisinya selalu 0 dan
    // riwayat tidak pernah tercatat (keluhan Rams 2026-11-23);
    // untuk embed dipakai lama episode dibuka sebagai posisi.
    var embedWatchedSec by remember(epId) { mutableStateOf(0) }
    var anyReportSent by remember(epId) { mutableStateOf(false) }
    fun reportPosNow(): Int {
        val src = playback?.sources?.getOrNull(sourceIdx)
        return if (src != null && isEmbedUrl(src.url)) embedWatchedSec
        else (player.currentPosition / 1000).toInt()
    }
    LaunchedEffect(epId) {
        var first = true
        while (true) {
            kotlinx.coroutines.delay(5_000)
            val src = playback?.sources?.getOrNull(sourceIdx)
            if (src != null && isEmbedUrl(src.url)) embedWatchedSec += 5
            val pos = reportPosNow()
            // Tik pertama selalu melapor (walau posisi masih 0)
            // agar episode yang dibuka langsung masuk riwayat;
            // server menjaga posisi terbesar, jadi laporan awal
            // tidak menimpa progres lama.
            if (first || pos > 0) {
                repo.reportWatch(epId, pos)
                anyReportSent = true
            }
            first = false
        }
    }
    DisposableEffect(epId) {
        onDispose {
            val pos = reportPosNow()
            if (pos > 0 || anyReportSent) {
                scope.launch { repo.reportWatch(epId, pos) }
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

    // Layar penuh: putar ke lanskap + sembunyikan bar sistem
    // (mode imersif ala YouTube) selama aktif.
    val activity = context as? Activity
    LaunchedEffect(fullscreen) {
        activity?.requestedOrientation = if (fullscreen) {
            ActivityInfo.SCREEN_ORIENTATION_LANDSCAPE
        } else {
            ActivityInfo.SCREEN_ORIENTATION_UNSPECIFIED
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
            activity?.requestedOrientation = ActivityInfo.SCREEN_ORIENTATION_UNSPECIFIED
            val window = activity?.window
            if (window != null) {
                androidx.core.view.WindowInsetsControllerCompat(window, window.decorView)
                    .show(androidx.core.view.WindowInsetsCompat.Type.systemBars())
            }
        }
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
    val curSrc = pb?.sources?.getOrNull(sourceIdx)
    val readySubs = pb?.subtitles?.filter { it.status == "done" && it.fileUrl != null } ?: emptyList()
    val canTranslate = readySubs.isNotEmpty() &&
        readySubs.none { it.lang == deviceLang } && !translating
    val episodes = detail?.episodes ?: emptyList()
    val curEpIdx = episodes.indexOfFirst { it.id == epId }

    Column(modifier = Modifier.fillMaxSize().background(WatchBg)) {
        if (!fullscreen) {
            WatchHeader(
                title = detail?.card?.title,
                partyCode = partyCode,
                onBack = onBack,
            )
        }
        VideoPlayerContainer(
            player = player,
            curSrc = curSrc,
            error = error,
            locked = locked,
            loading = pb == null && locked == null && error == null,
            fullscreen = fullscreen,
            onToggleFullscreen = { fullscreen = !fullscreen },
            onNeedLogin = onNeedLogin,
            onOpenPremium = onOpenPremium,
            onAdUnlock = { onOpenAdUnlock(epId) },
            modifier = if (fullscreen) {
                Modifier.fillMaxSize()
            } else {
                Modifier.fillMaxWidth().aspectRatio(16f / 9f)
            },
        )
        if (!fullscreen) {
            LazyColumn(modifier = Modifier.weight(1f)) {
                item {
                    VideoInformation(
                        detail = detail,
                        playback = pb,
                        episodeTitle = episodes.getOrNull(curEpIdx)?.title,
                    )
                }
                if (pb != null) {
                    item {
                        PlayerActions(
                            sourceLabel = curSrc?.label
                                ?: curSrc?.lang?.let { "Video ${it.uppercase()}" }
                                ?: "Server",
                            subLabel = when (val sc = subChoice) {
                                is SubChoice.Remote -> "Sub ${sc.sub.lang.uppercase()}"
                                is SubChoice.Local -> "Sub ${sc.lang.uppercase()} (terjemahan)"
                                SubChoice.None -> "Subtitle"
                            },
                            canTranslate = canTranslate,
                            translating = translating,
                            showParty = repo.isLoggedIn,
                            partyActive = partyCode != null,
                            onOpenServer = { sheet = SheetKind.Server },
                            onOpenSubtitle = { sheet = SheetKind.Subtitle },
                            onOpenParty = { sheet = SheetKind.Party },
                            onReport = { showReport = true },
                            onShareToPost = { detail?.let { onShareToPost(it) } },
                            onShareSystem = {
                                val d = detail
                                if (d != null) {
                                    // Bagikan dari pemutar menautkan
                                    // EPISODE yang sedang diputar
                                    // (/nonton/{id}), bukan detail.
                                    val cur = episodes.getOrNull(curEpIdx)
                                    val send = Intent(Intent.ACTION_SEND).apply {
                                        type = "text/plain"
                                        putExtra(
                                            Intent.EXTRA_TEXT,
                                            com.subsenpai.data.shareTextFor(
                                                d, cur?.id, cur?.episodeNo,
                                            ),
                                        )
                                    }
                                    context.startActivity(Intent.createChooser(send, "Bagikan"))
                                }
                            },
                            onTranslate = {
                                val base = readySubs.firstOrNull { it.lang == "en" } ?: readySubs.first()
                                translating = true
                                note = "Menerjemahkan subtitle ke ${deviceLang.uppercase()}…"
                                scope.launch {
                                    val uri = translator.translateSubtitle(context, epId, base, deviceLang)
                                    translating = false
                                    if (uri != null) {
                                        subChoice = SubChoice.Local(uri, deviceLang)
                                        note = null
                                    } else {
                                        note = "Terjemahan gagal — menampilkan subtitle asli."
                                    }
                                }
                            },
                        )
                    }
                }
                if (note != null) {
                    item {
                        Text(
                            note!!,
                            color = WatchMuted,
                            style = MaterialTheme.typography.bodySmall,
                            modifier = Modifier.padding(horizontal = 16.dp, vertical = 2.dp),
                        )
                    }
                }
                if (episodes.isNotEmpty()) {
                    item {
                        EpisodeSelector(
                            episodes = episodes,
                            currentId = epId,
                            onSelect = { epId = it },
                            onPrev = episodes.getOrNull(curEpIdx - 1)?.let { prev ->
                                { if (prev.locked) onOpenPremium() else epId = prev.id }
                            },
                            onNext = episodes.getOrNull(curEpIdx + 1)?.let { next ->
                                { if (next.locked) onOpenPremium() else epId = next.id }
                            },
                            onOpenPremium = onOpenPremium,
                        )
                    }
                }
                val synopsis = detail?.synopsis
                if (!synopsis.isNullOrBlank()) {
                    item {
                        AnimeDescription(
                            synopsis = synopsis,
                            expanded = descExpanded,
                            onToggle = { descExpanded = !descExpanded },
                        )
                    }
                }
                if (recommendations.isNotEmpty()) {
                    item {
                        RecommendedAnime(
                            titles = recommendations,
                            onOpen = onOpenTitle,
                        )
                    }
                }
                item { Spacer(Modifier.height(28.dp)) }
            }
        }
    }

    // ---------- BottomSheet pengaturan ----------
    if (showReport) {
        AlertDialog(
            onDismissRequest = { showReport = false },
            title = { Text("Laporkan Masalah") },
            text = {
                Column {
                    Text(
                        "Ada masalah dengan episode ini? Ceritakan ke admin biar segera diperbaiki.",
                        style = MaterialTheme.typography.bodyMedium,
                    )
                    Spacer(Modifier.height(10.dp))
                    listOf(
                        "semua_server_gagal" to "Semua server tidak bisa diputar",
                        "link_rusak" to "Link rusak / video tidak muncul",
                        "video_salah" to "Video salah / tidak sesuai episode",
                        "subtitle" to "Subtitle bermasalah",
                        "lainnya" to "Lainnya",
                    ).forEach { (value, label) ->
                        Row(
                            verticalAlignment = Alignment.CenterVertically,
                            modifier = Modifier.fillMaxWidth().clickable { reportReason = value },
                        ) {
                            RadioButton(selected = reportReason == value, onClick = { reportReason = value })
                            Text(label, style = MaterialTheme.typography.bodyMedium)
                        }
                    }
                    Spacer(Modifier.height(8.dp))
                    OutlinedTextField(
                        value = reportNote,
                        onValueChange = { reportNote = it },
                        label = { Text("Catatan (opsional)") },
                        modifier = Modifier.fillMaxWidth(),
                        maxLines = 3,
                    )
                }
            },
            confirmButton = {
                TextButton(
                    enabled = !reportSending,
                    onClick = {
                        reportSending = true
                        scope.launch {
                            val ok = repo.createReport(epId, reportReason, reportNote.trim())
                            reportSending = false
                            showReport = false
                            Toast.makeText(
                                context,
                                if (ok) "Laporan terkirim. Terima kasih!" else "Gagal mengirim laporan, coba lagi.",
                                Toast.LENGTH_SHORT,
                            ).show()
                        }
                    },
                ) { Text(if (reportSending) "Mengirim…" else "Kirim Laporan") }
            },
            dismissButton = {
                TextButton(onClick = { showReport = false }) { Text("Batal") }
            },
        )
    }

    when (sheet) {
        SheetKind.Server -> if (pb != null) {
            ServerSheet(
                sources = pb.sources,
                selected = sourceIdx,
                onPick = { sourceIdx = it; sheet = null },
                onDismiss = { sheet = null },
            )
        }
        SheetKind.Subtitle -> if (pb != null) {
            SubtitleSheet(
                subs = readySubs,
                choice = subChoice,
                onPickRemote = { subChoice = SubChoice.Remote(it); sheet = null },
                onPickNone = { subChoice = SubChoice.None; sheet = null },
                onDismiss = { sheet = null },
            )
        }
        SheetKind.Party -> PartySheet(
            partyCode = partyCode,
            joinCode = partyJoinCode,
            onJoinCodeChange = { partyJoinCode = it.uppercase().take(6) },
            onCreate = { partySocket.value?.partyCreate(epId) },
            onJoin = { partySocket.value?.partyJoin(partyJoinCode) },
            onDismiss = { sheet = null },
        )
        null -> Unit
    }
}

@Composable
private fun WatchHeader(title: String?, partyCode: String?, onBack: () -> Unit) {
    Row(
        modifier = Modifier.fillMaxWidth().padding(horizontal = 4.dp, vertical = 2.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        TextButton(onClick = onBack) {
            Text("←", color = WatchText, style = MaterialTheme.typography.titleLarge)
        }
        Column(modifier = Modifier.weight(1f)) {
            Text(
                "SubSenpai",
                color = WatchAccent,
                fontWeight = FontWeight.Black,
                style = MaterialTheme.typography.titleMedium,
            )
            if (title != null) {
                Text(
                    title,
                    color = WatchMuted,
                    style = MaterialTheme.typography.bodySmall,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
            }
        }
        if (partyCode != null) {
            Surface(shape = RoundedCornerShape(50), color = WatchAccent) {
                Text(
                    "Party $partyCode",
                    color = Color.White,
                    style = MaterialTheme.typography.labelMedium,
                    modifier = Modifier.padding(horizontal = 10.dp, vertical = 4.dp),
                )
            }
            Spacer(Modifier.width(8.dp))
        }
    }
}

/**
 * Pendengar sentuh pemutar: meneruskan ketukan (onTapDown untuk
 * memunculkan tombol Layar Penuh) sambil membaca gestur cubit
 * lewat ScaleGestureDetector. Cubit ke luar (onPinch(true)) =
 * perbesar memenuhi layar; cubit ke dalam (onPinch(false)) =
 * kembali ke ukuran pas seluruh gambar — ala YouTube (permintaan
 * Rams 2026-11-24: subtitle hardsub terpotong di layar penuh
 * karena video 16:9 dipaksa memenuhi layar 20:9). Selalu
 * mengembalikan false: sentuhan tetap diteruskan ke pemutar
 * embed/ExoPlayer seperti biasa.
 */
private fun pinchZoomListener(
    context: android.content.Context,
    onTapDown: () -> Unit,
    onPinch: (Boolean) -> Unit,
): android.view.View.OnTouchListener {
    var product = 1f
    val detector = android.view.ScaleGestureDetector(
        context,
        object : android.view.ScaleGestureDetector.SimpleOnScaleGestureListener() {
            override fun onScaleBegin(d: android.view.ScaleGestureDetector): Boolean {
                product = 1f
                return true
            }

            override fun onScale(d: android.view.ScaleGestureDetector): Boolean {
                product *= d.scaleFactor
                return true
            }

            override fun onScaleEnd(d: android.view.ScaleGestureDetector) {
                if (product > 1.3f) {
                    onPinch(true)
                } else if (product < 0.75f) {
                    onPinch(false)
                }
            }
        },
    )
    return android.view.View.OnTouchListener { _, e ->
        if (e.action == MotionEvent.ACTION_DOWN) onTapDown()
        detector.onTouchEvent(e)
        false
    }
}

/**
 * Halaman pembungkus embed: viewport terkunci (tidak bisa
 * zoom/geser) dan iframe tepat 100% wadah, sehingga video pas
 * mengisi wadah seperti di YouTube — tidak terpotong dan halaman
 * luarnya tidak bisa di-scroll. Dipakai di SEMUA mode (potret &
 * layar penuh): sejak v0.24.22 WebView hanya boleh memuat SATU
 * halaman yang sama persis — dulu mode potret memuat URL langsung
 * dan layar penuh memuat pembungkus ini, ditambah key() komposisi
 * yang ikut berubah oleh status layar penuh, sehingga setiap
 * membesarkan/mengecilkan layar MEMBUANG WebView dan memuat ulang
 * dari nol (video terjeda / tertutup sendiri — keluhan Rams).
 * Kini beralih ukuran hanya mengubah ukuran wadah; halaman &
 * putaran video tidak disentuh sama sekali.
 */
private fun embedFullscreenHtml(url: String): String {
    val safe = url.replace("&", "&amp;").replace("\"", "&quot;")
    return "<!DOCTYPE html><html><head>" +
        "<meta name=\"viewport\" content=\"width=device-width, initial-scale=1, maximum-scale=1, user-scalable=no\">" +
        "<style>html,body{margin:0;padding:0;background:#000;width:100%;height:100%;overflow:hidden}" +
        "iframe{position:absolute;left:0;top:0;width:100%;height:100%;border:0}</style></head>" +
        "<body><iframe src=\"" + safe + "\" allow=\"autoplay; fullscreen; encrypted-media; picture-in-picture\" allowfullscreen></iframe></body></html>"
}

/**
 * Penampung fungsi keluar dari layar penuh bawaan pemutar embed,
 * diisi oleh WebChromeClient di dalam factory WebView (yang berjalan
 * satu kali) agar bisa dipanggil saat komposisi dibuang.
 */
private class NativeFsExitHolder {
    var exit: (() -> Unit)? = null
}

private fun findActivity(ctx: Context): Activity? {
    var c: Context? = ctx
    while (c is ContextWrapper) {
        if (c is Activity) return c
        c = c.baseContext
    }
    return null
}

private fun embedHostOf(url: String): String =
    runCatching { java.net.URL(url).host.lowercase() }.getOrDefault("")

/**
 * Potongan nama host pemutar embed yang DIKETAHUI punya tombol layar
 * penuh bawaan yang berfungsi. Untuk server-server ini tombol layar
 * penuh buatan sendiri disembunyikan — tombol bawaannya yang dipakai
 * (permintaan Rams 2026-11-25). Host lain yang tombol bawaannya
 * terbukti jalan ikut dicatat di perangkat (lihat loadNativeFsHosts)
 * dan perlakuannya sama.
 */
private val NATIVE_FS_HOST_TOKENS = listOf(
    "rumble", "youtube", "vk.com", "vkvideo", "ok.ru", "mp4upload",
    "dailymotion", "vimeo", "filemoon", "streamtape", "dood",
    "vidhide", "streamwish", "mixdrop", "voe.sx", "filelions",
    "vidguard", "turbovid", "streamhide", "drive.google",
)

private fun embedHasNativeFs(url: String, learned: Set<String>): Boolean {
    val host = embedHostOf(url)
    if (host.isEmpty()) return false
    return host in learned || NATIVE_FS_HOST_TOKENS.any { host.contains(it) }
}

private fun loadNativeFsHosts(ctx: Context): Set<String> =
    ctx.getSharedPreferences("subsenpai-player", Context.MODE_PRIVATE)
        .getStringSet("native_fs_hosts", emptySet())?.toSet() ?: emptySet()

private fun saveNativeFsHost(ctx: Context, host: String) {
    val prefs = ctx.getSharedPreferences("subsenpai-player", Context.MODE_PRIVATE)
    val cur = prefs.getStringSet("native_fs_hosts", emptySet())?.toSet() ?: emptySet()
    prefs.edit().putStringSet("native_fs_hosts", cur + host).apply()
}

@Composable
private fun VideoPlayerContainer(
    player: ExoPlayer,
    curSrc: PlaybackSource?,
    error: String?,
    locked: String?,
    loading: Boolean,
    fullscreen: Boolean,
    onToggleFullscreen: () -> Unit,
    onNeedLogin: () -> Unit,
    onOpenPremium: () -> Unit = {},
    onAdUnlock: () -> Unit = {},
    modifier: Modifier = Modifier,
) {
    // Halaman embed yang sedang dimuat WebView: selama ini wadah
    // menutupinya dengan animasi merek agar kilap putih bawaan
    // WebView (latar putih sebelum halaman selesai dilukis) tidak
    // terlihat sebagai kedip putih.
    var webLoading by remember(curSrc?.url) {
        mutableStateOf(curSrc != null && isEmbedUrl(curSrc.url))
    }
    // Mode skala video di layar penuh (ala YouTube): false = PAS
    // (seluruh gambar tampil sebagai kotak 16:9 di tengah layar —
    // subtitle aman), true = ZOOM (memenuhi layar, tepi
    // terpotong). Berganti lewat gestur cubit pada video; diatur
    // ulang tiap ganti sumber.
    var zoomed by remember(curSrc?.url) { mutableStateOf(false) }
    var zoomNote by remember { mutableStateOf<String?>(null) }
    LaunchedEffect(zoomNote) {
        if (zoomNote != null) {
            kotlinx.coroutines.delay(1300)
            zoomNote = null
        }
    }
    // onPinch ditangkap oleh pendengar sentuh di dalam factory
    // AndroidView (dibuat SATU kali) — status layar penuh harus
    // dibaca lewat rememberUpdatedState agar selalu nilai terbaru,
    // bukan nilai saat factory pertama berjalan (mode potret).
    val fullscreenNow = rememberUpdatedState(fullscreen)
    val onPinch: (Boolean) -> Unit = { out ->
        if (fullscreenNow.value && zoomed != out) {
            zoomed = out
            zoomNote = if (out) {
                "Memenuhi layar — cubit ke dalam untuk seluruh gambar"
            } else {
                "Seluruh gambar — cubit ke luar untuk memenuhi layar"
            }
        }
    }
    // Layar penuh BAWAAN pemutar embed (tombol layar penuh di dalam
    // pemutar Rumble/VK/dll): WebChromeClient di factory WebView
    // memasang tampilannya menutupi seluruh layar dan menyalakan
    // status ini. Selama aktif, orientasi + bar sistem mengikuti
    // mode ini dan tombol layar penuh buatan sendiri disembunyikan.
    // State disimpan sebagai objek MutableState (bukan delegasi saja)
    // karena factory AndroidView — yang berjalan satu kali — harus
    // bisa menulisnya juga.
    val appCtx = LocalContext.current
    val nativeFsState = remember { mutableStateOf(false) }
    var nativeFs by nativeFsState
    val learnedFsState = remember { mutableStateOf(loadNativeFsHosts(appCtx)) }
    var learnedFsHosts by learnedFsState
    val nativeFsExit = remember { NativeFsExitHolder() }
    LaunchedEffect(nativeFs) {
        val act = findActivity(appCtx) ?: return@LaunchedEffect
        val window = act.window
        val controller = androidx.core.view.WindowInsetsControllerCompat(window, window.decorView)
        if (nativeFs || fullscreenNow.value) {
            act.requestedOrientation = ActivityInfo.SCREEN_ORIENTATION_LANDSCAPE
            controller.hide(androidx.core.view.WindowInsetsCompat.Type.systemBars())
            controller.systemBarsBehavior =
                androidx.core.view.WindowInsetsControllerCompat.BEHAVIOR_SHOW_TRANSIENT_BARS_BY_SWIPE
        } else {
            act.requestedOrientation = ActivityInfo.SCREEN_ORIENTATION_UNSPECIFIED
            controller.show(androidx.core.view.WindowInsetsCompat.Type.systemBars())
        }
    }
    DisposableEffect(Unit) {
        onDispose { nativeFsExit.exit?.invoke() }
    }
    // Tombol Layar Penuh ala YouTube: muncul sesaat saat sumber baru
    // dimuat dan setiap kali video disentuh, lalu sembunyi sendiri.
    // Sejak 2026-11-24 (keluhan Rams): ikon saja tanpa teks, di pojok
    // KANAN ATAS — posisi lama di kanan bawah menimpa tombol bawaan
    // pemutar embed (volume/pengaturan). Sentuhan diamati lewat
    // OnTouchListener yang mengembalikan false, jadi ketukan tetap
    // diteruskan ke video.
    var controlsTick by remember { mutableStateOf(0) }
    var controlsVisible by remember { mutableStateOf(true) }
    val pokeControls = remember { { controlsTick++ } }
    LaunchedEffect(controlsTick, curSrc?.url, locked) {
        controlsVisible = true
        if (locked == null) {
            kotlinx.coroutines.delay(3000)
            controlsVisible = false
        }
    }
    BoxWithConstraints(modifier = modifier.background(Color.Black)) {
        // Daerah tampil video: layar penuh mode PAS memakai kotak
        // 16:9 terbesar yang muat di layar (di tengah, sisa hitam);
        // mode ZOOM & mode potret memakai seluruh wadah.
        val mediaModifier = if (fullscreen && !zoomed) {
            val w = if (maxWidth < maxHeight * 16f / 9f) maxWidth else maxHeight * 16f / 9f
            val h = if (maxHeight < maxWidth * 9f / 16f) maxHeight else maxWidth * 9f / 16f
            Modifier.align(Alignment.Center).size(w, h)
        } else {
            Modifier.fillMaxSize()
        }
        if (locked != null) {
            val premiumLock = locked.startsWith("premium_required")
            Column(
                modifier = Modifier.align(Alignment.Center).padding(20.dp),
                horizontalAlignment = Alignment.CenterHorizontally,
            ) {
                Text(
                    "🔒 " + locked.removePrefix("premium_required:").trim(),
                    color = Color.White,
                    style = MaterialTheme.typography.titleMedium,
                )
                Text(
                    if (premiumLock) {
                        "Film khusus Premium — atau buka episode ini dengan menonton iklan 60 detik."
                    } else {
                        "Masuk gratis untuk membuka semua episode — atau Premium untuk tanpa iklan."
                    },
                    color = Color.LightGray,
                    style = MaterialTheme.typography.bodySmall,
                    modifier = Modifier.padding(vertical = 8.dp),
                )
                if (premiumLock) {
                    Button(
                        onClick = onOpenPremium,
                        colors = ButtonDefaults.buttonColors(containerColor = WatchAccent),
                    ) { Text("Lihat Premium") }
                } else {
                    Button(
                        onClick = onNeedLogin,
                        colors = ButtonDefaults.buttonColors(containerColor = WatchAccent),
                    ) { Text("Masuk") }
                }
                // Alternatif gratis: buka episode ini dengan menonton
                // iklan (Monetag) — juga untuk tamu tanpa login.
                OutlinedButton(
                    onClick = onAdUnlock,
                    colors = ButtonDefaults.outlinedButtonColors(contentColor = Color.White),
                    modifier = Modifier.padding(top = 8.dp),
                ) { Text("Tonton Iklan untuk Membuka") }
            }
        } else if (curSrc != null && isEmbedUrl(curSrc.url)) {
            // key HANYA pada URL sumber: status layar penuh TIDAK
            // boleh masuk key — mengubah key membuang AndroidView
            // dan membuat WebView baru dari nol (muat ulang =
            // video terjeda/tertutup saat layar dibesarkan).
            key(curSrc.url) {
                AndroidView(
                    factory = {
                        WebView(it).apply {
                            val webCtx = it
                            // Latar gelap sejak awal: tanpa ini WebView
                            // berwarna putih sampai halaman embed
                            // selesai dilukis (kedip putih).
                            setBackgroundColor(0xFF0D0D0F.toInt())
                            // Amati ketukan untuk memunculkan tombol
                            // Layar Penuh + gestur cubit untuk mode
                            // Pas/Zoom; sentuhan selalu diteruskan
                            // ke halaman embed seperti biasa.
                            setOnTouchListener(
                                pinchZoomListener(
                                    context = it,
                                    onTapDown = { pokeControls() },
                                    onPinch = onPinch,
                                ),
                            )
                            // Halaman embed tidak boleh bisa digeser
                            // dari sisi WebView.
                            isVerticalScrollBarEnabled = false
                            isHorizontalScrollBarEnabled = false
                            overScrollMode = android.view.View.OVER_SCROLL_NEVER
                            settings.javaScriptEnabled = true
                            settings.domStorageEnabled = true
                            settings.mediaPlaybackRequiresUserGesture = false
                            settings.userAgentString =
                                "Mozilla/5.0 (Linux; Android 13) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Mobile Safari/537.36"
                            webViewClient = object : WebViewClient() {
                                override fun onPageFinished(view: WebView?, url: String?) {
                                    webLoading = false
                                }

                                @Deprecated("Deprecated in Java")
                                override fun onReceivedError(
                                    view: WebView?,
                                    errorCode: Int,
                                    description: String?,
                                    failingUrl: String?,
                                ) {
                                    webLoading = false
                                }
                            }
                            // Tombol layar penuh BAWAAN pemutar embed:
                            // tanpa WebChromeClient tombol itu mati —
                            // permintaan layar penuh dari halaman tidak
                            // pernah ditampilkan. Tampilan khususnya
                            // dipasang menutupi seluruh layar (di dekor
                            // jendela); keluar lewat tombol bawaan
                            // pemutar yang sama atau tombol Kembali.
                            webChromeClient = object : WebChromeClient() {
                                private var cView: View? = null
                                private var cCallback: CustomViewCallback? = null

                                private fun exitFs() {
                                    val v = cView ?: return
                                    (v.parent as? ViewGroup)?.removeView(v)
                                    cView = null
                                    val cb = cCallback
                                    cCallback = null
                                    cb?.onCustomViewHidden()
                                    nativeFsState.value = false
                                }

                                override fun onShowCustomView(
                                    view: View,
                                    callback: CustomViewCallback,
                                ) {
                                    if (cView != null) {
                                        callback.onCustomViewHidden()
                                        return
                                    }
                                    val decor = (findActivity(webCtx)?.window?.decorView) as? ViewGroup
                                    if (decor == null) {
                                        callback.onCustomViewHidden()
                                        return
                                    }
                                    cView = view
                                    cCallback = callback
                                    view.setBackgroundColor(android.graphics.Color.BLACK)
                                    decor.addView(
                                        view,
                                        ViewGroup.LayoutParams(
                                            ViewGroup.LayoutParams.MATCH_PARENT,
                                            ViewGroup.LayoutParams.MATCH_PARENT,
                                        ),
                                    )
                                    nativeFsExit.exit = { exitFs() }
                                    view.isFocusableInTouchMode = true
                                    view.requestFocus()
                                    view.setOnKeyListener { _, keyCode, event ->
                                        if (keyCode == KeyEvent.KEYCODE_BACK &&
                                            event.action == KeyEvent.ACTION_UP
                                        ) {
                                            exitFs()
                                            true
                                        } else {
                                            false
                                        }
                                    }
                                    // Host ini terbukti punya tombol layar
                                    // penuh bawaan yang jalan: catat agar
                                    // tombol buatan sendiri disembunyikan
                                    // untuk server ini mulai sekarang.
                                    val host = embedHostOf(curSrc.url)
                                    if (host.isNotEmpty() && host !in learnedFsState.value) {
                                        learnedFsState.value = learnedFsState.value + host
                                        saveNativeFsHost(webCtx, host)
                                    }
                                    nativeFsState.value = true
                                }

                                override fun onHideCustomView() {
                                    exitFs()
                                }
                            }
                            layoutParams = ViewGroup.LayoutParams(
                                ViewGroup.LayoutParams.MATCH_PARENT,
                                ViewGroup.LayoutParams.MATCH_PARENT,
                            )
                            // SATU muatan yang sama untuk potret &
                            // layar penuh (lihat embedFullscreenHtml):
                            // beralih ukuran tidak memuat ulang,
                            // video terus berputar tanpa jeda.
                            val origin = runCatching {
                                val u = java.net.URL(curSrc.url)
                                u.protocol + "://" + u.host
                            }.getOrDefault(curSrc.url)
                            loadDataWithBaseURL(
                                origin,
                                embedFullscreenHtml(curSrc.url),
                                "text/html",
                                "UTF-8",
                                null,
                            )
                        }
                    },
                    modifier = mediaModifier,
                )
            }
        } else {
            AndroidView(
                factory = {
                    PlayerView(it).apply {
                        this.player = player
                        setOnTouchListener(
                            pinchZoomListener(
                                context = it,
                                onTapDown = { pokeControls() },
                                onPinch = onPinch,
                            ),
                        )
                        layoutParams = ViewGroup.LayoutParams(
                            ViewGroup.LayoutParams.MATCH_PARENT,
                            ViewGroup.LayoutParams.MATCH_PARENT,
                        )
                    }
                },
                update = { view ->
                    view.resizeMode = if (fullscreen && zoomed) {
                        AspectRatioFrameLayout.RESIZE_MODE_ZOOM
                    } else {
                        AspectRatioFrameLayout.RESIZE_MODE_FIT
                    }
                },
                modifier = mediaModifier,
            )
        }
        if (loading || (curSrc != null && isEmbedUrl(curSrc.url) && webLoading)) {
            // Penutup gelap + animasi memuat ala Netflix selama
            // data playback atau halaman embed belum siap.
            Box(
                modifier = Modifier.fillMaxSize().background(WatchBg),
            ) {
                com.subsenpai.ui.BrandLoading(
                    modifier = Modifier.fillMaxSize(),
                    accent = WatchAccent,
                    compact = true,
                )
            }
        }
        if (error != null && locked == null) {
            Text(
                error,
                color = MaterialTheme.colorScheme.error,
                style = MaterialTheme.typography.bodySmall,
                modifier = Modifier.align(Alignment.BottomStart).padding(12.dp),
            )
        }
        if (fullscreen && zoomNote != null) {
            // Label mode skala sesaat sesudah gestur cubit (ala
            // YouTube: "Zoomed to fill" / "Original").
            Text(
                zoomNote!!,
                color = Color.White,
                style = MaterialTheme.typography.bodySmall,
                modifier = Modifier.align(Alignment.TopCenter).padding(top = 14.dp)
                    .background(Color(0xB3000000), RoundedCornerShape(14.dp))
                    .padding(horizontal = 12.dp, vertical = 6.dp),
            )
        }
        // Tombol layar penuh buatan sendiri disembunyikan bila pemutar
        // embed sumber ini punya tombol bawaan yang berfungsi (host
        // dikenal / tercatat di perangkat) — tombol bawaannya yang
        // dipakai — dan selama mode bawaan itu aktif. Saat layar penuh
        // buatan sendiri sedang aktif, tombol tetap tampil sebagai
        // jalan keluar.
        val embedNativeFs = curSrc != null && isEmbedUrl(curSrc.url) &&
            embedHasNativeFs(curSrc.url, learnedFsHosts)
        if (locked == null && !nativeFs && (!embedNativeFs || fullscreen)) {
            AnimatedVisibility(
                visible = controlsVisible,
                enter = fadeIn(),
                exit = fadeOut(),
                modifier = Modifier.align(Alignment.TopEnd),
            ) {
                Surface(
                    shape = RoundedCornerShape(8.dp),
                    color = Color(0x99000000),
                    modifier = Modifier
                        .padding(end = 10.dp, top = 10.dp)
                        .clickable { onToggleFullscreen() },
                ) {
                    Text(
                        "⤢",
                        color = Color.White,
                        style = MaterialTheme.typography.titleLarge,
                        modifier = Modifier.padding(horizontal = 9.dp, vertical = 3.dp),
                    )
                }
            }
        }
    }
}

@Composable
private fun VideoInformation(detail: TitleDetail?, playback: Playback?, episodeTitle: String?) {
    Column(modifier = Modifier.padding(horizontal = 16.dp, vertical = 12.dp)) {
        val card = detail?.card
        Text(
            card?.title ?: "Memuat judul…",
            color = WatchText,
            fontWeight = FontWeight.Bold,
            style = MaterialTheme.typography.titleLarge,
        )
        val epNo = playback?.episodeNo
        if (epNo != null) {
            Text(
                "Episode $epNo" + (episodeTitle?.let { " — $it" } ?: ""),
                color = WatchText,
                style = MaterialTheme.typography.titleSmall,
                modifier = Modifier.padding(top = 2.dp),
            )
        }
        val meta = listOfNotNull(
            card?.rating?.let { "★ $it" },
            card?.year?.toString(),
            detail?.durationMin?.let { "$it mnt/ep" },
            card?.type?.replaceFirstChar { it.uppercase() },
            card?.status,
        )
        if (meta.isNotEmpty()) {
            Text(
                meta.joinToString(" · "),
                color = WatchMuted,
                style = MaterialTheme.typography.bodyMedium,
                modifier = Modifier.padding(top = 4.dp),
            )
        }
        if (card != null && card.genres.isNotEmpty()) {
            Row(
                modifier = Modifier.padding(top = 8.dp).horizontalScroll(rememberScrollState()),
                horizontalArrangement = Arrangement.spacedBy(6.dp),
            ) {
                card.genres.forEach { g ->
                    Surface(shape = RoundedCornerShape(50), color = WatchSurface) {
                        Text(
                            g,
                            color = WatchText,
                            style = MaterialTheme.typography.labelMedium,
                            modifier = Modifier.padding(horizontal = 10.dp, vertical = 4.dp),
                        )
                    }
                }
            }
        }
    }
}

@Composable
private fun PlayerActions(
    sourceLabel: String,
    subLabel: String,
    canTranslate: Boolean,
    translating: Boolean,
    showParty: Boolean,
    partyActive: Boolean,
    onOpenServer: () -> Unit,
    onOpenSubtitle: () -> Unit,
    onOpenParty: () -> Unit,
    onTranslate: () -> Unit,
    onShareToPost: () -> Unit,
    onShareSystem: () -> Unit,
    onReport: () -> Unit,
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .horizontalScroll(rememberScrollState())
            .padding(horizontal = 16.dp, vertical = 4.dp),
        horizontalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        ActionButton(text = "Server: $sourceLabel", onClick = onOpenServer)
        ActionButton(text = subLabel, onClick = onOpenSubtitle)
        if (canTranslate || translating) {
            ActionButton(
                text = if (translating) "Menerjemahkan…" else "Terjemahkan",
                onClick = onTranslate,
            )
        }
        if (showParty) {
            ActionButton(
                text = if (partyActive) "Party Aktif" else "Nonton Bareng",
                accent = partyActive,
                onClick = onOpenParty,
            )
        }
        Box {
            var shareMenu by remember { mutableStateOf(false) }
            ActionButton(text = "Bagikan", onClick = { shareMenu = true })
            DropdownMenu(expanded = shareMenu, onDismissRequest = { shareMenu = false }) {
                DropdownMenuItem(
                    text = { Text("Bagikan ke Post") },
                    onClick = {
                        shareMenu = false
                        onShareToPost()
                    },
                )
                DropdownMenuItem(
                    text = { Text("Bagikan ke aplikasi lain") },
                    onClick = {
                        shareMenu = false
                        onShareSystem()
                    },
                )
            }
        }
        ActionButton(text = "Laporkan", onClick = onReport)
    }
}

@Composable
private fun ActionButton(text: String, accent: Boolean = false, onClick: () -> Unit) {
    Surface(
        shape = RoundedCornerShape(10.dp),
        color = if (accent) WatchAccent else WatchSurface,
        modifier = Modifier.clickable { onClick() },
    ) {
        Text(
            text,
            color = Color.White,
            style = MaterialTheme.typography.labelLarge,
            maxLines = 1,
            modifier = Modifier.padding(horizontal = 14.dp, vertical = 9.dp),
        )
    }
}

@Composable
private fun EpisodeSelector(
    episodes: List<EpisodeItem>,
    currentId: String,
    onSelect: (String) -> Unit,
    onPrev: (() -> Unit)?,
    onNext: (() -> Unit)?,
    onOpenPremium: () -> Unit = {},
) {
    Column(modifier = Modifier.padding(top = 10.dp)) {
        Row(
            modifier = Modifier.fillMaxWidth().padding(horizontal = 16.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text(
                "Episode",
                color = WatchText,
                fontWeight = FontWeight.Bold,
                style = MaterialTheme.typography.titleMedium,
                modifier = Modifier.weight(1f),
            )
            TextButton(onClick = { onPrev?.invoke() }, enabled = onPrev != null) {
                Text("‹ Prev", color = if (onPrev != null) WatchText else WatchMuted)
            }
            TextButton(onClick = { onNext?.invoke() }, enabled = onNext != null) {
                Text("Next ›", color = if (onNext != null) WatchText else WatchMuted)
            }
        }
        // Serial TV: saring grid per season (chip di atas grid) agar
        // 60+ episode lintas season tidak tercampur dalam satu grid
        // datar. Default = season episode yang sedang diputar; pindah
        // episode (Prev/Next/ketuk) mengembalikan saringan ke season
        // episode baru itu.
        val seasons = episodes.map { it.seasonNo }.distinct().sorted()
        val currentSeason = episodes.firstOrNull { it.id == currentId }?.seasonNo
            ?: seasons.firstOrNull() ?: 1
        var pickedSeason by remember(currentId) { mutableStateOf<Int?>(null) }
        val activeSeason = pickedSeason?.takeIf { it in seasons } ?: currentSeason
        val shownEpisodes = if (seasons.size > 1) {
            episodes.filter { it.seasonNo == activeSeason }
        } else {
            episodes
        }
        if (seasons.size > 1) {
            Row(
                modifier = Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 4.dp)
                    .horizontalScroll(rememberScrollState()),
                horizontalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                seasons.forEach { s ->
                    Surface(
                        shape = RoundedCornerShape(16.dp),
                        color = if (s == activeSeason) WatchAccent else WatchSurface,
                        modifier = Modifier.clickable { pickedSeason = s },
                    ) {
                        Text(
                            if (s == 0) "Spesial" else "Season $s",
                            color = Color.White,
                            style = MaterialTheme.typography.bodyMedium,
                            modifier = Modifier.padding(horizontal = 14.dp, vertical = 7.dp),
                        )
                    }
                }
            }
        }
        // Grid membungkus 5 per baris (1–5, 6–10, …) dengan scroll
        // vertikal sendiri — permintaan Rams 2026-11-24: deretan geser
        // menyamping menyiksa untuk judul berepisode puluhan sampai
        // ratusan. Episode aktif ditandai warna aksen + huruf tebal
        // (awalan ▶ dibuang agar nomor 3 digit muat di sel sempit),
        // dan grid langsung melompat ke episode yang sedang diputar.
        val gridState = rememberLazyGridState()
        LaunchedEffect(currentId, shownEpisodes.size) {
            val idx = shownEpisodes.indexOfFirst { it.id == currentId }
            if (idx >= 0) gridState.scrollToItem(idx)
        }
        LazyVerticalGrid(
            columns = GridCells.Fixed(5),
            state = gridState,
            modifier = Modifier
                .fillMaxWidth()
                .heightIn(max = 190.dp)
                .padding(horizontal = 16.dp, vertical = 4.dp),
            horizontalArrangement = Arrangement.spacedBy(8.dp),
            verticalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            itemsIndexed(shownEpisodes, key = { _, ep -> ep.id }) { _, ep ->
                val active = ep.id == currentId
                Surface(
                    shape = RoundedCornerShape(10.dp),
                    color = if (active) WatchAccent else WatchSurface,
                    modifier = Modifier.clickable {
                        // Gembok diketuk → langsung ke halaman Premium
                        // (permintaan Rams 2026-11-22, seperti di detail).
                        if (ep.locked) onOpenPremium() else onSelect(ep.id)
                    },
                ) {
                    Text(
                        (if (ep.locked) "🔒 " else "") + "Ep ${ep.episodeNo}",
                        color = Color.White,
                        fontWeight = if (active) FontWeight.Bold else FontWeight.Normal,
                        style = MaterialTheme.typography.labelLarge,
                        textAlign = TextAlign.Center,
                        maxLines = 1,
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(horizontal = 4.dp, vertical = 8.dp),
                    )
                }
            }
        }
    }
}

@Composable
private fun AnimeDescription(synopsis: String, expanded: Boolean, onToggle: () -> Unit) {
    Surface(
        shape = RoundedCornerShape(12.dp),
        color = WatchSurface,
        modifier = Modifier
            .fillMaxWidth()
            .padding(horizontal = 16.dp, vertical = 10.dp)
            .clickable { onToggle() },
    ) {
        Column(modifier = Modifier.padding(14.dp)) {
            Text(
                "Sinopsis",
                color = WatchText,
                fontWeight = FontWeight.Bold,
                style = MaterialTheme.typography.titleSmall,
            )
            Spacer(Modifier.height(4.dp))
            Text(
                synopsis,
                color = WatchMuted,
                style = MaterialTheme.typography.bodyMedium,
                maxLines = if (expanded) Int.MAX_VALUE else 3,
                overflow = TextOverflow.Ellipsis,
            )
            Text(
                if (expanded) "Tutup" else "Selengkapnya",
                color = WatchAccent,
                style = MaterialTheme.typography.labelLarge,
                modifier = Modifier.padding(top = 6.dp),
            )
        }
    }
}

@Composable
private fun RecommendedAnime(titles: List<TitleCard>, onOpen: (String) -> Unit) {
    Column(modifier = Modifier.padding(top = 8.dp)) {
        Text(
            "Rekomendasi",
            color = WatchText,
            fontWeight = FontWeight.Bold,
            style = MaterialTheme.typography.titleMedium,
            modifier = Modifier.padding(horizontal = 16.dp),
        )
        LazyRow(
            modifier = Modifier.padding(top = 8.dp),
            contentPadding = androidx.compose.foundation.layout.PaddingValues(horizontal = 16.dp),
            horizontalArrangement = Arrangement.spacedBy(10.dp),
        ) {
            items(titles) { t ->
                Column(
                    modifier = Modifier.width(110.dp).clickable { onOpen(t.id) },
                ) {
                    PosterImage(
                        url = t.posterUrl,
                        title = t.title,
                        modifier = Modifier
                            .width(110.dp)
                            .height(155.dp)
                            .background(WatchSurface, RoundedCornerShape(10.dp)),
                    )
                    Text(
                        t.title,
                        color = WatchText,
                        style = MaterialTheme.typography.bodySmall,
                        maxLines = 2,
                        overflow = TextOverflow.Ellipsis,
                        modifier = Modifier.padding(top = 4.dp),
                    )
                }
            }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun ServerSheet(
    sources: List<PlaybackSource>,
    selected: Int,
    onPick: (Int) -> Unit,
    onDismiss: () -> Unit,
) {
    ModalBottomSheet(
        onDismissRequest = onDismiss,
        sheetState = rememberModalBottomSheetState(),
        containerColor = WatchSurface,
    ) {
        Column(modifier = Modifier.padding(horizontal = 16.dp, vertical = 8.dp)) {
            Text(
                "Pilih Server",
                color = WatchText,
                fontWeight = FontWeight.Bold,
                style = MaterialTheme.typography.titleMedium,
            )
            Spacer(Modifier.height(8.dp))
            sources.forEachIndexed { i, s ->
                val label = s.label ?: "Server ${i + 1}"
                SheetRow(
                    text = label + (s.lang?.let { " · Video ${it.uppercase()}" } ?: ""),
                    active = i == selected,
                    onClick = { onPick(i) },
                )
            }
            Spacer(Modifier.height(20.dp))
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun SubtitleSheet(
    subs: List<SubtitleItem>,
    choice: SubChoice,
    onPickRemote: (SubtitleItem) -> Unit,
    onPickNone: () -> Unit,
    onDismiss: () -> Unit,
) {
    ModalBottomSheet(
        onDismissRequest = onDismiss,
        sheetState = rememberModalBottomSheetState(),
        containerColor = WatchSurface,
    ) {
        Column(modifier = Modifier.padding(horizontal = 16.dp, vertical = 8.dp)) {
            Text(
                "Subtitle",
                color = WatchText,
                fontWeight = FontWeight.Bold,
                style = MaterialTheme.typography.titleMedium,
            )
            Spacer(Modifier.height(8.dp))
            SheetRow(
                text = "Tanpa Subtitle",
                active = choice == SubChoice.None,
                onClick = onPickNone,
            )
            subs.forEach { sub ->
                SheetRow(
                    text = "Subtitle ${sub.lang.uppercase()}",
                    active = (choice as? SubChoice.Remote)?.sub == sub,
                    onClick = { onPickRemote(sub) },
                )
            }
            if (choice is SubChoice.Local) {
                Text(
                    "Terjemahan perangkat (${choice.lang.uppercase()}) sedang aktif.",
                    color = WatchMuted,
                    style = MaterialTheme.typography.bodySmall,
                    modifier = Modifier.padding(vertical = 6.dp),
                )
            }
            Spacer(Modifier.height(20.dp))
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun PartySheet(
    partyCode: String?,
    joinCode: String,
    onJoinCodeChange: (String) -> Unit,
    onCreate: () -> Unit,
    onJoin: () -> Unit,
    onDismiss: () -> Unit,
) {
    ModalBottomSheet(
        onDismissRequest = onDismiss,
        sheetState = rememberModalBottomSheetState(),
        containerColor = WatchSurface,
    ) {
        Column(modifier = Modifier.padding(horizontal = 16.dp, vertical = 8.dp)) {
            Text(
                "Nonton Bareng",
                color = WatchText,
                fontWeight = FontWeight.Bold,
                style = MaterialTheme.typography.titleMedium,
            )
            Spacer(Modifier.height(8.dp))
            if (partyCode == null) {
                Button(
                    onClick = onCreate,
                    colors = ButtonDefaults.buttonColors(containerColor = WatchAccent),
                    modifier = Modifier.fillMaxWidth(),
                ) { Text("Buat Party Baru") }
                Spacer(Modifier.height(10.dp))
                OutlinedTextField(
                    value = joinCode,
                    onValueChange = onJoinCodeChange,
                    label = { Text("Kode party (6 karakter)") },
                    singleLine = true,
                    modifier = Modifier.fillMaxWidth(),
                )
                Spacer(Modifier.height(8.dp))
                Button(
                    onClick = onJoin,
                    enabled = joinCode.length == 6,
                    colors = ButtonDefaults.buttonColors(containerColor = WatchSurface),
                    modifier = Modifier.fillMaxWidth(),
                ) { Text("Gabung Party") }
            } else {
                Text(
                    "Party aktif — kode: $partyCode",
                    color = WatchText,
                    style = MaterialTheme.typography.bodyLarge,
                )
                Text(
                    "Bagikan kodenya ke temanmu; putar/jeda/lompat tersinkron otomatis (pemutar langsung).",
                    color = WatchMuted,
                    style = MaterialTheme.typography.bodySmall,
                    modifier = Modifier.padding(top = 4.dp),
                )
            }
            Spacer(Modifier.height(20.dp))
        }
    }
}

@Composable
private fun SheetRow(text: String, active: Boolean, onClick: () -> Unit) {
    Surface(
        shape = RoundedCornerShape(10.dp),
        color = if (active) WatchAccent else Color.Transparent,
        modifier = Modifier
            .fillMaxWidth()
            .padding(vertical = 3.dp)
            .clickable { onClick() },
    ) {
        Text(
            (if (active) "✓ " else "") + text,
            color = Color.White,
            style = MaterialTheme.typography.bodyLarge,
            modifier = Modifier.padding(horizontal = 12.dp, vertical = 10.dp),
        )
    }
}
