package com.subsenpai.data

/** Satu chapter hasil uraian halaman sumber di perangkat (F39). */
data class ExtChapter(val no: Double, val label: String, val url: String)

/**
 * Pengurai HTML sumber komik ber-Cloudflare (F39) — berjalan di perangkat
 * setelah user melewati verifikasi di WebView. Heuristik umum situs komik
 * (tema Madara dan sejenisnya): tautan chapter mengandung "chapter",
 * halaman adalah <img> berekstensi gambar pada isi chapter.
 */
object DeviceSourceParser {

    private val chapterNoRe = Regex("""chapter[-_/ ]?(\d+(?:[.,]\d+)?)""", RegexOption.IGNORE_CASE)
    private val anchorRe = Regex("""<a[^>]+href=["'](https?://[^"']+)["'][^>]*>(.*?)</a>""", RegexOption.IGNORE_CASE)
    private val imgRe = Regex(
        """(?:data-src|data-lazy-src|src)\s*=\s*["'](https?://[^"']+\.(?:jpg|jpeg|png|webp)(?:\?[^"']*)?)["']""",
        RegexOption.IGNORE_CASE,
    )
    private val tagRe = Regex("""<[^>]+>""")

    /** Daftar chapter dari HTML halaman seri, terurut nomor menaik. */
    fun parseChapters(html: String): List<ExtChapter> {
        val byUrl = LinkedHashMap<String, ExtChapter>()
        for (m in anchorRe.findAll(html)) {
            val url = m.groupValues[1]
            if (!url.contains("chapter", ignoreCase = true)) continue
            val noText = chapterNoRe.find(url)?.groupValues?.get(1)
                ?: chapterNoRe.find(m.groupValues[2])?.groupValues?.get(1)
                ?: continue
            val no = noText.replace(',', '.').toDoubleOrNull() ?: continue
            val label = m.groupValues[2].replace(tagRe, " ").replace(Regex("""\s+"""), " ").trim()
                .ifBlank { "Chapter $noText" }
            byUrl.putIfAbsent(url, ExtChapter(no, label.take(80), url))
        }
        return byUrl.values.sortedBy { it.no }
    }

    /** URL halaman gambar dari HTML halaman chapter (urut dokumen). */
    fun parsePages(html: String): List<String> {
        val out = LinkedHashSet<String>()
        for (m in imgRe.findAll(html)) {
            val url = m.groupValues[1]
            val low = url.lowercase()
            if (low.contains("logo") || low.contains("avatar") || low.contains("banner") ||
                low.contains("icon") || low.contains("ads")
            ) {
                continue
            }
            out.add(url)
        }
        return out.toList()
    }
}

/** Penampung navigasi reader: daftar chapter judul yang sedang dibuka. */
object ReaderNav {
    var titleId: String = ""
    var titleType: String = ""
    var chapters: List<ChapterItem> = emptyList()
    var detail: TitleDetail? = null
}
