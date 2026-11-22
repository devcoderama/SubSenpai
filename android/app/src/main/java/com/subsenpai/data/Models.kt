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
) {
    companion object {
        fun fromJson(o: JSONObject): TitleDetail {
            val t = o.getJSONObject("title")
            return TitleDetail(
                card = TitleCard.fromJson(t),
                synopsis = t.optString("synopsis").ifBlank { null },
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

data class PlaybackSource(val url: String, val label: String?)

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
) {
    companion object {
        fun fromJson(o: JSONObject): Playback {
            val ep = o.getJSONObject("episode")
            return Playback(
                episodeId = ep.getString("id"),
                sources = buildList {
                    val arr = o.optJSONArray("sources")
                    if (arr != null) for (i in 0 until arr.length()) {
                        val s = arr.getJSONObject(i)
                        add(PlaybackSource(s.getString("url"), s.optString("label").ifBlank { null }))
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

data class ChapterPage(
    val pageNo: Int,
    val imageUrl: String,
    val lightUrl: String?,
)

/** Repositori katalog — satu-satunya jalan UI ke API. */
class CatalogRepository(private val api: ApiClient = ApiClient()) {

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

    suspend fun chapterPages(chapterId: String): List<ChapterPage> {
        val o = api.get("/api/v1/chapters/$chapterId")
        return buildList {
            val arr = o.optJSONArray("pages")
            if (arr != null) for (i in 0 until arr.length()) {
                val p = arr.getJSONObject(i)
                add(
                    ChapterPage(
                        pageNo = p.getInt("page_no"),
                        imageUrl = p.getString("image_url"),
                        lightUrl = p.optString("light_url").ifBlank { null },
                    )
                )
            }
        }
    }
}
