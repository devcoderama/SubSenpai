package com.subsenpai.data

import org.json.JSONObject
import java.time.Instant

/** Model feed komunitas (postingan sosial) — pasangan httpapi/feed.go. */

data class FeedAuthor(
    val id: String,
    val displayName: String,
    val avatarUrl: String?,
    val isPremium: Boolean,
) {
    companion object {
        fun fromJson(o: JSONObject) = FeedAuthor(
            id = o.getString("id"),
            displayName = o.optString("display_name", "Pengguna"),
            avatarUrl = o.optString("avatar_url").ifBlank { null },
            isPremium = o.optBoolean("is_premium"),
        )
    }
}

data class FeedPost(
    val id: String, // id baris feed (baris repost bila ini repost)
    val postId: String, // id postingan konten (asli) — target like/komentar/repost
    val author: FeedAuthor,
    val repostedBy: FeedAuthor?,
    val body: String?,
    val imageUrl: String?,
    val category: String,
    val visibility: String,
    val groupId: String?,
    val groupName: String?,
    val titleId: String?,
    val likeCount: Int,
    val commentCount: Int,
    val repostCount: Int,
    val shareCount: Int,
    val liked: Boolean,
    val bookmarked: Boolean,
    val reposted: Boolean,
    val mine: Boolean,
    val createdAtMs: Long,
    val commentsPreview: List<FeedComment>,
) {
    companion object {
        fun fromJson(o: JSONObject) = FeedPost(
            id = o.getString("id"),
            postId = o.optString("post_id").ifBlank { o.getString("id") },
            author = FeedAuthor.fromJson(o.getJSONObject("author")),
            repostedBy = if (o.isNull("reposted_by")) null else FeedAuthor.fromJson(o.getJSONObject("reposted_by")),
            body = o.optString("body").ifBlank { null },
            imageUrl = o.optString("image_url").ifBlank { null },
            category = o.optString("category", "umum"),
            visibility = o.optString("visibility", "publik"),
            groupId = o.optString("group_id").ifBlank { null },
            groupName = o.optString("group_name").ifBlank { null },
            titleId = o.optString("title_id").ifBlank { null },
            likeCount = o.optInt("like_count"),
            commentCount = o.optInt("comment_count"),
            repostCount = o.optInt("repost_count"),
            shareCount = o.optInt("share_count"),
            liked = o.optBoolean("liked"),
            bookmarked = o.optBoolean("bookmarked"),
            reposted = o.optBoolean("reposted"),
            mine = o.optBoolean("mine"),
            createdAtMs = parseIsoMs(o.optString("created_at")),
            commentsPreview = buildList {
                val arr = o.optJSONArray("comments_preview")
                if (arr != null) for (i in 0 until arr.length()) add(FeedComment.fromJson(arr.getJSONObject(i)))
            },
        )
    }
}

data class FeedComment(
    val id: String,
    val postId: String,
    val parentId: String?,
    val author: FeedAuthor,
    val body: String,
    val likeCount: Int,
    val liked: Boolean,
    val mine: Boolean,
    val createdAtMs: Long,
) {
    companion object {
        fun fromJson(o: JSONObject) = FeedComment(
            id = o.getString("id"),
            postId = o.getString("post_id"),
            parentId = o.optString("parent_id").ifBlank { null },
            author = FeedAuthor.fromJson(o.getJSONObject("author")),
            body = o.optString("body"),
            likeCount = o.optInt("like_count"),
            liked = o.optBoolean("liked"),
            mine = o.optBoolean("mine"),
            createdAtMs = parseIsoMs(o.optString("created_at")),
        )
    }
}

data class FeedNotification(
    val id: String,
    val kind: String,
    val title: String,
    val body: String,
    val refPostId: String?,
    val actorAvatarUrl: String?,
    val read: Boolean,
    val createdAtMs: Long,
) {
    companion object {
        fun fromJson(o: JSONObject) = FeedNotification(
            id = o.getString("id"),
            kind = o.optString("kind", "social"),
            title = o.optString("title"),
            body = o.optString("body"),
            refPostId = o.optString("ref_post_id").ifBlank { null },
            actorAvatarUrl = o.optString("actor_avatar_url").ifBlank { null },
            read = !o.isNull("read_at"),
            createdAtMs = parseIsoMs(o.optString("created_at")),
        )
    }
}

/** True bila lampiran postingan adalah video (dari akhiran URL). */
val FeedPost.isVideo: Boolean
    get() = imageUrl?.let {
        it.endsWith(".mp4", true) || it.endsWith(".webm", true)
    } == true

/** Waktu RFC3339 dari backend → epoch milidetik (0 bila tak terbaca). */
fun parseIsoMs(s: String): Long = try {
    Instant.parse(s).toEpochMilli()
} catch (e: Exception) {
    0L
}

/** Label waktu relatif berbahasa Indonesia untuk feed. */
fun relativeTimeId(ms: Long): String {
    if (ms <= 0) return ""
    val diff = System.currentTimeMillis() - ms
    val menit = diff / 60_000
    val jam = menit / 60
    val hari = jam / 24
    return when {
        menit < 1 -> "baru saja"
        menit < 60 -> "$menit menit yang lalu"
        jam < 24 -> "$jam jam yang lalu"
        hari < 7 -> "$hari hari yang lalu"
        else -> {
            val d = java.time.Instant.ofEpochMilli(ms).atZone(java.time.ZoneId.systemDefault())
            "${d.dayOfMonth}/${d.monthValue}/${d.year}"
        }
    }
}
