package com.subsenpai.ui

import android.content.Intent
import android.graphics.Bitmap
import android.graphics.BitmapFactory
import android.net.Uri
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.PickVisualMediaRequest
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
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
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.derivedStateOf
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.runtime.snapshotFlow
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
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
import com.subsenpai.data.GroupItem
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

private val feedCategories = listOf(
    null to "Semua", "umum" to "Umum", "anime" to "Anime", "donghua" to "Donghua",
    "film" to "Film", "manga" to "Manga", "rekomendasi" to "Rekomendasi",
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
    var imageUri by remember { mutableStateOf<Uri?>(null) }
    var imageBytes by remember { mutableStateOf<ByteArray?>(null) }
    var posting by remember { mutableStateOf(false) }
    var postError by remember { mutableStateOf<String?>(null) }

    val picker = rememberLauncherForActivityResult(
        ActivityResultContracts.PickVisualMedia()
    ) { uri ->
        if (uri != null) {
            imageUri = uri
            scope.launch { imageBytes = loadCompressedImage(context, uri) }
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

    fun updatePost(updated: FeedPost) {
        posts = posts.map { if (it.id == updated.id) updated else it }
    }

    fun toggleLike(post: FeedPost) {
        val before = post
        updatePost(post.copy(liked = !post.liked, likeCount = post.likeCount + if (post.liked) -1 else 1))
        scope.launch {
            try {
                val (liked, count) = repo.feedLike(post.id)
                updatePost(before.copy(liked = liked, likeCount = count))
            } catch (e: Exception) {
                updatePost(before)
            }
        }
    }

    fun toggleBookmark(post: FeedPost) {
        val before = post
        updatePost(post.copy(bookmarked = !post.bookmarked))
        scope.launch {
            try {
                val saved = repo.feedBookmark(post.id)
                updatePost(before.copy(bookmarked = saved))
            } catch (e: Exception) {
                updatePost(before)
            }
        }
    }

    fun sharePost(post: FeedPost) {
        val send = Intent(Intent.ACTION_SEND).apply {
            type = "text/plain"
            putExtra(Intent.EXTRA_TEXT, (post.body ?: "") + "\n\n— dibagikan dari SubSenpai")
        }
        context.startActivity(Intent.createChooser(send, "Bagikan postingan"))
        scope.launch {
            try {
                val count = repo.feedShare(post.id)
                updatePost(post.copy(shareCount = count))
            } catch (e: Exception) {
                // Penghitung bagikan boleh gagal diam-diam.
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

    LazyColumn(
        state = listState,
        modifier = Modifier.fillMaxSize(),
        verticalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        item {
            Row(
                modifier = Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 10.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text("Chat", style = MaterialTheme.typography.headlineSmall, fontWeight = FontWeight.Bold)
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
                        OutlinedTextField(
                            value = draft,
                            onValueChange = { draft = it },
                            placeholder = { Text("Apa yang kamu pikirkan?") },
                            minLines = 1,
                            maxLines = 5,
                            modifier = Modifier.weight(1f),
                        )
                    }
                    if (imageUri != null) {
                        Spacer(Modifier.height(10.dp))
                        Box {
                            AsyncImage(
                                model = imageUri,
                                contentDescription = "Lampiran",
                                contentScale = ContentScale.Crop,
                                modifier = Modifier.fillMaxWidth().aspectRatio(16f / 9f)
                                    .clip(RoundedCornerShape(10.dp)),
                            )
                            Box(
                                modifier = Modifier.align(Alignment.TopEnd).padding(6.dp)
                                    .background(Color(0xB3000000), CircleShape)
                                    .clickable { imageUri = null; imageBytes = null }
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
                            picker.launch(PickVisualMediaRequest(ActivityResultContracts.PickVisualMedia.ImageOnly))
                        }) {
                            Icon(painterResource(R.drawable.ic_image), contentDescription = "Tambah gambar", tint = FeedPurple)
                        }
                        CategoryMenu(selected = draftCategory, onSelect = { draftCategory = it })
                        Spacer(Modifier.weight(1f))
                        Button(
                            enabled = !posting && (draft.isNotBlank() || imageBytes != null) &&
                                (tab != "grup" || groupId != null),
                            onClick = {
                                posting = true
                                postError = null
                                scope.launch {
                                    try {
                                        val url = imageBytes?.let {
                                            repo.feedUpload(it, "feed-${System.currentTimeMillis()}.jpg")
                                        }
                                        val created = repo.feedCreate(
                                            draft.trim(), url, draftCategory,
                                            if (tab == "grup") groupId else null,
                                        )
                                        draft = ""
                                        imageUri = null
                                        imageBytes = null
                                        if (tab == "teman") {
                                            posts = listOf(created) + posts
                                        } else {
                                            loadFirst()
                                        }
                                    } catch (e: Exception) {
                                        postError = e.message
                                    } finally {
                                        posting = false
                                    }
                                }
                            },
                            colors = ButtonDefaults.buttonColors(containerColor = FeedPurple),
                            shape = RoundedCornerShape(20.dp),
                        ) {
                            if (posting) {
                                CircularProgressIndicator(color = Color.White, modifier = Modifier.size(16.dp), strokeWidth = 2.dp)
                                Spacer(Modifier.width(6.dp))
                            }
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
        item {
            // Tab Teman / Grup (pil ungu ala referensi).
            Row(
                modifier = Modifier.fillMaxWidth().padding(horizontal = 16.dp),
                horizontalArrangement = Arrangement.spacedBy(10.dp),
            ) {
                TabPill("Teman", tab == "teman", Modifier.weight(1f)) { tab = "teman" }
                TabPill("Grup", tab == "grup", Modifier.weight(1f)) { tab = "grup" }
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
                onLike = { toggleLike(post) },
                onBookmark = { toggleBookmark(post) },
                onShare = { sharePost(post) },
                onComments = { onOpenComments(post.id) },
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
}

@Composable
private fun TabPill(label: String, selected: Boolean, modifier: Modifier = Modifier, onClick: () -> Unit) {
    Box(
        modifier = modifier.height(40.dp)
            .background(if (selected) FeedPurple else Color(0xFF1F1F1F), RoundedCornerShape(20.dp))
            .clickable { onClick() },
        contentAlignment = Alignment.Center,
    ) {
        Text(
            label,
            color = Color.White,
            fontWeight = if (selected) FontWeight.Bold else FontWeight.Normal,
        )
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
    onLike: () -> Unit,
    onBookmark: () -> Unit,
    onShare: () -> Unit,
    onComments: () -> Unit,
    onDelete: () -> Unit,
    onHide: () -> Unit,
) {
    var menuOpen by remember { mutableStateOf(false) }
    Card(
        modifier = Modifier.fillMaxWidth().padding(horizontal = 16.dp),
        colors = CardDefaults.cardColors(containerColor = Color(0xFF1A1A1E)),
        shape = RoundedCornerShape(16.dp),
    ) {
        Column(modifier = Modifier.padding(14.dp)) {
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
                            post.groupName?.let { "Grup $it" }
                                ?: feedCategories.firstOrNull { it.first == post.category }?.second,
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
                Text(
                    post.body,
                    style = MaterialTheme.typography.bodyLarge,
                    color = Color.White,
                    modifier = Modifier.padding(top = 10.dp),
                )
            }
            if (post.imageUrl != null) {
                AsyncImage(
                    model = post.imageUrl,
                    contentDescription = "Gambar postingan",
                    contentScale = ContentScale.Crop,
                    modifier = Modifier.fillMaxWidth().padding(top = 10.dp)
                        .aspectRatio(16f / 10f).clip(RoundedCornerShape(12.dp)),
                )
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
                    icon = R.drawable.ic_share,
                    count = post.shareCount,
                    tint = Color(0xFFB3B3B3),
                    onClick = onShare,
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
                            Text(p.body, color = Color.White, modifier = Modifier.padding(top = 8.dp))
                        }
                        if (p.imageUrl != null) {
                            AsyncImage(
                                model = p.imageUrl,
                                contentDescription = "Gambar postingan",
                                contentScale = ContentScale.Crop,
                                modifier = Modifier.fillMaxWidth().padding(top = 8.dp)
                                    .aspectRatio(16f / 10f).clip(RoundedCornerShape(12.dp)),
                            )
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
                            Text(c.body, color = Color.White, style = MaterialTheme.typography.bodyLarge)
                            Row(verticalAlignment = Alignment.CenterVertically) {
                                Row(
                                    modifier = Modifier.clickable { toggleCommentLike(c) }.padding(vertical = 4.dp),
                                    verticalAlignment = Alignment.CenterVertically,
                                ) {
                                    Icon(
                                        painterResource(R.drawable.ic_heart),
                                        contentDescription = "Suka",
                                        tint = if (c.liked) Color(0xFFE50914) else Color(0xFF9E9EA7),
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
                "Belum ada notifikasi. Like, komentar, dan balasan pada postinganmu akan muncul di sini.",
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
