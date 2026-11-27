package com.subsenpai.data

import org.json.JSONArray
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
    /** Terkunci untuk tier penonton ini (non-premium: film; anime & donghua selalu terbuka). */
    val locked: Boolean = false,
) {
    companion object {
        fun fromJson(o: JSONObject) = EpisodeItem(
            id = o.getString("id"),
            episodeNo = o.getInt("episode_no"),
            title = o.optString("title").ifBlank { null },
            durationSec = if (o.has("duration_sec") && !o.isNull("duration_sec")) o.getInt("duration_sec") else null,
            locked = o.optBoolean("locked"),
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

// Teks bagikan kaya (v0.24.17, permintaan Rams 2026-11-24):
// format sama dengan notifikasi jadwal rilis — judul, judul
// lain, genre, studio, musim, sinopsis ringkas, tautan nonton.
// Dari pemutar (episodeId terisi), tautannya /nonton/{episodeId}
// yang membuka aplikasi langsung memutar episode itu; dari
// halaman detail, tautannya /judul/{id}.
fun shareTextFor(detail: TitleDetail, episodeId: String? = null, episodeNo: Int? = null): String {
    val d = detail
    val b = StringBuilder()
    b.append("Nonton ").append(d.card.title)
    if (episodeNo != null) b.append(" — Episode ").append(episodeNo)
    b.append(" di SubSenpai")
    d.card.titleAlt?.takeIf { it.isNotBlank() }?.let { b.append("\nJudul lain: ").append(it) }
    if (d.card.genres.isNotEmpty()) {
        b.append("\nGenre: ").append(d.card.genres.joinToString(", "))
    }
    val meta = mutableListOf<String>()
    d.studio?.takeIf { it.isNotBlank() }?.let { meta += "Studio: $it" }
    val seasonLabel = d.season?.takeIf { it.isNotBlank() }?.let { s ->
        s.replaceFirstChar { it.uppercase() } + (d.card.year?.let { " $it" } ?: "")
    }
    if (seasonLabel != null) meta += "Musim: $seasonLabel"
    else d.card.year?.let { meta += "Tahun: $it" }
    if (meta.isNotEmpty()) b.append("\n").append(meta.joinToString(" • "))
    d.synopsis?.takeIf { it.isNotBlank() }?.let {
        val ringkas = if (it.length > 180) it.take(180).trimEnd() + "…" else it
        b.append("\n").append(ringkas)
    }
    if (episodeId != null) {
        b.append("\n▶️ Nonton")
        if (episodeNo != null) b.append(" Episode ").append(episodeNo)
        b.append(": https://www.subsenpai.web.id/nonton/").append(episodeId)
    } else {
        b.append("\n▶️ Nonton: https://www.subsenpai.web.id/judul/").append(d.card.id)
    }
    return b.toString()
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
/**
 * Ubah pesan penolakan akun nonaktif dari server
 * ("akun_nonaktif:<status>:<catatan>") menjadi teks ramah untuk
 * layar masuk & layar blokir. Pesan lain diteruskan apa adanya.
 */
fun parseBlockedMessage(msg: String?): Pair<String, String>? {
    val m = msg.orEmpty()
    if (!m.startsWith("akun_nonaktif:")) return null
    val parts = m.split(":", limit = 3)
    return (parts.getOrNull(1) ?: "") to (parts.getOrNull(2) ?: "")
}

fun blockedTitle(status: String): String = when (status) {
    "banned" -> "Akun Dibekukan"
    "suspended" -> "Akun Ditangguhkan"
    else -> "Akun Tidak Aktif"
}

fun blockedDescription(status: String, note: String): String {
    val base = when (status) {
        "banned" -> "Akunmu dibekukan oleh admin dan tidak bisa digunakan untuk masuk atau menonton."
        "suspended" -> "Akunmu sedang ditangguhkan oleh admin dan untuk sementara tidak bisa digunakan."
        else -> "Akunmu sedang tidak aktif dan tidak bisa digunakan untuk masuk atau menonton."
    }
    val notePart = note.takeIf { it.isNotBlank() }?.let { "\n\nCatatan admin: $it" } ?: ""
    return base + notePart + "\n\nHubungi admin SubSenpai bila menurutmu ini keliru."
}

class CatalogRepository(
    private val api: ApiClient = ApiClient(),
    val auth: AuthStore? = null,
) {
    init {
        if (auth != null) {
            api.tokenProvider = { auth.token }
            api.deviceIdProvider = { auth.deviceId }
        }
    }

    val isLoggedIn: Boolean get() = auth?.isLoggedIn == true

    // Cache kartu judul di memori: kartu yang sudah tampil di
    // Beranda/daftar dipakai halaman detail untuk menggambar
    // kepala halaman SEKETIKA saat poster diketuk, selagi detail
    // penuh masih dimuat (optimasi 2026-11-23).
    private val cardCache = HashMap<String, TitleCard>()

    /** Kartu judul yang sudah dikenal aplikasi, bila ada. */
    fun cachedCard(id: String): TitleCard? = cardCache[id]

    private fun cacheCards(cards: List<TitleCard>) {
        for (c in cards) cardCache[c.id] = c
    }

    // Hasil playback yang di-prefetch dari halaman detail agar
    // tombol Putar tidak menunggu satu perjalanan jaringan lagi.
    // Sekali pakai dan kedaluwarsa cepat (URL aliran bisa berubah).
    private var playbackPrefetch: Triple<String, Playback, Long>? = null

    /** Ambil di latar hasil playback satu episode untuk Putar nanti. */
    suspend fun prefetchPlayback(episodeId: String) {
        try {
            val pb = playback(episodeId)
            playbackPrefetch = Triple(episodeId, pb, System.currentTimeMillis())
        } catch (e: Exception) {
            // Prefetch gagal tidak masalah: pemutar mengambil sendiri.
        }
    }

    /** Ambil hasil prefetch (sekali pakai, maks 3 menit) bila cocok. */
    fun takeCachedPlayback(episodeId: String): Playback? {
        val p = playbackPrefetch ?: return null
        playbackPrefetch = null
        return if (p.first == episodeId &&
            System.currentTimeMillis() - p.third < 180_000
        ) {
            p.second
        } else {
            null
        }
    }

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

    /**
     * Profil sendiri dari server (GET /api/v1/me). Dipakai layar
     * utama untuk memeriksa status akun tiap siklus poll: akun
     * yang di-ban/ditangguhkan admin ditolak server di sini
     * dengan ApiException "akun_nonaktif:..." (v0.24.15).
     */
    suspend fun me(): UserProfile {
        val o = api.get("/api/v1/me")
        val p = o.getJSONObject("profile")
        return UserProfile(
            id = p.getString("id"),
            email = p.getString("email"),
            displayName = p.optString("display_name").ifBlank { null },
            avatarUrl = p.optString("avatar_url").ifBlank { null },
            role = p.optString("role", "user"),
            isPremium = p.optBoolean("is_premium"),
            appLanguage = p.optString("app_language", "id"),
            friendCode = p.optString("friend_code").ifBlank { null },
        )
    }

    // ---------- Watchlist (Fase 3) ----------

    suspend fun watchlistCheck(titleId: String): Boolean =
        api.get("/api/v1/watchlist/$titleId").optBoolean("in_watchlist")

    suspend fun watchlistAdd(titleId: String) {
        api.post("/api/v1/watchlist/$titleId", JSONObject().put("notify", true))
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

    /** Konfigurasi aplikasi (perawatan / paksa perbarui / pengumuman). */
    suspend fun appConfig(): AppConfig {
        val o = api.get("/api/v1/app-config")
        return AppConfig(
            maintenanceMode = o.optBoolean("maintenance_mode"),
            maintenanceMessage = o.optString("maintenance_message"),
            minVersionCode = o.optInt("min_version_code"),
            minVersionName = o.optString("min_version_name"),
            updateUrl = o.optString("update_url"),
            announcement = o.optString("announcement"),
        )
    }

    /** Unggah satu laporan crash yang disimpan CrashReporter. */
    suspend fun uploadCrashReport(body: org.json.JSONObject) {
        api.post("/api/v1/crashes", body)
    }

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

    /** Metode pembayaran aktif dari database (diatur admin). */
    suspend fun paymentMethods(): List<PaymentMethodItem> {
        val o = api.get("/api/v1/payment-methods")
        return buildList {
            val arr = o.optJSONArray("methods")
            if (arr != null) for (i in 0 until arr.length()) {
                val m = arr.getJSONObject(i)
                add(
                    PaymentMethodItem(
                        m.getString("code"), m.optString("name", m.getString("code")),
                        m.optString("group"), m.optString("icon_url"),
                    ),
                )
            }
        }
    }

    /** Jadwal rilis per kategori: anime (Kuronime), donghua (Anichin), film (rilis terbaru katalog). */
    suspend fun schedule(type: String): List<ScheduleGroup> {
        val o = api.get("/api/v1/schedule?type=$type")
        return buildList {
            val groups = o.optJSONArray("groups")
            if (groups != null) for (g in 0 until groups.length()) {
                val go = groups.getJSONObject(g)
                val items = buildList {
                    val arr = go.optJSONArray("items")
                    if (arr != null) for (i in 0 until arr.length()) {
                        val it = arr.getJSONObject(i)
                        add(
                            ScheduleItem(
                                title = it.optString("title"),
                                time = it.optString("time"),
                                episode = it.optInt("episode"),
                                poster = it.optString("poster").ifBlank { null },
                                titleId = it.optString("title_id").ifBlank { null },
                                slug = it.optString("slug"),
                                url = it.optString("url").ifBlank { null },
                            ),
                        )
                    }
                }
                add(ScheduleGroup(go.optString("day"), items))
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
        quantity: Int = 1,
    ): JSONObject {
        val body = JSONObject()
            .put("plan_id", planId)
            .put("provider", provider)
            .put("purpose", purpose)
        if (recipientCode != null) body.put("recipient_code", recipientCode)
        if (channel != null) body.put("channel", channel)
        if (quantity > 1) body.put("quantity", quantity)
        return api.post("/api/v1/payments/checkout", body)
    }

    /** Cek status pembayaran (server menanyakan gateway & mengaktifkan sesi bila lunas). */
    suspend fun paymentStatus(sessionId: String): JSONObject =
        api.get("/api/v1/payments/status/$sessionId")

    /** Riwayat transaksi (sesi pembayaran) milik user. */
    suspend fun transactions(): List<TransactionItem> {
        val o = api.get("/api/v1/transactions")
        return buildList {
            val arr = o.optJSONArray("transactions")
            if (arr != null) for (i in 0 until arr.length()) {
                val t = arr.getJSONObject(i)
                add(
                    TransactionItem(
                        t.getString("id"), t.optString("purpose"), t.optString("provider"),
                        t.optString("plan_label"), t.optInt("amount"), t.optString("status"),
                        t.optString("created_at"),
                    ),
                )
            }
        }
    }

    suspend fun redeemVoucher(code: String): JSONObject =
        api.post("/api/v1/vouchers/redeem", JSONObject().put("code", code))

    suspend fun myVouchers(): List<VoucherItem> {
        val o = api.get("/api/v1/vouchers/mine")
        return buildList {
            val arr = o.optJSONArray("vouchers")
            if (arr != null) for (i in 0 until arr.length()) {
                val v = arr.getJSONObject(i)
                add(
                    VoucherItem(
                        code = v.getString("code"),
                        days = v.getInt("duration_days"),
                        source = v.optString("source"),
                        status = v.optString("status", "active"),
                        createdByMe = v.optBoolean("created_by_me"),
                        forMe = v.optBoolean("for_me"),
                        paymentStatus = v.optString("payment_status").ifBlank { null },
                        redeemedByName = v.optString("redeemed_by_name").ifBlank { null },
                    ),
                )
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
        val result = cards("trending") to cards("terbaru")
        cacheCards(result.first)
        cacheCards(result.second)
        return result
    }

    suspend fun titles(
        type: String?,
        query: String?,
        genre: String? = null,
        year: Int? = null,
        country: String? = null,
        network: String? = null,
    ): List<TitleCard> = titlesPaged(type, query, genre, year, country, network, null).first

    /**
     * Satu halaman katalog dengan pagination kursor server —
     * mengembalikan (item, kursor halaman berikutnya / null bila
     * habis). Dipakai daftar Hasil agar memuat bertahap saat
     * digulir (ala Netflix/YouTube), tidak sekaligus.
     */
    suspend fun titlesPaged(
        type: String?,
        query: String?,
        genre: String? = null,
        year: Int? = null,
        country: String? = null,
        network: String? = null,
        cursor: String? = null,
        limit: Int = 30,
    ): Pair<List<TitleCard>, String?> {
        val params = StringBuilder("/api/v1/titles?limit=").append(limit)
        if (!type.isNullOrBlank()) params.append("&type=").append(type)
        if (!query.isNullOrBlank()) params.append("&q=").append(java.net.URLEncoder.encode(query, "UTF-8"))
        if (!genre.isNullOrBlank()) params.append("&genre=").append(java.net.URLEncoder.encode(genre, "UTF-8"))
        if (year != null) params.append("&year=").append(year)
        if (!country.isNullOrBlank()) params.append("&country=").append(java.net.URLEncoder.encode(country, "UTF-8"))
        if (!network.isNullOrBlank()) params.append("&network=").append(java.net.URLEncoder.encode(network, "UTF-8"))
        if (!cursor.isNullOrBlank()) params.append("&cursor=").append(java.net.URLEncoder.encode(cursor, "UTF-8"))
        val o = api.get(params.toString())
        val items = buildList {
            val arr = o.optJSONArray("items")
            if (arr != null) for (i in 0 until arr.length()) add(TitleCard.fromJson(arr.getJSONObject(i)))
        }
        cacheCards(items)
        return items to o.optString("next_cursor").ifBlank { null }
    }

    /** Satu judul hasil telusur langsung dari situs sumber. */
    data class LiveBrowseItem(
        val title: String,
        val slug: String,
        val posterUrl: String?,
        val year: Int?,
        val genres: List<String>,
        val url: String,
        val catalogId: String?,
    )

    /**
     * Telusur genre LANGSUNG dari situs sumber (pilihan filter
     * sebanyak di web sumbernya). Mengembalikan (nama sumber,
     * masih ada halaman berikutnya, daftar judul).
     */
    suspend fun liveBrowse(type: String, genre: String?, page: Int): Triple<String, Boolean, List<LiveBrowseItem>> {
        val path = StringBuilder("/api/v1/live-browse?type=").append(type).append("&page=").append(page)
        if (!genre.isNullOrBlank()) path.append("&genre=").append(java.net.URLEncoder.encode(genre, "UTF-8"))
        val o = api.get(path.toString())
        val items = buildList {
            val arr = o.optJSONArray("items")
            if (arr != null) for (i in 0 until arr.length()) {
                val it = arr.getJSONObject(i)
                val gs = buildList {
                    val ga = it.optJSONArray("genres")
                    if (ga != null) for (j in 0 until ga.length()) add(ga.getString(j))
                }
                add(
                    LiveBrowseItem(
                        title = it.optString("title"),
                        slug = it.optString("slug"),
                        posterUrl = it.optString("poster_url").ifBlank { null },
                        year = if (it.has("year")) it.optInt("year") else null,
                        genres = gs,
                        url = it.optString("url"),
                        catalogId = it.optString("catalog_id").ifBlank { null },
                    ),
                )
            }
        }
        return Triple(o.optString("source"), o.optBoolean("has_more"), items)
    }

    /** Impor satu judul hasil telusur ke katalog; mengembalikan id judul. */
    suspend fun importTitle(type: String, item: LiveBrowseItem, genre: String?): String {
        val body = JSONObject()
            .put("type", type)
            .put("url", item.url)
            .put("title", item.title)
            .put("poster_url", item.posterUrl ?: "")
            .put("year", item.year ?: 0)
            .put("genres", JSONArray(item.genres))
        if (!genre.isNullOrBlank()) body.put("genre", genre)
        return api.post("/api/v1/titles/import", body).getString("id")
    }

    /** Daftar genre yang ada di katalog untuk satu tipe (chip filter). */
    suspend fun genres(type: String): List<String> {
        val o = api.get("/api/v1/genres?type=$type")
        return buildList {
            val arr = o.optJSONArray("genres")
            if (arr != null) for (i in 0 until arr.length()) add(arr.getJSONObject(i).getString("name"))
        }
    }

    /** Faset filter lanjutan (genre, tahun, negara, network) satu tipe. */
    suspend fun filters(type: String): TitleFacets {
        val o = api.get("/api/v1/filters?type=$type")
        fun names(key: String): List<String> = buildList {
            val arr = o.optJSONArray(key)
            if (arr != null) for (i in 0 until arr.length()) add(arr.getJSONObject(i).getString("name"))
        }
        fun options(key: String): List<FacetOption> = buildList {
            val arr = o.optJSONArray(key)
            if (arr != null) for (i in 0 until arr.length()) {
                val f = arr.getJSONObject(i)
                add(FacetOption(f.getString("name"), f.getString("value")))
            }
        }
        val years = buildList {
            val arr = o.optJSONArray("years")
            if (arr != null) for (i in 0 until arr.length()) {
                arr.getJSONObject(i).getString("value").toIntOrNull()?.let { add(it) }
            }
        }
        return TitleFacets(names("genres"), years, options("countries"), names("networks"))
    }

    suspend fun titleDetail(id: String): TitleDetail {
        val d = TitleDetail.fromJson(api.get("/api/v1/titles/$id"))
        cardCache[d.card.id] = d.card
        return d
    }

    suspend fun playback(episodeId: String): Playback = Playback.fromJson(api.get("/api/v1/episodes/$episodeId/playback"))

    /** Mulai sesi unlock iklan: server menerbitkan token + lama
     * tonton minimum (detik) yang ditegakkan di sisi server. */
    /** Kirim laporan masalah putar (link mati, semua server
     * gagal, dsb.) dari halaman pemutar ke admin. */
    suspend fun createReport(episodeId: String, reason: String, note: String): Boolean =
        try {
            api.post(
                "/api/v1/reports",
                JSONObject().put("episode_id", episodeId).put("reason", reason).put("note", note),
            )
            true
        } catch (e: Exception) {
            false
        }

    suspend fun adUnlockStart(episodeId: String): Pair<String, Int> {
        val o = api.post("/api/v1/episodes/$episodeId/ad-unlock/start")
        return o.getString("token") to o.optInt("wait_seconds", 15)
    }

    /** Selesaikan sesi unlock iklan: episode terbuka permanen untuk
     * user/perangkat ini bila token sudah cukup umur di server. */
    suspend fun adUnlockComplete(episodeId: String, token: String) {
        api.post(
            "/api/v1/episodes/$episodeId/ad-unlock/complete",
            JSONObject().put("token", token),
        )
    }

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

    suspend fun feedCreate(body: String, imageUrl: String?, category: String, visibility: String, groupId: String?, titleId: String? = null): FeedPost {
        val req = org.json.JSONObject()
            .put("body", body)
            .put("category", category)
            .put("visibility", visibility)
        if (imageUrl != null) req.put("image_url", imageUrl)
        if (groupId != null) req.put("group_id", groupId)
        if (titleId != null) req.put("title_id", titleId)
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

    /** Toggle posting ulang; mengembalikan (reposted, jumlah repost). */
    suspend fun feedRepost(id: String): Pair<Boolean, Int> {
        val o = api.post("/api/v1/feed/posts/$id/repost")
        return o.optBoolean("reposted") to o.optInt("repost_count")
    }

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

    /** Unggah video langsung dari perangkat ke image2url (null = gagal). */
    suspend fun feedUploadVideoDirect(bytes: ByteArray): String? =
        api.uploadImage2UrlDirect(bytes)

    /** Urai tautan pendek (vt.tiktok.com dsb.) ke URL akhirnya. */
    suspend fun resolveUrl(url: String): String? = api.resolveRedirect(url)

    /** Unduh berkas mentah (poster untuk Bagikan ke Post). */
    suspend fun downloadBytes(url: String): ByteArray? = api.downloadBytes(url)

    /**
     * Notifikasi pengguna. Pengumuman admin (kind "broadcast") dan
     * aktivitas sosial (kind "social") adalah dua hal TERPISAH:
     * saring dengan kind, atau null untuk keduanya (perilaku lama,
     * dipakai lencana & polling notifikasi sistem).
     */
    suspend fun feedNotifications(kind: String? = null): Pair<List<FeedNotification>, Int> {
        val path = if (kind != null) "/api/v1/feed/notifications?kind=$kind" else "/api/v1/feed/notifications"
        val o = api.get(path)
        val items = buildList {
            val arr = o.optJSONArray("items")
            if (arr != null) for (i in 0 until arr.length()) add(FeedNotification.fromJson(arr.getJSONObject(i)))
        }
        return items to o.optInt("unread")
    }

    /** Tandai dibaca; kind membatasi hanya jenis itu saja. */
    suspend fun feedNotificationsRead(kind: String? = null) {
        if (kind != null) {
            api.post(
                "/api/v1/feed/notifications/read",
                org.json.JSONObject().put("kind", kind),
            )
        } else {
            api.post("/api/v1/feed/notifications/read")
        }
    }
}
