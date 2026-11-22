package com.subsenpai.data

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import okhttp3.OkHttpClient
import okhttp3.Request
import org.json.JSONObject
import java.util.concurrent.TimeUnit

/** Satu seri komik dari API publik manhuarmtl (dibaca di perangkat). */
data class ComicSeries(
    val slug: String,
    val title: String,
    val url: String,
    val poster: String?,
    val type: String,
    val status: String?,
    val year: Int?,
    val genres: List<String>,
    val synopsis: String?,
    val author: String? = null,
    val artist: String? = null,
    val rating: String? = null,
)

/** Satu chapter komik (URL halaman chapter di situs sumber). */
data class ComicChapter(val label: String, val url: String, val no: Double)

/** Pegangan navigasi layar komik (pola sama dengan ReaderNav). */
object ComicNav {
    var series: ComicSeries? = null
    var chapters: List<ComicChapter> = emptyList()
    var startChapter: ComicChapter? = null
}

/**
 * Klien API publik manhuarmtl (/wp-json/manhuarmtl/v1) yang dipanggil
 * LANGSUNG dari perangkat — pola ekstensi sumber ala Mihon: daftar seri,
 * pencarian, dan detail + daftar chapter tidak lewat server SubSenpai.
 * Isi chapter tetap dibaca lewat WebView (endpoint chapter API situs
 * mensyaratkan otorisasi aplikasi mereka; halaman HTML lolos dengan
 * pola F39).
 */
object ManhuarmtlApi {
    private const val BASE = "https://manhuarmtl.com/wp-json/manhuarmtl/v1"
    private const val UA =
        "Mozilla/5.0 (Linux; Android 13) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Mobile Safari/537.36"

    private val client = OkHttpClient.Builder()
        .connectTimeout(20, TimeUnit.SECONDS)
        .readTimeout(30, TimeUnit.SECONDS)
        .build()

    private suspend fun get(url: String): JSONObject = withContext(Dispatchers.IO) {
        val req = Request.Builder().url(url).header("User-Agent", UA).build()
        client.newCall(req).execute().use { resp ->
            val body = resp.body?.string() ?: ""
            if (!resp.isSuccessful) throw IllegalStateException("HTTP ${resp.code} dari sumber komik")
            JSONObject(body)
        }
    }

    private fun slugOf(url: String): String =
        url.trimEnd('/').substringAfterLast('/')

    private fun parseSeries(o: JSONObject, withSynopsis: Boolean): ComicSeries {
        val url = o.optString("mangaUrl")
        val genres = mutableListOf<String>()
        val g = o.optJSONArray("genres")
        if (g != null) {
            for (i in 0 until g.length()) {
                g.optJSONObject(i)?.optString("name")?.takeIf { it.isNotBlank() }?.let { genres.add(it) }
            }
        }
        if (genres.isEmpty()) {
            val gs = o.optJSONArray("genreSlugs")
            if (gs != null) {
                for (i in 0 until gs.length()) {
                    val s = gs.optString(i).replace('-', ' ')
                    if (s.isNotBlank()) genres.add(s.replaceFirstChar { it.uppercase() })
                }
            }
        }
        return ComicSeries(
            slug = slugOf(url),
            title = o.optString("title"),
            url = url,
            poster = o.optString("thumbnail").takeIf { it.isNotBlank() },
            type = o.optString("type"),
            status = o.optString("status").takeIf { it.isNotBlank() },
            year = o.optInt("year").takeIf { it > 0 },
            genres = genres,
            synopsis = if (withSynopsis) {
                o.optString("description").takeIf { it.isNotBlank() }
            } else {
                null
            },
            author = o.optString("author").takeIf { it.isNotBlank() },
            artist = o.optString("artist").takeIf { it.isNotBlank() },
            rating = o.optString("rating").takeIf { it.isNotBlank() },
        )
    }

    private fun parseList(root: JSONObject): Pair<List<ComicSeries>, Int> {
        val items = root.optJSONArray("items")
        val out = mutableListOf<ComicSeries>()
        if (items != null) {
            for (i in 0 until items.length()) {
                val o = items.optJSONObject(i) ?: continue
                val s = parseSeries(o, withSynopsis = false)
                if (s.title.isNotBlank() && s.url.isNotBlank()) out.add(s)
            }
        }
        return out to root.optInt("total_pages", 1).coerceAtLeast(1)
    }

    suspend fun latest(page: Int): Pair<List<ComicSeries>, Int> =
        parseList(get("$BASE/latest?page=$page"))

    suspend fun search(query: String, page: Int): Pair<List<ComicSeries>, Int> {
        val q = java.net.URLEncoder.encode(query, "UTF-8")
        return parseList(get("$BASE/search?q=$q&page=$page"))
    }

    private val chapterNoRe = Regex("""chapter[-_/ ](\d+(?:\.\d+)?)""", RegexOption.IGNORE_CASE)

    /** Detail seri + daftar chapter menaik (chapter 1 dulu). */
    suspend fun detail(seriesUrl: String): Pair<ComicSeries, List<ComicChapter>> {
        val enc = java.net.URLEncoder.encode(seriesUrl, "UTF-8")
        val root = get("$BASE/manga?url=$enc")
        val series = parseSeries(root, withSynopsis = true)
        val chapters = mutableListOf<ComicChapter>()
        val arr = root.optJSONArray("chapters")
        if (arr != null) {
            for (i in 0 until arr.length()) {
                val o = arr.optJSONObject(i) ?: continue
                val url = o.optString("url")
                val name = o.optString("name")
                if (url.isBlank()) continue
                val no = chapterNoRe.find(url)?.groupValues?.get(1)?.toDoubleOrNull()
                    ?: name.filter { it.isDigit() || it == '.' }.toDoubleOrNull()
                    ?: (i + 1).toDouble()
                chapters.add(ComicChapter(name.ifBlank { "Chapter ${no.toInt()}" }, url, no))
            }
        }
        chapters.sortBy { it.no }
        return series to chapters
    }
}
