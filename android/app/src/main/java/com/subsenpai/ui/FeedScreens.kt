package com.subsenpai.ui

import android.content.Intent
import android.graphics.Bitmap
import android.graphics.BitmapFactory
import android.net.Uri
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.text.ClickableText
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
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TextField
import androidx.compose.material3.TextFieldDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.derivedStateOf
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.runtime.snapshotFlow
import androidx.compose.foundation.gestures.detectTransformGestures
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.window.Dialog
import androidx.compose.ui.window.DialogProperties
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextDecoration
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.viewinterop.AndroidView
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import coil.compose.AsyncImage
import com.subsenpai.R
import com.subsenpai.data.AuthStore
import com.subsenpai.data.CatalogRepository
import com.subsenpai.data.FeedAuthor
import com.subsenpai.data.FeedComment
import com.subsenpai.data.FeedNotification
import com.subsenpai.data.FeedPost
import com.subsenpai.data.FeedUploadManager
import com.subsenpai.data.FeedUploadState
import com.subsenpai.data.PendingPost
import com.subsenpai.data.ShareToPostBus
import com.subsenpai.data.GroupItem
import com.subsenpai.data.isVideo
import com.subsenpai.data.relativeTimeId
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import java.io.ByteArrayOutputStream
import kotlin.math.max
import kotlin.math.min

/** Aksen ungu komunitas SubSenpai (feed sosial). */
val FeedPurple = Color(0xFF8B5CF6)

private val urlRegex = Regex("""(https?://[^\s]+|www\.[^\s]+)""")

/** Teks dengan tautan yang bisa diketuk (feed membolehkan tautan). */
@Composable
fun LinkText(text: String, color: Color, style: TextStyle, modifier: Modifier = Modifier) {
    if (!urlRegex.containsMatchIn(text)) {
        Text(text, color = color, style = style, modifier = modifier)
        return
    }
    val context = LocalContext.current
    val annotated = buildAnnotatedString {
        var last = 0
        for (m in urlRegex.findAll(text)) {
            append(text.substring(last, m.range.first))
            pushStringAnnotation("URL", m.value)
            withStyle(SpanStyle(color = Color(0xFF6EA8FE), textDecoration = TextDecoration.Underline)) {
                append(m.value)
            }
            pop()
            last = m.range.last + 1
        }
        append(text.substring(last))
    }
    ClickableText(
        text = annotated,
        style = style.copy(color = color),
        modifier = modifier,
        onClick = { offset ->
            annotated.getStringAnnotations("URL", offset, offset).firstOrNull()?.let { ann ->
                val url = if (ann.item.startsWith("http")) ann.item else "https://" + ann.item
                try {
                    context.startActivity(Intent(Intent.ACTION_VIEW, Uri.parse(url)))
                } catch (e: Exception) {
                    // Abaikan tautan yang tidak bisa dibuka.
                }
            }
        },
    )
}

private val feedCategories = listOf(
    null to "Semua", "umum" to "Umum", "rekomendasi" to "Rekomendasi",
    "anime" to "Anime", "donghua" to "Donghua", "film" to "Film",
)

private val avatarColors = listOf(
    Color(0xFF7C4DFF), Color(0xFF00897B), Color(0xFFC2185B), Color(0xFF1976D2),
    Color(0xFFEF6C00), Color(0xFF5E35B1), Color(0xFF2E7D32), Color(0xFFAD1457),
)

@Composable
fun FeedAvatar(author: FeedAuthor, size: Int = 44) {
    val color = avatarColors[(author.displayName.hashCode() and Int.MAX_VALUE) % avatarColors.size]
    Box(
        modifier = Modifier.size(size.dp).background(color, CircleShape),
        contentAlignment = Alignment.Center,
    ) {
        if (author.avatarUrl != null) {
            AsyncImage(
                model = author.avatarUrl,
                contentDescription = author.displayName,
                contentScale = ContentScale.Crop,
                modifier = Modifier.fillMaxSize().clip(CircleShape),
            )
        } else {
            Text(
                author.displayName.take(1).uppercase(),
                color = Color.White,
                fontWeight = FontWeight.Bold,
                fontSize = (size / 2).sp,
            )
        }
    }
}

@Composable
private fun VerifiedBadge() {
    Box(
        modifier = Modifier.size(15.dp).background(FeedPurple, CircleShape),
        contentAlignment = Alignment.Center,
    ) {
        Text("✓", color = Color.White, fontSize = 9.sp, fontWeight = FontWeight.Bold)
    }
}

/** Pemutar video sebaris untuk lampiran video postingan (Media3). */
@Composable
fun FeedVideoPlayer(url: String, modifier: Modifier = Modifier) {
    val context = LocalContext.current
    val exo = remember(url) {
        androidx.media3.exoplayer.ExoPlayer.Builder(context).build().apply {
            setMediaItem(androidx.media3.common.MediaItem.fromUri(url))
            prepare()
        }
    }
    DisposableEffect(url) {
        onDispose { exo.release() }
    }
    AndroidView(
        factory = { ctx ->
            androidx.media3.ui.PlayerView(ctx).apply {
                player = exo
                useController = true
            }
        },
        modifier = modifier,
    )
}

/** URL embed pemutar untuk tautan YouTube/TikTok/Instagram/Facebook. */
fun socialEmbedUrl(url: String): String? {
    val yt = Regex("""(?:youtube\.com/(?:watch\?v=|shorts/|embed/)|youtu\.be/)([A-Za-z0-9_-]{6,})""", RegexOption.IGNORE_CASE).find(url)
    if (yt != null) return "https://www.youtube.com/embed/${yt.groupValues[1]}"
    val tt = Regex("""tiktok\.com/.*/video/(\d+)""", RegexOption.IGNORE_CASE).find(url)
    if (tt != null) return "https://www.tiktok.com/embed/v2/${tt.groupValues[1]}"
    val ig = Regex("""instagram\.com/(p|reel)/([A-Za-z0-9_-]+)""", RegexOption.IGNORE_CASE).find(url)
    if (ig != null) return "https://www.instagram.com/${ig.groupValues[1]}/${ig.groupValues[2]}/embed"
    if (url.contains("facebook.com/", true) || url.contains("fb.watch/", true)) {
        return "https://www.facebook.com/plugins/video.php?href=${Uri.encode(url)}&show_text=false"
    }
    return null
}

/**
 * Kotak pemutar embed media sosial di dalam kartu postingan.
 * Embed dimuat sebagai iframe di dalam halaman pembungkus yang
 * base URL-nya domain platform itu sendiri: memuat URL embed
 * langsung sebagai halaman utama membuat TikTok menolak tampil
 * (kotak hitam — temuan lapangan Rams 2026-11-22), karena pemutar
 * embed memeriksa asal/referer halaman induknya.
 */
@Composable
fun MediaEmbed(embedUrl: String) {
    val base = when {
        embedUrl.contains("tiktok.com") -> "https://www.tiktok.com/"
        embedUrl.contains("instagram.com") -> "https://www.instagram.com/"
        embedUrl.contains("facebook.com") -> "https://www.facebook.com/"
        else -> "https://www.youtube.com/"
    }
    // Tampilkan sedekat mungkin dengan ukuran asli kontennya:
    // TikTok & Instagram umumnya vertikal, YouTube/Facebook 16:9.
    val aspect = when {
        embedUrl.contains("tiktok.com") || embedUrl.contains("instagram.com") -> 4f / 5f
        else -> 16f / 9f
    }
    AndroidView(
        factory = { ctx ->
            android.webkit.WebView(ctx).apply {
                settings.javaScriptEnabled = true
                settings.domStorageEnabled = true
                settings.mediaPlaybackRequiresUserGesture = false
                settings.mixedContentMode = android.webkit.WebSettings.MIXED_CONTENT_ALWAYS_ALLOW
                settings.userAgentString =
                    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36"
                android.webkit.CookieManager.getInstance().setAcceptThirdPartyCookies(this, true)
                webViewClient = android.webkit.WebViewClient()
                val html = """<!DOCTYPE html><html><head><meta name="viewport" content="width=device-width, initial-scale=1.0"></head><body style="margin:0;padding:0;background:#000"><iframe src="$embedUrl" style="border:0;width:100%;height:100%;position:absolute;top:0;left:0" allow="autoplay; encrypted-media; fullscreen; picture-in-picture" allowfullscreen></iframe></body></html>"""
                loadDataWithBaseURL(base, html, "text/html", "utf-8", null)
            }
        },
        modifier = Modifier.fillMaxWidth().padding(top = 10.dp)
            .aspectRatio(aspect).clip(RoundedCornerShape(12.dp)),
    )
}

/** Tautan pendek yang harus diurai dulu sebelum bisa di-embed. */
private fun isShortSocialLink(url: String): Boolean =
    url.contains("vt.tiktok.com", true) || url.contains("vm.tiktok.com", true) ||
        url.contains("fb.watch", true)

/**
 * Embed sosial dari URL mentah postingan. Tautan pendek
 * (vt.tiktok.com dsb.) tidak memuat ID video, jadi diurai dulu
 * lewat pengalihan HTTP — temuan lapangan 2026-11-22: postingan
 * berisi tautan vt.tiktok.com tampil polos tanpa pemutar.
 */
@Composable
fun SocialEmbed(rawUrl: String, resolve: suspend (String) -> String?) {
    var embedUrl by remember(rawUrl) { mutableStateOf(socialEmbedUrl(rawUrl)) }
    if (embedUrl == null && isShortSocialLink(rawUrl)) {
        LaunchedEffect(rawUrl) {
            val resolved = resolve(rawUrl)
            if (resolved != null) {
                embedUrl = socialEmbedUrl(resolved)
            }
        }
    }
    embedUrl?.let { MediaEmbed(it) }
}

/** Baca gambar dari galeri → kompres JPEG maks 1600px agar unggahan ringan. */
private suspend fun loadCompressedImage(ctx: android.content.Context, uri: Uri): ByteArray? =
    withContext(Dispatchers.IO) {
        try {
            val bmp = ctx.contentResolver.openInputStream(uri)?.use { BitmapFactory.decodeStream(it) }
                ?: return@withContext null
            val scale = min(1f, 1600f / max(bmp.width, bmp.height))
            val out = if (scale < 1f) {
                Bitmap.createScaledBitmap(bmp, (bmp.width * scale).toInt(), (bmp.height * scale).toInt(), true)
            } else {
                bmp
            }
            val baos = ByteArrayOutputStream()
            out.compress(Bitmap.CompressFormat.JPEG, 85, baos)
            baos.toByteArray()
        } catch (e: Exception) {
            null
        }
}

/**
 * Halaman Chat = feed komunitas sosial (redesain total 2026-11-22,
 * referensi Threads/Facebook dari Rams): composer + tab Teman/Grup +
 * filter kategori (bekas kanal topik) + kartu postingan.
 */
@Composable
fun FeedScreen(
    repo: CatalogRepository,
    onNeedLogin: () -> Unit,
    onOpenComments: (String) -> Unit,
    onOpenNotifications: () -> Unit,
    onBack: () -> Unit,
    onOpenTitle: (String) -> Unit,
) {
    val context = LocalContext.current
    val scope = rememberCoroutineScope()
    val auth = remember { AuthStore(context) }

    var tab by remember { mutableStateOf("teman") }
    var category by remember { mutableStateOf<String?>(null) }
    var groupId by remember { mutableStateOf<String?>(null) }
    var myGroups by remember { mutableStateOf<List<GroupItem>>(emptyList()) }
    var query by remember { mutableStateOf("") }
    var searchOpen by remember { mutableStateOf(false) }
    var posts by remember { mutableStateOf<List<FeedPost>>(emptyList()) }
    var nextCursor by remember { mutableStateOf("") }
    var loading by remember { mutableStateOf(false) }
    var loadingMore by remember { mutableStateOf(false) }
    var feedError by remember { mutableStateOf<String?>(null) }
    var hiddenIds by remember { mutableStateOf<Set<String>>(emptySet()) }
    var unread by remember { mutableStateOf(0) }

    // Composer
    var draft by remember { mutableStateOf("") }
    var draftCategory by remember { mutableStateOf("umum") }
    var draftVisibility by remember { mutableStateOf("publik") }
    var draftTitleId by remember { mutableStateOf<String?>(null) }
    var viewerUrl by remember { mutableStateOf<String?>(null) }
    var mediaUri by remember { mutableStateOf<Uri?>(null) }
    var mediaBytes by remember { mutableStateOf<ByteArray?>(null) }
    var mediaIsVideo by remember { mutableStateOf(false) }
    var postError by remember { mutableStateOf<String?>(null) }
    val uploadState by FeedUploadManager.state.collectAsState()
    val sharePrefill by ShareToPostBus.prefill.collectAsState()

    // OpenDocument (bukan Photo Picker): di beberapa perangkat
    // (HyperOS) Photo Picker hanya menampilkan gambar walau diminta
    // ImageAndVideo — temuan lapangan Rams 2026-11-22.
    val picker = rememberLauncherForActivityResult(
        ActivityResultContracts.OpenDocument()
    ) { uri ->
        if (uri != null) {
            val mime = context.contentResolver.getType(uri)
                ?: if (uri.toString().contains(".mp4", true) ||
                    uri.toString().contains(".webm", true) ||
                    uri.toString().contains("video", true)
                ) {
                    "video/mp4"
                } else {
                    "image/jpeg"
                }
            mediaUri = uri
            postError = null
            if (mime.startsWith("video")) {
                mediaIsVideo = true
                scope.launch {
                    val bytes = withContext(Dispatchers.IO) {
                        try {
                            context.contentResolver.openInputStream(uri)?.use { it.readBytes() }
                        } catch (e: Exception) {
                            null
                        }
                    }
                    if (bytes != null && bytes.size > 50 * 1024 * 1024) {
                        postError = "Video maksimal 50 MB"
                        mediaUri = null
                        mediaBytes = null
                    } else {
                        mediaBytes = bytes
                    }
                }
            } else {
                mediaIsVideo = false
                scope.launch { mediaBytes = loadCompressedImage(context, uri) }
            }
        }
    }

    suspend fun loadFirst() {
        loading = true
        feedError = null
        try {
            val (items, next) = repo.feedList(
                tab, category, query.ifBlank { null },
                if (tab == "grup") groupId else null, null,
            )
            posts = items
            nextCursor = next
        } catch (e: Exception) {
            feedError = e.message
        } finally {
            loading = false
        }
    }

    LaunchedEffect(Unit) {
        if (repo.isLoggedIn) {
            try {
                myGroups = repo.groups().filter { it.isMember }
            } catch (e: Exception) {
                // Tab Grup menampilkan keadaan kosong bila gagal.
            }
            try {
                unread = repo.feedNotifications().second
            } catch (e: Exception) {
                // Lencana dilewati bila gagal.
            }
        }
    }
    LaunchedEffect(tab, category, groupId) {
        if (repo.isLoggedIn) loadFirst()
    }
    LaunchedEffect(query) {
        if (repo.isLoggedIn && searchOpen) {
            delay(500)
            loadFirst()
        }
    }
    // Unggahan latar belakang selesai → muat ulang feed, lalu
    // spanduk keberhasilan hilang sendiri.
    LaunchedEffect(uploadState) {
        if (uploadState is FeedUploadState.Done) {
            loadFirst()
            delay(3000)
            FeedUploadManager.dismiss()
        }
    }

    fun updatePost(updated: FeedPost) {
        posts = posts.map { if (it.id == updated.id) updated else it }
    }

    fun toggleLike(post: FeedPost) {
        val before = post
        updatePost(post.copy(liked = !post.liked, likeCount = post.likeCount + if (post.liked) -1 else 1))
        scope.launch {
            try {
                val (liked, count) = repo.feedLike(post.postId)
                updatePost(before.copy(liked = liked, likeCount = count))
            } catch (e: Exception) {
                updatePost(before)
            }
        }
    }

    fun toggleRepost(post: FeedPost) {
        val before = post
        updatePost(post.copy(reposted = !post.reposted, repostCount = post.repostCount + if (post.reposted) -1 else 1))
        scope.launch {
            try {
                val (reposted, count) = repo.feedRepost(post.postId)
                updatePost(before.copy(reposted = reposted, repostCount = count))
            } catch (e: Exception) {
                updatePost(before)
            }
        }
    }

    fun togglePreviewCommentLike(post: FeedPost, comment: FeedComment) {
        fun mapComment(c: FeedComment, liked: Boolean, count: Int) = c.copy(liked = liked, likeCount = count)
        updatePost(
            post.copy(
                commentsPreview = post.commentsPreview.map {
                    if (it.id == comment.id) mapComment(it, !comment.liked, comment.likeCount + if (comment.liked) -1 else 1) else it
                }
            )
        )
        scope.launch {
            try {
                val (liked, count) = repo.feedCommentLike(comment.id)
                val current = posts.firstOrNull { it.id == post.id } ?: return@launch
                updatePost(
                    current.copy(
                        commentsPreview = current.commentsPreview.map {
                            if (it.id == comment.id) mapComment(it, liked, count) else it
                        }
                    )
                )
            } catch (e: Exception) {
                updatePost(post)
            }
        }
    }

    fun sendInlineComment(post: FeedPost, text: String) {
        scope.launch {
            try {
                val created = repo.feedCommentCreate(post.postId, text, null)
                val current = posts.firstOrNull { it.id == post.id } ?: return@launch
                updatePost(
                    current.copy(
                        commentCount = current.commentCount + 1,
                        commentsPreview = current.commentsPreview + created,
                    )
                )
            } catch (e: Exception) {
                feedError = e.message
            }
        }
    }

    fun toggleBookmark(post: FeedPost) {
        val before = post
        updatePost(post.copy(bookmarked = !post.bookmarked))
        scope.launch {
            try {
                val saved = repo.feedBookmark(post.postId)
                updatePost(before.copy(bookmarked = saved))
            } catch (e: Exception) {
                updatePost(before)
            }
        }
    }

    if (!repo.isLoggedIn) {
        Column(
            modifier = Modifier.fillMaxSize().padding(24.dp),
            verticalArrangement = Arrangement.Center,
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            Text("Komunitas SubSenpai", style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.Bold)
            Spacer(Modifier.height(8.dp))
            Text("Masuk untuk melihat feed, membuat postingan, dan berinteraksi dengan komunitas.")
            Spacer(Modifier.height(14.dp))
            Button(onClick = onNeedLogin) { Text("Masuk") }
        }
        return
    }

    val listState = rememberLazyListState()
    val shouldLoadMore by remember {
        derivedStateOf {
            val info = listState.layoutInfo
            val last = info.visibleItemsInfo.lastOrNull()?.index ?: 0
            info.totalItemsCount > 0 && last >= info.totalItemsCount - 3
        }
    }
    LaunchedEffect(shouldLoadMore) {
        if (shouldLoadMore && nextCursor.isNotBlank() && !loading && !loadingMore) {
            loadingMore = true
            try {
                val (items, next) = repo.feedList(
                    tab, category, query.ifBlank { null },
                    if (tab == "grup") groupId else null, nextCursor,
                )
                posts = posts + items
                nextCursor = next
            } catch (e: Exception) {
                // Muat-lagi gagal: kursor dipertahankan untuk percobaan berikut.
            } finally {
                loadingMore = false
            }
        }
    }

    val composerFocus = remember { FocusRequester() }
    // "Bagikan ke Post" dari Detail/Player: terapkan ke composer.
    LaunchedEffect(sharePrefill) {
        val p = sharePrefill ?: return@LaunchedEffect
        draft = p.text
        draftCategory = p.category
        draftTitleId = p.titleId
        if (p.posterUrl != null) {
            mediaUri = Uri.parse(p.posterUrl)
            mediaIsVideo = false
            mediaBytes = null
            scope.launch {
                mediaBytes = repo.downloadBytes(p.posterUrl)
            }
        }
        ShareToPostBus.clear()
        listState.animateScrollToItem(0)
        composerFocus.requestFocus()
    }
    Box(modifier = Modifier.fillMaxSize()) {
    LazyColumn(
        state = listState,
        modifier = Modifier.fillMaxSize(),
        verticalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        item {
            Column(modifier = Modifier.fillMaxWidth()) {
                TextButton(onClick = onBack) {
                    Text("← Kembali", color = Color(0xFFE50914), fontWeight = FontWeight.Bold)
                }
                Row(
                    modifier = Modifier.fillMaxWidth().padding(horizontal = 16.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Text("Post", style = MaterialTheme.typography.headlineMedium, fontWeight = FontWeight.Bold)
                    Spacer(Modifier.weight(1f))
                    IconButton(onClick = {
                        searchOpen = !searchOpen
                        if (!searchOpen) query = ""
                    }) {
                        Icon(painterResource(R.drawable.ic_search), contentDescription = "Cari", tint = Color.White)
                    }
                    Box {
                        IconButton(onClick = onOpenNotifications) {
                            Icon(painterResource(R.drawable.ic_bell), contentDescription = "Notifikasi", tint = Color.White)
                        }
                        if (unread > 0) {
                            Box(
                                modifier = Modifier.align(Alignment.TopEnd).padding(4.dp)
                                    .background(Color(0xFFE50914), CircleShape)
                                    .padding(horizontal = 5.dp, vertical = 1.dp),
                            ) {
                                Text(unread.coerceAtMost(99).toString(), color = Color.White, fontSize = 10.sp, fontWeight = FontWeight.Bold)
                            }
                        }
                    }
                }
            }
        }
        if (searchOpen) {
            item {
                OutlinedTextField(
                    value = query,
                    onValueChange = { query = it },
                    label = { Text("Cari postingan…") },
                    singleLine = true,
                    modifier = Modifier.fillMaxWidth().padding(horizontal = 16.dp),
                )
            }
        }
        item {
            // Composer buat postingan.
            Card(
                modifier = Modifier.fillMaxWidth().padding(horizontal = 16.dp),
                colors = CardDefaults.cardColors(containerColor = Color(0xFF1F1F1F)),
                shape = RoundedCornerShape(16.dp),
            ) {
                Column(modifier = Modifier.padding(14.dp)) {
                    Row(verticalAlignment = Alignment.Top) {
                        Box(
                            modifier = Modifier.size(40.dp).background(FeedPurple, CircleShape),
                            contentAlignment = Alignment.Center,
                        ) {
                            Text(
                                (auth.displayName ?: "S").take(1).uppercase(),
                                color = Color.White, fontWeight = FontWeight.Bold,
                            )
                        }
                        Spacer(Modifier.width(10.dp))
                        TextField(
                            value = draft,
                            onValueChange = { draft = it },
                            placeholder = { Text("Apa yang kamu pikirkan?", color = Color(0xFF9E9EA7)) },
                            minLines = 1,
                            maxLines = 5,
                            modifier = Modifier.weight(1f).focusRequester(composerFocus),
                            colors = TextFieldDefaults.colors(
                                focusedContainerColor = Color.Transparent,
                                unfocusedContainerColor = Color.Transparent,
                                disabledContainerColor = Color.Transparent,
                                focusedIndicatorColor = Color.Transparent,
                                unfocusedIndicatorColor = Color.Transparent,
                                focusedTextColor = Color.White,
                                unfocusedTextColor = Color.White,
                                cursorColor = FeedPurple,
                            ),
                        )
                    }
                    if (mediaUri != null) {
                        Spacer(Modifier.height(10.dp))
                        Box {
                            if (mediaIsVideo) {
                                FeedVideoPlayer(
                                    mediaUri.toString(),
                                    Modifier.fillMaxWidth().aspectRatio(16f / 9f)
                                        .clip(RoundedCornerShape(10.dp)),
                                )
                            } else {
                                AsyncImage(
                                    model = mediaUri,
                                    contentDescription = "Lampiran",
                                    contentScale = ContentScale.Crop,
                                    modifier = Modifier.fillMaxWidth().aspectRatio(16f / 9f)
                                        .clip(RoundedCornerShape(10.dp)),
                                )
                            }
                            Box(
                                modifier = Modifier.align(Alignment.TopEnd).padding(6.dp)
                                    .background(Color(0xB3000000), CircleShape)
                                    .clickable { mediaUri = null; mediaBytes = null }
                                    .padding(6.dp),
                            ) {
                                Text("✕", color = Color.White, fontWeight = FontWeight.Bold)
                            }
                        }
                    }
                    if (postError != null) {
                        Text(
                            postError!!,
                            color = MaterialTheme.colorScheme.error,
                            style = MaterialTheme.typography.bodySmall,
                            modifier = Modifier.padding(top = 8.dp),
                        )
                    }
                    Row(
                        modifier = Modifier.padding(top = 10.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        IconButton(onClick = {
                            picker.launch(arrayOf("image/*", "video/*"))
                        }) {
                            Icon(painterResource(R.drawable.ic_image), contentDescription = "Tambah gambar/video", tint = FeedPurple)
                        }
                        CategoryMenu(selected = draftCategory, onSelect = { draftCategory = it })
                        VisibilityMenu(selected = draftVisibility, onSelect = { draftVisibility = it })
                        Spacer(Modifier.weight(1f))
                        Button(
                            enabled = (draft.isNotBlank() || mediaBytes != null) &&
                                (tab != "grup" || groupId != null),
                            onClick = {
                                // Unggah di latar belakang (ala FB/IG):
                                // composer langsung bebas, statusnya
                                // tampil sebagai spanduk di atas feed.
                                FeedUploadManager.enqueue(
                                    repo,
                                    PendingPost(
                                        body = draft.trim(),
                                        category = draftCategory,
                                        visibility = draftVisibility,
                                        groupId = if (tab == "grup") groupId else null,
                                        mediaBytes = mediaBytes,
                                        isVideo = mediaIsVideo,
                                        titleId = draftTitleId,
                                    ),
                                )
                                draft = ""
                                mediaUri = null
                                mediaBytes = null
                                draftTitleId = null
                                postError = null
                            },
                            colors = ButtonDefaults.buttonColors(containerColor = FeedPurple),
                            shape = RoundedCornerShape(20.dp),
                        ) {
                            Text("Posting")
                            Spacer(Modifier.width(4.dp))
                            Icon(painterResource(R.drawable.ic_send), contentDescription = null, modifier = Modifier.size(15.dp))
                        }
                    }
                    if (tab == "grup" && groupId == null) {
                        Text(
                            "Pilih grup di bawah untuk memposting ke grup.",
                            style = MaterialTheme.typography.bodySmall,
                            color = Color(0xFFB3B3B3),
                            modifier = Modifier.padding(top = 6.dp),
                        )
                    }
                }
            }
        }
        if (uploadState !is FeedUploadState.Idle) {
            item {
                // Spanduk status unggahan latar belakang.
                Card(
                    modifier = Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 4.dp),
                    colors = CardDefaults.cardColors(containerColor = Color(0xFF262033)),
                    shape = RoundedCornerShape(12.dp),
                ) {
                    Row(
                        modifier = Modifier.padding(horizontal = 14.dp, vertical = 10.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        when (val st = uploadState) {
                            is FeedUploadState.Uploading -> {
                                CircularProgressIndicator(color = FeedPurple, modifier = Modifier.size(16.dp), strokeWidth = 2.dp)
                                Spacer(Modifier.width(10.dp))
                                Text(
                                    "Mengunggah postingan… kamu bisa lanjut scroll",
                                    style = MaterialTheme.typography.bodyMedium,
                                    color = Color.White,
                                    modifier = Modifier.weight(1f),
                                )
                            }
                            is FeedUploadState.Done -> {
                                Text(
                                    "Postingan terbit ✓",
                                    style = MaterialTheme.typography.bodyMedium,
                                    color = Color.White,
                                    modifier = Modifier.weight(1f),
                                )
                            }
                            is FeedUploadState.Failed -> {
                                Text(
                                    "Unggahan gagal: ${st.message}",
                                    style = MaterialTheme.typography.bodyMedium,
                                    color = Color(0xFFFF8A80),
                                    modifier = Modifier.weight(1f),
                                )
                                TextButton(onClick = { FeedUploadManager.retry(repo) }) {
                                    Text("Coba lagi", color = FeedPurple)
                                }
                                TextButton(onClick = { FeedUploadManager.dismiss() }) {
                                    Text("Buang", color = Color(0xFFB3B3B3))
                                }
                            }
                            else -> {}
                        }
                    }
                }
            }
        }
        item {
            // Tab Teman / Grup (pil ungu ala referensi).
            Row(
                modifier = Modifier.fillMaxWidth().padding(horizontal = 16.dp),
                horizontalArrangement = Arrangement.spacedBy(10.dp),
            ) {
                TabPill(R.drawable.ic_person, "Teman", tab == "teman", Modifier.weight(1f)) { tab = "teman" }
                TabPill(R.drawable.ic_groups, "Grup", tab == "grup", Modifier.weight(1f)) { tab = "grup" }
            }
        }
        if (tab == "grup") {
            item {
                if (myGroups.isEmpty()) {
                    Text(
                        "Kamu belum bergabung dengan grup mana pun. Buka Lainnya › Grup Chat untuk bergabung atau membuat grup.",
                        style = MaterialTheme.typography.bodySmall,
                        color = Color(0xFFB3B3B3),
                        modifier = Modifier.padding(horizontal = 16.dp),
                    )
                } else {
                    LazyRow(
                        horizontalArrangement = Arrangement.spacedBy(8.dp),
                        modifier = Modifier.padding(horizontal = 16.dp),
                    ) {
                        item { FilterChip2("Semua grup", groupId == null) { groupId = null } }
                        items(myGroups) { g ->
                            FilterChip2(g.name, groupId == g.id) { groupId = g.id }
                        }
                    }
                }
            }
        }
        item {
            LazyRow(
                horizontalArrangement = Arrangement.spacedBy(8.dp),
                modifier = Modifier.padding(horizontal = 16.dp),
            ) {
                items(feedCategories) { (value, label) ->
                    FilterChip2(label, category == value) { category = value }
                }
            }
        }
        if (loading) {
            item {
                Box(Modifier.fillMaxWidth().padding(32.dp), contentAlignment = Alignment.Center) {
                    CircularProgressIndicator(color = FeedPurple)
                }
            }
        }
        if (feedError != null) {
            item {
                Text(
                    "Gagal memuat feed: $feedError",
                    color = MaterialTheme.colorScheme.error,
                    modifier = Modifier.padding(horizontal = 16.dp),
                )
            }
        }
        if (!loading && feedError == null && posts.isEmpty()) {
            item {
                Text(
                    when {
                        query.isNotBlank() -> "Tidak ada postingan yang cocok dengan pencarianmu."
                        tab == "grup" -> "Belum ada postingan di grup ini. Jadilah yang pertama posting!"
                        else -> "Belum ada postingan dari teman. Tambah teman lewat Lainnya › Teman, atau buat postingan pertamamu di atas."
                    },
                    color = Color(0xFFB3B3B3),
                    modifier = Modifier.padding(horizontal = 16.dp, vertical = 12.dp),
                )
            }
        }
        items(posts.filter { it.id !in hiddenIds }, key = { it.id }) { post ->
            FeedPostCard(
                post = post,
                myName = auth.displayName ?: "Saya",
                onLike = { toggleLike(post) },
                onBookmark = { toggleBookmark(post) },
                onRepost = { toggleRepost(post) },
                onComments = { onOpenComments(post.postId) },
                onCommentLike = { c -> togglePreviewCommentLike(post, c) },
                onInlineComment = { text -> sendInlineComment(post, text) },
                onDelete = {
                    scope.launch {
                        try {
                            repo.feedDeletePost(post.id)
                            posts = posts.filter { it.id != post.id }
                        } catch (e: Exception) {
                            feedError = e.message
                        }
                    }
                },
                onHide = { hiddenIds = hiddenIds + post.id },
                onResolveUrl = { repo.resolveUrl(it) },
                onOpenTitle = onOpenTitle,
                onImageClick = { viewerUrl = it },
            )
        }
        if (loadingMore) {
            item {
                Box(Modifier.fillMaxWidth().padding(16.dp), contentAlignment = Alignment.Center) {
                    CircularProgressIndicator(color = FeedPurple, modifier = Modifier.size(22.dp))
                }
            }
        }
        item { Spacer(Modifier.height(16.dp)) }
    }
        // Tombol tulis melayang (ala referensi): kembali ke composer.
        Box(
            modifier = Modifier.align(Alignment.BottomEnd).padding(20.dp)
                .size(56.dp).background(FeedPurple, CircleShape)
                .clickable {
                    scope.launch {
                        listState.animateScrollToItem(0)
                        composerFocus.requestFocus()
                    }
                },
            contentAlignment = Alignment.Center,
        ) {
            Icon(painterResource(R.drawable.ic_edit), contentDescription = "Buat postingan", tint = Color.White)
        }
        if (viewerUrl != null) {
            FullImageViewer(url = viewerUrl!!, onDismiss = { viewerUrl = null })
        }
    }
}

/** Penampil gambar layar penuh ala Facebook: cubit untuk memperbesar. */
@Composable
fun FullImageViewer(url: String, onDismiss: () -> Unit) {
    var scale by remember { mutableStateOf(1f) }
    var offset by remember { mutableStateOf(Offset.Zero) }
    Dialog(
        onDismissRequest = onDismiss,
        properties = DialogProperties(usePlatformDefaultWidth = false),
    ) {
        Box(
            modifier = Modifier.fillMaxSize().background(Color.Black)
                .clickable { if (scale <= 1f) onDismiss() },
            contentAlignment = Alignment.Center,
        ) {
            AsyncImage(
                model = url,
                contentDescription = "Gambar ukuran penuh",
                contentScale = ContentScale.Fit,
                modifier = Modifier.fillMaxSize()
                    .graphicsLayer(
                        scaleX = scale, scaleY = scale,
                        translationX = offset.x, translationY = offset.y,
                    )
                    .pointerInput(url) {
                        detectTransformGestures { _, pan, zoom, _ ->
                            scale = (scale * zoom).coerceIn(1f, 5f)
                            offset = if (scale > 1f) offset + pan else Offset.Zero
                        }
                    },
            )
            IconButton(
                onClick = onDismiss,
                modifier = Modifier.align(Alignment.TopEnd).padding(12.dp),
            ) {
                Text("✕", color = Color.White, style = MaterialTheme.typography.titleLarge)
            }
        }
    }
}

@Composable
private fun TabPill(icon: Int, label: String, selected: Boolean, modifier: Modifier = Modifier, onClick: () -> Unit) {
    Box(
        modifier = modifier.height(40.dp)
            .background(if (selected) FeedPurple else Color(0xFF1F1F1F), RoundedCornerShape(20.dp))
            .clickable { onClick() },
        contentAlignment = Alignment.Center,
    ) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Icon(
                painterResource(icon),
                contentDescription = null,
                tint = if (selected) Color.White else Color(0xFF9E9EA7),
                modifier = Modifier.size(19.dp),
            )
            Spacer(Modifier.width(7.dp))
            Text(
                label,
                color = if (selected) Color.White else Color(0xFF9E9EA7),
                fontWeight = if (selected) FontWeight.Bold else FontWeight.Normal,
            )
        }
    }
}

@Composable
private fun FilterChip2(label: String, selected: Boolean, onClick: () -> Unit) {
    Box(
        modifier = Modifier
            .background(if (selected) FeedPurple else Color(0xFF1F1F1F), RoundedCornerShape(16.dp))
            .clickable { onClick() }
            .padding(horizontal = 14.dp, vertical = 7.dp),
    ) {
        Text(
            label,
            color = Color.White,
            style = MaterialTheme.typography.bodyMedium,
            fontWeight = if (selected) FontWeight.Bold else FontWeight.Normal,
            maxLines = 1,
        )
    }
}

private val visibilityOptions = listOf(
    "publik" to "Publik", "teman" to "Teman", "pribadi" to "Pribadi",
)

@Composable
private fun VisibilityMenu(selected: String, onSelect: (String) -> Unit) {
    var open by remember { mutableStateOf(false) }
    val label = visibilityOptions.firstOrNull { it.first == selected }?.second ?: "Publik"
    Box {
        TextButton(onClick = { open = true }) {
            Text(label, color = Color(0xFFB3B3B3))
        }
        DropdownMenu(expanded = open, onDismissRequest = { open = false }) {
            visibilityOptions.forEach { (value, name) ->
                DropdownMenuItem(
                    text = { Text(name) },
                    onClick = {
                        onSelect(value)
                        open = false
                    },
                )
            }
        }
    }
}

@Composable
private fun CategoryMenu(selected: String, onSelect: (String) -> Unit) {
    var open by remember { mutableStateOf(false) }
    val label = feedCategories.firstOrNull { it.first == selected }?.second ?: "Umum"
    Box {
        TextButton(onClick = { open = true }) {
            Text("Kategori: $label", color = Color(0xFFB3B3B3))
        }
        DropdownMenu(expanded = open, onDismissRequest = { open = false }) {
            feedCategories.filter { it.first != null }.forEach { (value, name) ->
                DropdownMenuItem(
                    text = { Text(name) },
                    onClick = {
                        onSelect(value!!)
                        open = false
                    },
                )
            }
        }
    }
}

@Composable
fun FeedPostCard(
    post: FeedPost,
    myName: String,
    onLike: () -> Unit,
    onBookmark: () -> Unit,
    onRepost: () -> Unit,
    onComments: () -> Unit,
    onCommentLike: (FeedComment) -> Unit,
    onInlineComment: (String) -> Unit,
    onDelete: () -> Unit,
    onHide: () -> Unit,
    onResolveUrl: suspend (String) -> String?,
    onOpenTitle: (String) -> Unit,
    onImageClick: (String) -> Unit,
) {
    var menuOpen by remember { mutableStateOf(false) }
    var inlineText by remember(post.id) { mutableStateOf("") }
    Card(
        modifier = Modifier.fillMaxWidth().padding(horizontal = 16.dp),
        colors = CardDefaults.cardColors(containerColor = Color(0xFF1E1B26)),
        shape = RoundedCornerShape(16.dp),
    ) {
        Column(modifier = Modifier.padding(14.dp)) {
            if (post.repostedBy != null) {
                Row(
                    modifier = Modifier.padding(bottom = 8.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Icon(
                        painterResource(R.drawable.ic_repeat),
                        contentDescription = null,
                        tint = FeedPurple,
                        modifier = Modifier.size(15.dp),
                    )
                    Spacer(Modifier.width(6.dp))
                    Text(
                        "${post.repostedBy.displayName} memposting ulang",
                        style = MaterialTheme.typography.bodySmall,
                        color = Color(0xFF9E9EA7),
                        fontWeight = FontWeight.Bold,
                    )
                }
            }
            Row(verticalAlignment = Alignment.CenterVertically) {
                FeedAvatar(post.author, 44)
                Spacer(Modifier.width(10.dp))
                Column(modifier = Modifier.weight(1f)) {
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        Text(
                            post.author.displayName,
                            style = MaterialTheme.typography.titleSmall,
                            fontWeight = FontWeight.Bold,
                            color = Color.White,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                        )
                        if (post.author.isPremium) {
                            Spacer(Modifier.width(5.dp))
                            VerifiedBadge()
                        }
                    }
                    Text(
                        listOfNotNull(
                            relativeTimeId(post.createdAtMs),
                            post.groupName?.let { "Grup $it" } ?: "Teman",
                            when (post.visibility) {
                                "teman" -> "Hanya teman"
                                "pribadi" -> "Pribadi"
                                else -> "Publik"
                            },
                        ).joinToString(" · "),
                        style = MaterialTheme.typography.bodySmall,
                        color = Color(0xFF9E9EA7),
                    )
                }
                Box {
                    IconButton(onClick = { menuOpen = true }) {
                        Icon(painterResource(R.drawable.ic_more_vert), contentDescription = "Menu", tint = Color(0xFFB3B3B3))
                    }
                    DropdownMenu(expanded = menuOpen, onDismissRequest = { menuOpen = false }) {
                        if (post.mine) {
                            DropdownMenuItem(
                                text = { Text("Hapus postingan") },
                                onClick = { menuOpen = false; onDelete() },
                            )
                        }
                        DropdownMenuItem(
                            text = { Text("Sembunyikan") },
                            onClick = { menuOpen = false; onHide() },
                        )
                    }
                }
            }
            if (!post.body.isNullOrBlank()) {
                LinkText(
                    post.body,
                    color = Color.White,
                    style = MaterialTheme.typography.bodyLarge,
                    modifier = Modifier.padding(top = 10.dp),
                )
            }
            post.body?.let { b -> urlRegex.find(b)?.value }?.let { firstUrl ->
                SocialEmbed(firstUrl, onResolveUrl)
            }
            if (post.imageUrl != null) {
                if (post.isVideo) {
                    FeedVideoPlayer(
                        post.imageUrl,
                        Modifier.fillMaxWidth().padding(top = 10.dp)
                            .aspectRatio(16f / 10f).clip(RoundedCornerShape(12.dp)),
                    )
                } else {
                    AsyncImage(
                        model = post.imageUrl,
                        contentDescription = "Gambar postingan",
                        contentScale = ContentScale.Crop,
                        modifier = Modifier.fillMaxWidth().padding(top = 10.dp)
                            .aspectRatio(16f / 10f).clip(RoundedCornerShape(12.dp))
                            .clickable { onImageClick(post.imageUrl) },
                    )
                }
            }
            if (post.titleId != null) {
                // Postingan hasil "Bagikan ke Post": judulnya bisa
                // diklik menuju halaman detail untuk langsung ditonton.
                val sharedTitle = post.body
                    ?.takeIf { it.startsWith("🎬 Lagi nonton ") }
                    ?.removePrefix("🎬 Lagi nonton ")
                    ?.substringBefore(" (")
                    ?.substringBefore(" — ")
                    ?.trim()
                Row(
                    modifier = Modifier.padding(top = 10.dp)
                        .clip(RoundedCornerShape(10.dp))
                        .background(FeedPurple.copy(alpha = 0.16f))
                        .clickable { onOpenTitle(post.titleId) }
                        .padding(horizontal = 12.dp, vertical = 9.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Icon(
                        painterResource(R.drawable.ic_play),
                        contentDescription = null,
                        tint = FeedPurple,
                        modifier = Modifier.size(15.dp),
                    )
                    Spacer(Modifier.width(8.dp))
                    Text(
                        if (sharedTitle.isNullOrBlank()) {
                            "Buka judul ini — langsung tonton"
                        } else {
                            "$sharedTitle — tonton sekarang"
                        },
                        style = MaterialTheme.typography.bodyMedium,
                        fontWeight = FontWeight.SemiBold,
                        color = Color.White,
                        maxLines = 1,
                    )
                }
            }
            Row(
                modifier = Modifier.fillMaxWidth().padding(top = 12.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                ActionItem(
                    icon = R.drawable.ic_heart,
                    count = post.likeCount,
                    tint = if (post.liked) Color(0xFFE50914) else Color(0xFFB3B3B3),
                    onClick = onLike,
                )
                Spacer(Modifier.width(22.dp))
                ActionItem(
                    icon = R.drawable.ic_chat,
                    count = post.commentCount,
                    tint = Color(0xFFB3B3B3),
                    onClick = onComments,
                )
                Spacer(Modifier.width(22.dp))
                ActionItem(
                    icon = R.drawable.ic_repeat,
                    count = post.repostCount,
                    tint = if (post.reposted) FeedPurple else Color(0xFFB3B3B3),
                    onClick = onRepost,
                )
                Spacer(Modifier.weight(1f))
                IconButton(onClick = onBookmark) {
                    Icon(
                        painterResource(R.drawable.ic_bookmark),
                        contentDescription = "Simpan",
                        tint = if (post.bookmarked) FeedPurple else Color(0xFFB3B3B3),
                    )
                }
            }
            // Komentar tampil sebaris di dalam kartu (gaya referensi).
            if (post.commentsPreview.isNotEmpty()) {
                Box(
                    modifier = Modifier.fillMaxWidth().padding(top = 10.dp)
                        .background(Color(0xFF17141F), RoundedCornerShape(12.dp))
                        .padding(horizontal = 10.dp, vertical = 4.dp),
                ) {
                    Column {
                        post.commentsPreview.forEach { c ->
                            InlineCommentRow(
                                comment = c,
                                onLike = { onCommentLike(c) },
                                onReply = onComments,
                            )
                        }
                        if (post.commentCount > post.commentsPreview.size) {
                            Text(
                                "Lihat semua ${post.commentCount} komentar",
                                color = FeedPurple,
                                style = MaterialTheme.typography.bodySmall,
                                fontWeight = FontWeight.Bold,
                                modifier = Modifier.clickable { onComments() }
                                    .padding(vertical = 8.dp),
                            )
                        }
                    }
                }
            }
            // Input balasan sebaris di kaki kartu.
            Row(
                modifier = Modifier.fillMaxWidth().padding(top = 10.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Box(
                    modifier = Modifier.size(32.dp).background(FeedPurple, CircleShape),
                    contentAlignment = Alignment.Center,
                ) {
                    Text(myName.take(1).uppercase(), color = Color.White, fontWeight = FontWeight.Bold, fontSize = 13.sp)
                }
                Spacer(Modifier.width(8.dp))
                TextField(
                    value = inlineText,
                    onValueChange = { inlineText = it },
                    placeholder = { Text("Tulis balasan...", color = Color(0xFF9E9EA7)) },
                    singleLine = true,
                    modifier = Modifier.weight(1f),
                    shape = RoundedCornerShape(20.dp),
                    colors = TextFieldDefaults.colors(
                        focusedContainerColor = Color(0xFF17141F),
                        unfocusedContainerColor = Color(0xFF17141F),
                        disabledContainerColor = Color(0xFF17141F),
                        focusedIndicatorColor = Color.Transparent,
                        unfocusedIndicatorColor = Color.Transparent,
                        focusedTextColor = Color.White,
                        unfocusedTextColor = Color.White,
                        cursorColor = FeedPurple,
                    ),
                )
                IconButton(
                    enabled = inlineText.isNotBlank(),
                    onClick = {
                        onInlineComment(inlineText.trim())
                        inlineText = ""
                    },
                ) {
                    Icon(painterResource(R.drawable.ic_send), contentDescription = "Kirim", tint = FeedPurple)
                }
            }
        }
    }
}

/** Satu komentar di pratinjau sebaris kartu feed. */
@Composable
private fun InlineCommentRow(
    comment: FeedComment,
    onLike: () -> Unit,
    onReply: () -> Unit,
) {
    Row(
        modifier = Modifier
            .padding(start = if (comment.parentId != null) 26.dp else 0.dp, top = 6.dp, bottom = 6.dp),
        verticalAlignment = Alignment.Top,
    ) {
        FeedAvatar(comment.author, 30)
        Spacer(Modifier.width(8.dp))
        Column(modifier = Modifier.weight(1f)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Text(
                    comment.author.displayName,
                    style = MaterialTheme.typography.bodyMedium,
                    fontWeight = FontWeight.Bold,
                    color = Color.White,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
                if (comment.author.isPremium) {
                    Spacer(Modifier.width(4.dp))
                    VerifiedBadge()
                }
                Spacer(Modifier.width(6.dp))
                Text(
                    relativeTimeId(comment.createdAtMs),
                    style = MaterialTheme.typography.bodySmall,
                    color = Color(0xFF9E9EA7),
                )
            }
            LinkText(comment.body, color = Color.White, style = MaterialTheme.typography.bodyMedium)
            Row(verticalAlignment = Alignment.CenterVertically) {
                Row(
                    modifier = Modifier.clickable { onLike() }.padding(vertical = 3.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Icon(
                        painterResource(R.drawable.ic_thumb_up),
                        contentDescription = "Suka",
                        tint = if (comment.liked) FeedPurple else Color(0xFF9E9EA7),
                        modifier = Modifier.size(14.dp),
                    )
                    Spacer(Modifier.width(4.dp))
                    Text(comment.likeCount.toString(), style = MaterialTheme.typography.bodySmall, color = Color(0xFF9E9EA7))
                }
                Spacer(Modifier.width(16.dp))
                Text(
                    "Balas",
                    style = MaterialTheme.typography.bodySmall,
                    color = Color(0xFF9E9EA7),
                    fontWeight = FontWeight.Bold,
                    modifier = Modifier.clickable { onReply() }.padding(vertical = 3.dp),
                )
            }
        }
    }
}

@Composable
private fun ActionItem(icon: Int, count: Int, tint: Color, onClick: () -> Unit) {
    Row(
        modifier = Modifier.clickable { onClick() }.padding(vertical = 4.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Icon(painterResource(icon), contentDescription = null, tint = tint, modifier = Modifier.size(21.dp))
        Spacer(Modifier.width(5.dp))
        Text(count.toString(), color = tint, style = MaterialTheme.typography.bodyMedium)
    }
}


// ---------- Komentar berjenjang ----------

private data class CommentRow(
    val comment: FeedComment,
    val depth: Int,
    val descendantCount: Int,
    val hasChildren: Boolean,
)

private fun buildCommentRows(comments: List<FeedComment>, collapsed: Set<String>): List<CommentRow> {
    val children = comments.groupBy { it.parentId }
    val out = mutableListOf<CommentRow>()
    fun countDesc(id: String): Int = children[id].orEmpty().sumOf { 1 + countDesc(it.id) }
    fun walk(list: List<FeedComment>, depth: Int) {
        for (c in list) {
            val kids = children[c.id].orEmpty()
            out += CommentRow(c, depth, countDesc(c.id), kids.isNotEmpty())
            if (c.id !in collapsed) walk(kids, min(depth + 1, 3))
        }
    }
    walk(children[null].orEmpty(), 0)
    return out
}

@Composable
fun CommentsScreen(
    repo: CatalogRepository,
    postId: String,
    onBack: () -> Unit,
    onOpenTitle: (String) -> Unit,
) {
    val context = LocalContext.current
    val scope = rememberCoroutineScope()
    val auth = remember { AuthStore(context) }

    var post by remember { mutableStateOf<FeedPost?>(null) }
    var comments by remember { mutableStateOf<List<FeedComment>>(emptyList()) }
    var collapsed by remember { mutableStateOf<Set<String>>(emptySet()) }
    var loading by remember { mutableStateOf(true) }
    var error by remember { mutableStateOf<String?>(null) }
    var input by remember { mutableStateOf("") }
    var replyTo by remember { mutableStateOf<FeedComment?>(null) }
    var sending by remember { mutableStateOf(false) }
    var viewerUrl by remember { mutableStateOf<String?>(null) }

    suspend fun reload() {
        try {
            post = repo.feedPost(postId)
            comments = repo.feedComments(postId)
            error = null
        } catch (e: Exception) {
            error = e.message
        } finally {
            loading = false
        }
    }

    LaunchedEffect(postId) { reload() }

    fun toggleCommentLike(c: FeedComment) {
        val before = c
        comments = comments.map {
            if (it.id == c.id) it.copy(liked = !c.liked, likeCount = c.likeCount + if (c.liked) -1 else 1) else it
        }
        scope.launch {
            try {
                val (liked, count) = repo.feedCommentLike(c.id)
                comments = comments.map { if (it.id == c.id) before.copy(liked = liked, likeCount = count) else it }
            } catch (e: Exception) {
                comments = comments.map { if (it.id == c.id) before else it }
            }
        }
    }

    Column(modifier = Modifier.fillMaxSize()) {
        Row(
            modifier = Modifier.fillMaxWidth().padding(horizontal = 8.dp, vertical = 6.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            IconButton(onClick = onBack) {
                Icon(painterResource(R.drawable.ic_arrow_back), contentDescription = "Kembali", tint = Color.White)
            }
            Text("Komentar", style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.Bold)
        }
        if (viewerUrl != null) {
            FullImageViewer(url = viewerUrl!!, onDismiss = { viewerUrl = null })
        }
        if (loading) {
            Box(Modifier.fillMaxWidth().padding(32.dp), contentAlignment = Alignment.Center) {
                CircularProgressIndicator(color = FeedPurple)
            }
        }
        if (error != null) {
            Text(
                "Gagal memuat: $error",
                color = MaterialTheme.colorScheme.error,
                modifier = Modifier.padding(horizontal = 16.dp),
            )
        }
        val rows = remember(comments, collapsed) { buildCommentRows(comments, collapsed) }
        LazyColumn(
            modifier = Modifier.weight(1f),
            verticalArrangement = Arrangement.spacedBy(4.dp),
        ) {
            val p = post
            if (p != null) {
                item {
                    Column(modifier = Modifier.padding(horizontal = 16.dp, vertical = 6.dp)) {
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            FeedAvatar(p.author, 36)
                            Spacer(Modifier.width(8.dp))
                            Text(p.author.displayName, fontWeight = FontWeight.Bold, color = Color.White)
                            if (p.author.isPremium) {
                                Spacer(Modifier.width(5.dp))
                                VerifiedBadge()
                            }
                            Spacer(Modifier.width(8.dp))
                            Text(relativeTimeId(p.createdAtMs), color = Color(0xFF9E9EA7), style = MaterialTheme.typography.bodySmall)
                        }
                        if (!p.body.isNullOrBlank()) {
                            LinkText(
                                p.body,
                                color = Color.White,
                                style = MaterialTheme.typography.bodyLarge,
                                modifier = Modifier.padding(top = 8.dp),
                            )
                        }
                        p.body?.let { b -> urlRegex.find(b)?.value }?.let { firstUrl ->
                            SocialEmbed(firstUrl) { repo.resolveUrl(it) }
                        }
                        if (p.imageUrl != null) {
                            if (p.isVideo) {
                                FeedVideoPlayer(
                                    p.imageUrl,
                                    Modifier.fillMaxWidth().padding(top = 8.dp)
                                        .aspectRatio(16f / 10f).clip(RoundedCornerShape(12.dp)),
                                )
                            } else {
                                AsyncImage(
                                    model = p.imageUrl,
                                    contentDescription = "Gambar postingan",
                                    contentScale = ContentScale.Crop,
                                    modifier = Modifier.fillMaxWidth().padding(top = 8.dp)
                                        .aspectRatio(16f / 10f).clip(RoundedCornerShape(12.dp))
                                        .clickable { viewerUrl = p.imageUrl },
                                )
                            }
                        }
                        if (p.titleId != null) {
                            Row(
                                modifier = Modifier.padding(top = 10.dp)
                                    .clip(RoundedCornerShape(10.dp))
                                    .background(FeedPurple.copy(alpha = 0.16f))
                                    .clickable { onOpenTitle(p.titleId) }
                                    .padding(horizontal = 12.dp, vertical = 9.dp),
                                verticalAlignment = Alignment.CenterVertically,
                            ) {
                                Icon(
                                    painterResource(R.drawable.ic_play),
                                    contentDescription = null,
                                    tint = FeedPurple,
                                    modifier = Modifier.size(15.dp),
                                )
                                Spacer(Modifier.width(8.dp))
                                Text(
                                    "Buka judul ini — langsung tonton",
                                    style = MaterialTheme.typography.bodyMedium,
                                    fontWeight = FontWeight.SemiBold,
                                    color = Color.White,
                                )
                            }
                        }
                    }
                }
                item {
                    Text(
                        "${p.commentCount} komentar",
                        style = MaterialTheme.typography.titleSmall,
                        fontWeight = FontWeight.Bold,
                        modifier = Modifier.padding(horizontal = 16.dp, vertical = 8.dp),
                    )
                }
            }
            if (!loading && error == null && comments.isEmpty()) {
                item {
                    Text(
                        "Belum ada komentar. Jadilah yang pertama berkomentar.",
                        color = Color(0xFFB3B3B3),
                        modifier = Modifier.padding(horizontal = 16.dp, vertical = 8.dp),
                    )
                }
            }
            items(rows, key = { it.comment.id }) { row ->
                val c = row.comment
                Column(
                    modifier = Modifier
                        .padding(start = (16 + row.depth * 22).dp, end = 16.dp, top = 6.dp, bottom = 6.dp),
                ) {
                    Row(verticalAlignment = Alignment.Top) {
                        FeedAvatar(c.author, 32)
                        Spacer(Modifier.width(8.dp))
                        Column(modifier = Modifier.weight(1f)) {
                            Row(verticalAlignment = Alignment.CenterVertically) {
                                Text(
                                    c.author.displayName,
                                    style = MaterialTheme.typography.bodyMedium,
                                    fontWeight = FontWeight.Bold,
                                    color = Color.White,
                                )
                                if (c.author.isPremium) {
                                    Spacer(Modifier.width(4.dp))
                                    VerifiedBadge()
                                }
                                Spacer(Modifier.width(6.dp))
                                Text(
                                    relativeTimeId(c.createdAtMs),
                                    style = MaterialTheme.typography.bodySmall,
                                    color = Color(0xFF9E9EA7),
                                )
                            }
                            LinkText(c.body, color = Color.White, style = MaterialTheme.typography.bodyLarge)
                            Row(verticalAlignment = Alignment.CenterVertically) {
                                Row(
                                    modifier = Modifier.clickable { toggleCommentLike(c) }.padding(vertical = 4.dp),
                                    verticalAlignment = Alignment.CenterVertically,
                                ) {
                                    Icon(
                                        painterResource(R.drawable.ic_thumb_up),
                                        contentDescription = "Suka",
                                        tint = if (c.liked) FeedPurple else Color(0xFF9E9EA7),
                                        modifier = Modifier.size(15.dp),
                                    )
                                    Spacer(Modifier.width(4.dp))
                                    Text(
                                        c.likeCount.toString(),
                                        style = MaterialTheme.typography.bodySmall,
                                        color = Color(0xFF9E9EA7),
                                    )
                                }
                                Spacer(Modifier.width(16.dp))
                                Text(
                                    "Balas",
                                    style = MaterialTheme.typography.bodySmall,
                                    color = Color(0xFF9E9EA7),
                                    fontWeight = FontWeight.Bold,
                                    modifier = Modifier.clickable { replyTo = c }.padding(vertical = 4.dp),
                                )
                                if (c.mine) {
                                    Spacer(Modifier.width(16.dp))
                                    Text(
                                        "Hapus",
                                        style = MaterialTheme.typography.bodySmall,
                                        color = Color(0xFF9E9EA7),
                                        modifier = Modifier.clickable {
                                            scope.launch {
                                                try {
                                                    repo.feedCommentDelete(c.id)
                                                    reload()
                                                } catch (e: Exception) {
                                                    error = e.message
                                                }
                                            }
                                        }.padding(vertical = 4.dp),
                                    )
                                }
                            }
                            if (row.hasChildren) {
                                Text(
                                    if (c.id in collapsed) {
                                        "Lihat ${row.descendantCount} balasan"
                                    } else {
                                        "Sembunyikan balasan"
                                    },
                                    style = MaterialTheme.typography.bodySmall,
                                    color = FeedPurple,
                                    fontWeight = FontWeight.Bold,
                                    modifier = Modifier.clickable {
                                        collapsed = if (c.id in collapsed) collapsed - c.id else collapsed + c.id
                                    }.padding(vertical = 2.dp),
                                )
                            }
                        }
                    }
                }
            }
        }
        // Bilah input komentar / balasan.
        Column(modifier = Modifier.background(Color(0xFF1F1F1F))) {
            if (replyTo != null) {
                Row(
                    modifier = Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 6.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Text(
                        "Membalas ${replyTo!!.author.displayName}",
                        style = MaterialTheme.typography.bodySmall,
                        color = FeedPurple,
                        modifier = Modifier.weight(1f),
                    )
                    Text(
                        "✕",
                        color = Color.White,
                        modifier = Modifier.clickable { replyTo = null }.padding(4.dp),
                    )
                }
            }
            Row(
                modifier = Modifier.fillMaxWidth().padding(horizontal = 12.dp, vertical = 8.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Box(
                    modifier = Modifier.size(34.dp).background(FeedPurple, CircleShape),
                    contentAlignment = Alignment.Center,
                ) {
                    Text((auth.displayName ?: "S").take(1).uppercase(), color = Color.White, fontWeight = FontWeight.Bold, fontSize = 14.sp)
                }
                Spacer(Modifier.width(8.dp))
                OutlinedTextField(
                    value = input,
                    onValueChange = { input = it },
                    placeholder = { Text(if (replyTo != null) "Tulis balasan…" else "Tulis komentar…") },
                    maxLines = 4,
                    modifier = Modifier.weight(1f),
                )
                Spacer(Modifier.width(6.dp))
                IconButton(
                    enabled = !sending && input.isNotBlank(),
                    onClick = {
                        sending = true
                        val body = input.trim()
                        val parent = replyTo?.id
                        scope.launch {
                            try {
                                repo.feedCommentCreate(postId, body, parent)
                                input = ""
                                replyTo = null
                                reload()
                            } catch (e: Exception) {
                                error = e.message
                            } finally {
                                sending = false
                            }
                        }
                    },
                ) {
                    Icon(painterResource(R.drawable.ic_send), contentDescription = "Kirim", tint = FeedPurple)
                }
            }
        }
    }
}

// ---------- Notifikasi sosial ----------

@Composable
fun FeedNotificationsScreen(
    repo: CatalogRepository,
    onBack: () -> Unit,
    onOpenPost: (String) -> Unit,
) {
    var items by remember { mutableStateOf<List<FeedNotification>>(emptyList()) }
    var loading by remember { mutableStateOf(true) }
    var error by remember { mutableStateOf<String?>(null) }

    LaunchedEffect(Unit) {
        try {
            items = repo.feedNotifications().first
            repo.feedNotificationsRead()
        } catch (e: Exception) {
            error = e.message
        } finally {
            loading = false
        }
    }

    Column(modifier = Modifier.fillMaxSize()) {
        Row(
            modifier = Modifier.fillMaxWidth().padding(horizontal = 8.dp, vertical = 6.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            IconButton(onClick = onBack) {
                Icon(painterResource(R.drawable.ic_arrow_back), contentDescription = "Kembali", tint = Color.White)
            }
            Text("Notifikasi", style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.Bold)
        }
        if (loading) {
            Box(Modifier.fillMaxWidth().padding(32.dp), contentAlignment = Alignment.Center) {
                CircularProgressIndicator(color = FeedPurple)
            }
        }
        if (error != null) {
            Text("Gagal memuat: $error", color = MaterialTheme.colorScheme.error, modifier = Modifier.padding(horizontal = 16.dp))
        }
        if (!loading && error == null && items.isEmpty()) {
            Text(
                "Belum ada notifikasi. Pengumuman dari admin serta like, komentar, dan balasan pada postinganmu akan muncul di sini.",
                color = Color(0xFFB3B3B3),
                modifier = Modifier.padding(horizontal = 16.dp, vertical = 8.dp),
            )
        }
        LazyColumn(verticalArrangement = Arrangement.spacedBy(4.dp)) {
            items(items, key = { it.id }) { n ->
                Row(
                    modifier = Modifier.fillMaxWidth()
                        .background(if (n.read) Color.Transparent else Color(0xFF241B3A))
                        .clickable { n.refPostId?.let(onOpenPost) }
                        .padding(horizontal = 16.dp, vertical = 10.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    FeedAvatar(
                        FeedAuthor("", n.title, n.actorAvatarUrl, false),
                        40,
                    )
                    Spacer(Modifier.width(10.dp))
                    Column(modifier = Modifier.weight(1f)) {
                        if (n.kind == "broadcast") {
                            Text(
                                "📣 PENGUMUMAN ADMIN",
                                color = FeedPurple,
                                style = MaterialTheme.typography.labelSmall,
                                fontWeight = FontWeight.Bold,
                            )
                        }
                        Text(
                            "${n.title} ${n.body}",
                            color = Color.White,
                            style = MaterialTheme.typography.bodyMedium,
                            maxLines = 2,
                            overflow = TextOverflow.Ellipsis,
                        )
                        Text(
                            relativeTimeId(n.createdAtMs),
                            color = Color(0xFF9E9EA7),
                            style = MaterialTheme.typography.bodySmall,
                        )
                    }
                }
            }
        }
    }
}
