package com.subsenpai.data

import org.json.JSONObject

data class TitleCard(
    val id: String,
    val slug: String,
    val type: String,
    val title: String,
    val titleAlt: String?,
    val posterUrl: String?,
    val year: Int?,
    val status: String,
    val genres: List<String>,
    val rating: Double?,
    val isAdult: Boolean,
) {
    companion object {
        fun fromJson(o: JSONObject) = TitleCard(
            id = o.getString("id"),
            slug = o.getString("slug"),
            type = o.getString("type"),
            title = o.getString("title"),
            titleAlt = o.optString("title_alt").ifBlank { null },
            posterUrl = o.optString("poster_url").ifBlank { null },
            year = if (o.has("year") && !o.isNull("year")) o.getInt("year") else null,
            status = o.getString("status"),
            genres = buildList {
                val arr = o.optJSONArray("genres")
                if (arr != null) for (i in 0 until arr.length()) add(arr.getString(i))
            },
            rating = if (o.has("rating") && !o.isNull("rating")) o.getDouble("rating") else null,
            isAdult = o.optBoolean("is_adult"),
        )
    }
}

data class EpisodeItem(
    val id: String,
    val episodeNo: Int,
    val title: String?,
    val durationSec: Int?,
) {
    companion object {
        fun fromJson(o: JSONObject) = EpisodeItem(
            id = o.getString("id"),
            episodeNo = o.getInt("episode_no"),
            title = o.optString("title").ifBlank { null },
            durationSec = if (o.has("duration_sec") && !o.isNull("duration_sec")) o.getInt("duration_sec") else null,
        )
    }
}

data class ChapterItem(
    val id: String,
    val chapterNo: Double,
    val title: String?,
    val pageCount: Int,
) {
    companion object {
        fun fromJson(o: JSONObject) = ChapterItem(
            id = o.getString("id"),
            chapterNo = o.getDouble("chapter_no"),
            title = o.optString("title").ifBlank { null },
            pageCount = o.optInt("page_count"),
        )
    }
}

data class TitleDetail(
    val card: TitleCard,
    val synopsis: String?,
    val episodes: List<EpisodeItem>,
    val chapters: List<ChapterItem>,
    val fetchMode: String = "server",
    val sourceDomain: String? = null,
    val sourceUrlTemplate: String? = null,
    val season: String? = null,
    val network: String? = null,
    val studio: String? = null,
    val durationMin: Int? = null,
    val country: String? = null,
) {
    companion object {
        fun fromJson(o: JSONObject): TitleDetail {
            val t = o.getJSONObject("title")
            return TitleDetail(
                card = TitleCard.fromJson(t),
                synopsis = t.optString("synopsis").ifBlank { null },
                season = t.optString("season").ifBlank { null },
                network = t.optString("network").ifBlank { null },
                studio = t.optString("studio").ifBlank { null },
                durationMin = if (t.has("duration_min") && !t.isNull("duration_min")) t.getInt("duration_min") else null,
                country = t.optString("country").ifBlank { null },
                fetchMode = t.optString("fetch_mode").ifBlank { "server" },
                sourceDomain = t.optString("source_domain").ifBlank { null },
                sourceUrlTemplate = t.optString("source_url_template").ifBlank { null },
                episodes = buildList {
                    val arr = o.optJSONArray("episodes")
                    if (arr != null) for (i in 0 until arr.length()) add(EpisodeItem.fromJson(arr.getJSONObject(i)))
                },
                chapters = buildList {
                    val arr = o.optJSONArray("chapters")
                    if (arr != null) for (i in 0 until arr.length()) add(ChapterItem.fromJson(arr.getJSONObject(i)))
                },
            )
        }
    }
}

data class PlaybackSource(
    val url: String,
    val label: String?,
    /** Bahasa subtitle aliran ini bila hardsub (F40); null = softsub/tanpa bahasa. */
    val lang: String? = null,
)

data class SubtitleItem(
    val lang: String,
    val source: String,
    val format: String,
    val fileUrl: String?,
    val status: String,
)

data class Playback(
    val episodeId: String,
    val sources: List<PlaybackSource>,
    val subtitles: List<SubtitleItem>,
    val introStartSec: Int?,
    val introEndSec: Int?,
    /** Judul pemilik episode ini (dari objek episode respons yang sama). */
    val titleId: String = "",
    /** Nomor episode ini (dari objek episode respons yang sama). */
    val episodeNo: Int = 0,
) {
    companion object {
        fun fromJson(o: JSONObject): Playback {
            val ep = o.getJSONObject("episode")
            return Playback(
                episodeId = ep.getString("id"),
                titleId = ep.optString("title_id"),
                episodeNo = ep.optInt("episode_no"),
                sources = buildList {
                    val arr = o.optJSONArray("sources")
                    if (arr != null) for (i in 0 until arr.length()) {
                        val s = arr.getJSONObject(i)
                        add(
                            PlaybackSource(
                                url = s.getString("url"),
                                label = s.optString("label").ifBlank { null },
                                lang = s.optString("lang").ifBlank { null },
                            )
                        )
                    }
                },
                subtitles = buildList {
                    val arr = o.optJSONArray("subtitles")
                    if (arr != null) for (i in 0 until arr.length()) {
                        val s = arr.getJSONObject(i)
                        add(
                            SubtitleItem(
                                lang = s.getString("lang"),
                                source = s.getString("source"),
                                format = s.getString("format"),
                                fileUrl = s.optString("file_url").ifBlank { null },
                                status = s.getString("status"),
                            )
                        )
                    }
                },
                introStartSec = if (ep.has("intro_start_sec") && !ep.isNull("intro_start_sec")) ep.getInt("intro_start_sec") else null,
                introEndSec = if (ep.has("intro_end_sec") && !ep.isNull("intro_end_sec")) ep.getInt("intro_end_sec") else null,
            )
        }
    }
}

/** Repositori katalog & akun — satu-satunya jalan UI ke API. */
class CatalogRepository(
    private val api: ApiClient = ApiClient(),
    val auth: AuthStore? = null,
) {
    init {
        if (auth != null) {
            api.tokenProvider = { auth.token }
        }
    }

    val isLoggedIn: Boolean get() = auth?.isLoggedIn == true

    // ---------- Akun (Fase 3) ----------

    /** Tukar token ID Google dengan JWT backend; sesi disimpan di AuthStore. */
    suspend fun loginGoogle(idToken: String): UserProfile {
        val o = api.post("/api/v1/auth/google", JSONObject().put("id_token", idToken))
        val token = o.getString("token")
        val p = o.getJSONObject("profile")
        val profile = UserProfile(
            id = p.getString("id"),
            email = p.getString("email"),
            displayName = p.optString("display_name").ifBlank { null },
            avatarUrl = p.optString("avatar_url").ifBlank { null },
            role = p.optString("role", "user"),
            isPremium = p.optBoolean("is_premium"),
            appLanguage = p.optString("app_language", "id"),
            friendCode = p.optString("friend_code").ifBlank { null },
        )
        auth?.save(
            token = token,
            viewingProfileId = o.optString("viewing_profile_id").ifBlank { null },
            email = profile.email,
            displayName = profile.displayName,
            friendCode = profile.friendCode,
        )
        return profile
    }

    fun logout() {
        auth?.clear()
    }

    // ---------- Watchlist (Fase 3) ----------

    suspend fun watchlistCheck(titleId: String): Boolean =
        api.get("/api/v1/watchlist/$titleId").optBoolean("in_watchlist")

    suspend fun watchlistAdd(titleId: String) {
        api.post("/api/v1/watchlist/$titleId", JSONObject().put("notify", false))
    }

    suspend fun watchlistRemove(titleId: String) {
        api.delete("/api/v1/watchlist/$titleId")
    }

    suspend fun watchlist(): List<TitleCard> {
        val o = api.get("/api/v1/watchlist")
        return buildList {
            val arr = o.optJSONArray("items")
            if (arr != null) for (i in 0 until arr.length()) add(TitleCard.fromJson(arr.getJSONObject(i)))
        }
    }

    // ---------- Riwayat (Fase 3) ----------

    /** Lapor progres tonton; kegagalan diabaikan (bukan fungsi kritis). */
    suspend fun reportWatch(episodeId: String, positionSec: Int) {
        if (!isLoggedIn) return
        try {
            api.post(
                "/api/v1/history/watch",
                JSONObject().put("episode_id", episodeId).put("position_sec", positionSec),
            )
        } catch (e: Exception) {
            // Abaikan — progres dicoba lagi pada laporan berikutnya.
        }
    }

    /** Lapor progres baca chapter server (Fase 5). */
    suspend fun reportRead(chapterId: String, lastPage: Int) {
        if (!isLoggedIn) return
        try {
            api.post(
                "/api/v1/history/read",
                JSONObject().put("chapter_id", chapterId).put("last_page", lastPage),
            )
        } catch (e: Exception) {
            // Abaikan — progres dicoba lagi pada laporan berikutnya.
        }
    }

    /** Lapor progres baca chapter eksternal F39 (diselesaikan di perangkat). */
    suspend fun reportReadExternal(titleId: String, label: String, no: Double, lastPage: Int) {
        if (!isLoggedIn) return
        try {
            api.post(
                "/api/v1/history/read",
                JSONObject()
                    .put("title_id", titleId)
                    .put("external_label", label)
                    .put("external_no", no)
                    .put("last_page", lastPage),
            )
        } catch (e: Exception) {
            // Abaikan — progres dicoba lagi pada laporan berikutnya.
        }
    }

    // ---------- Premium & pembayaran (Fase 4) ----------

    suspend fun plans(): List<PlanItem> {
        val o = api.get("/api/v1/plans")
        return buildList {
            val arr = o.optJSONArray("plans")
            if (arr != null) for (i in 0 until arr.length()) {
                val p = arr.getJSONObject(i)
                add(PlanItem(p.getString("id"), p.getInt("days"), p.getInt("price"), p.optString("label")))
            }
        }
    }

    suspend fun premiumStatus(): PremiumStatus {
        val o = api.get("/api/v1/premium/status")
        return PremiumStatus(
            tier = o.optString("tier", "free"),
            isPremium = o.optBoolean("is_premium"),
            premiumUntil = o.optString("premium_until").ifBlank { null },
            trialAvailable = o.optBoolean("trial_available"),
        )
    }

    suspend fun claimTrial(): String? {
        val o = api.post("/api/v1/premium/trial")
        return o.optString("premium_until").ifBlank { null }
    }

    suspend fun checkout(
        planId: String,
        provider: String,
        purpose: String,
        recipientCode: String? = null,
        channel: String? = null,
    ): JSONObject {
        val body = JSONObject()
            .put("plan_id", planId)
            .put("provider", provider)
            .put("purpose", purpose)
        if (recipientCode != null) body.put("recipient_code", recipientCode)
        if (channel != null) body.put("channel", channel)
        return api.post("/api/v1/payments/checkout", body)
    }

    suspend fun redeemVoucher(code: String): JSONObject =
        api.post("/api/v1/vouchers/redeem", JSONObject().put("code", code))

    suspend fun myVouchers(): List<VoucherItem> {
        val o = api.get("/api/v1/vouchers/mine")
        return buildList {
            val arr = o.optJSONArray("vouchers")
            if (arr != null) for (i in 0 until arr.length()) {
                val v = arr.getJSONObject(i)
                add(VoucherItem(v.getString("code"), v.getInt("duration_days"), v.optString("source")))
            }
        }
    }

    // ---------- Sosial: chat, teman, grup (Fase 4) ----------

    suspend fun chatChannels(): List<ChatChannelItem> {
        val o = api.get("/api/v1/chat/channels")
        return buildList {
            val arr = o.optJSONArray("channels")
            if (arr != null) for (i in 0 until arr.length()) {
                val c = arr.getJSONObject(i)
                add(
                    ChatChannelItem(
                        id = c.getString("id"),
                        name = c.getString("name"),
                        kind = c.optString("kind", "global"),
                    )
                )
            }
        }
    }

    suspend fun chatHistory(channelId: String): List<ChatMsg> {
        val o = api.get("/api/v1/chat/channels/$channelId/messages")
        return buildList {
            val arr = o.optJSONArray("messages")
            if (arr != null) for (i in 0 until arr.length()) {
                add(ChatMsg.fromJson(arr.getJSONObject(i)))
            }
        }
    }

    suspend fun friends(): List<FriendItem> {
        val o = api.get("/api/v1/friends")
        return buildList {
            val arr = o.optJSONArray("friends")
            if (arr != null) for (i in 0 until arr.length()) {
                val f = arr.getJSONObject(i)
                add(
                    FriendItem(
                        userId = f.getString("user_id"),
                        email = f.optString("email"),
                        displayName = f.optString("display_name").ifBlank { null },
                        status = f.optString("status"),
                    )
                )
            }
        }
    }

    suspend fun friendRequest(code: String): JSONObject =
        api.post("/api/v1/friends/request", JSONObject().put("friend_code", code))

    suspend fun friendAccept(userId: String): JSONObject =
        api.post("/api/v1/friends/accept", JSONObject().put("user_id", userId))

    suspend fun friendBlock(userId: String): JSONObject =
        api.post("/api/v1/friends/block", JSONObject().put("user_id", userId))

    suspend fun dmOpen(userId: String): String =
        api.post("/api/v1/dm/$userId").getString("channel_id")

    suspend fun groups(): List<GroupItem> {
        val o = api.get("/api/v1/groups")
        return buildList {
            val arr = o.optJSONArray("groups")
            if (arr != null) for (i in 0 until arr.length()) {
                val g = arr.getJSONObject(i)
                add(
                    GroupItem(
                        id = g.getString("id"),
                        name = g.getString("name"),
                        isPublic = g.optBoolean("is_public"),
                        isMember = g.optBoolean("is_member"),
                        inviteCode = g.optString("invite_code").ifBlank { null },
                        channelId = g.optString("channel_id").ifBlank { null },
                    )
                )
            }
        }
    }

    suspend fun groupCreate(name: String, isPublic: Boolean): JSONObject =
        api.post("/api/v1/groups", JSONObject().put("name", name).put("is_public", isPublic))

    suspend fun groupJoin(groupId: String): JSONObject =
        api.post("/api/v1/groups/$groupId/join")

    suspend fun groupJoinByCode(code: String): JSONObject =
        api.post("/api/v1/groups/join", JSONObject().put("invite_code", code))

    suspend fun watchHistory(limit: Int = 10): List<WatchHistoryItem> {
        val o = api.get("/api/v1/history/watch?limit=$limit")
        return buildList {
            val arr = o.optJSONArray("items")
            if (arr != null) for (i in 0 until arr.length()) {
                val it = arr.getJSONObject(i)
                add(
                    WatchHistoryItem(
                        episodeId = it.getString("episode_id"),
                        titleId = it.getString("title_id"),
                        title = it.getString("title"),
                        posterUrl = it.optString("poster_url").ifBlank { null },
                        episodeNo = it.getInt("episode_no"),
                        positionSec = it.optInt("position_sec"),
                    )
                )
            }
        }
    }

    suspend fun home(): Pair<List<TitleCard>, List<TitleCard>> {
        val o = api.get("/api/v1/home")
        fun cards(key: String) = buildList {
            val arr = o.optJSONArray(key)
            if (arr != null) for (i in 0 until arr.length()) add(TitleCard.fromJson(arr.getJSONObject(i)))
        }
        return cards("trending") to cards("terbaru")
    }

    suspend fun titles(type: String?, query: String?): List<TitleCard> {
        val params = StringBuilder("/api/v1/titles?limit=30")
        if (!type.isNullOrBlank()) params.append("&type=").append(type)
        if (!query.isNullOrBlank()) params.append("&q=").append(java.net.URLEncoder.encode(query, "UTF-8"))
        val o = api.get(params.toString())
        return buildList {
            val arr = o.optJSONArray("items")
            if (arr != null) for (i in 0 until arr.length()) add(TitleCard.fromJson(arr.getJSONObject(i)))
        }
    }

    suspend fun titleDetail(id: String): TitleDetail = TitleDetail.fromJson(api.get("/api/v1/titles/$id"))

    suspend fun playback(episodeId: String): Playback = Playback.fromJson(api.get("/api/v1/episodes/$episodeId/playback"))

    // ---------- Feed komunitas (redesain Chat 2026-11-22) ----------

    suspend fun feedList(
        tab: String,
        category: String?,
        query: String?,
        groupId: String?,
        cursor: String?,
    ): Pair<List<FeedPost>, String> {
        val params = StringBuilder("/api/v1/feed?tab=").append(tab)
        if (!category.isNullOrBlank()) params.append("&category=").append(category)
        if (!query.isNullOrBlank()) {
            params.append("&q=").append(java.net.URLEncoder.encode(query, "UTF-8"))
        }
        if (!groupId.isNullOrBlank()) params.append("&group_id=").append(groupId)
        if (!cursor.isNullOrBlank()) {
            params.append("&cursor=").append(java.net.URLEncoder.encode(cursor, "UTF-8"))
        }
        val o = api.get(params.toString())
        val items = buildList {
            val arr = o.optJSONArray("items")
            if (arr != null) for (i in 0 until arr.length()) add(FeedPost.fromJson(arr.getJSONObject(i)))
        }
        return items to o.optString("next_cursor")
    }

    suspend fun feedPost(id: String): FeedPost =
        FeedPost.fromJson(api.get("/api/v1/feed/posts/$id").getJSONObject("post"))

    suspend fun feedCreate(body: String, imageUrl: String?, category: String, groupId: String?): FeedPost {
        val req = org.json.JSONObject()
            .put("body", body)
            .put("category", category)
        if (imageUrl != null) req.put("image_url", imageUrl)
        if (groupId != null) req.put("group_id", groupId)
        return FeedPost.fromJson(api.post("/api/v1/feed/posts", req).getJSONObject("post"))
    }

    suspend fun feedDeletePost(id: String) {
        api.delete("/api/v1/feed/posts/$id")
    }

    /** Mengembalikan (liked, jumlah like) sesudah toggle. */
    suspend fun feedLike(id: String): Pair<Boolean, Int> {
        val o = api.post("/api/v1/feed/posts/$id/like")
        return o.optBoolean("liked") to o.optInt("like_count")
    }

    suspend fun feedBookmark(id: String): Boolean =
        api.post("/api/v1/feed/posts/$id/bookmark").optBoolean("bookmarked")

    suspend fun feedShare(id: String): Int =
        api.post("/api/v1/feed/posts/$id/share").optInt("share_count")

    suspend fun feedComments(postId: String): List<FeedComment> {
        val o = api.get("/api/v1/feed/posts/$postId/comments")
        return buildList {
            val arr = o.optJSONArray("items")
            if (arr != null) for (i in 0 until arr.length()) add(FeedComment.fromJson(arr.getJSONObject(i)))
        }
    }

    suspend fun feedCommentCreate(postId: String, body: String, parentId: String?): FeedComment {
        val req = org.json.JSONObject().put("body", body)
        if (parentId != null) req.put("parent_id", parentId)
        return FeedComment.fromJson(
            api.post("/api/v1/feed/posts/$postId/comments", req).getJSONObject("comment")
        )
    }

    suspend fun feedCommentLike(id: String): Pair<Boolean, Int> {
        val o = api.post("/api/v1/feed/comments/$id/like")
        return o.optBoolean("liked") to o.optInt("like_count")
    }

    suspend fun feedCommentDelete(id: String) {
        api.delete("/api/v1/feed/comments/$id")
    }

    /** Unggah gambar postingan; mengembalikan URL publiknya. */
    suspend fun feedUpload(bytes: ByteArray, filename: String): String =
        api.uploadImage("/api/v1/feed/upload", bytes, filename).getString("url")

    suspend fun feedNotifications(): Pair<List<FeedNotification>, Int> {
        val o = api.get("/api/v1/feed/notifications")
        val items = buildList {
            val arr = o.optJSONArray("items")
            if (arr != null) for (i in 0 until arr.length()) add(FeedNotification.fromJson(arr.getJSONObject(i)))
        }
        return items to o.optInt("unread")
    }

    suspend fun feedNotificationsRead() {
        api.post("/api/v1/feed/notifications/read")
    }
}
