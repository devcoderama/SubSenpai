package com.subsenpai.data

import android.content.Context
import android.net.Uri
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import okhttp3.OkHttpClient
import okhttp3.Request
import org.json.JSONArray
import java.io.File
import java.net.URLEncoder

/**
 * Cadangan terjemahan subtitle di perangkat (F40) — pola yang sama dengan
 * D TRADE: endpoint publik gratis Google Translate (client=gtx) dipanggil
 * langsung dari perangkat user (IP per user, jarang kena limit), teks
 * dipotong per ±1.200 karakter, timestamp tidak diubah, dan hasilnya
 * disimpan di cache lokal perangkat (kunci: episode + bahasa tujuan).
 *
 * Dipakai hanya bila bahasa user belum punya trek siap pakai dari sumber.
 * Gagal di titik mana pun → null (pemanggil menampilkan trek aslinya).
 */
class GtxTranslator(private val http: OkHttpClient = OkHttpClient()) {

    private data class Cue(val timing: String, val text: String)

    suspend fun translateSubtitle(
        context: Context,
        episodeId: String,
        sub: SubtitleItem,
        targetLang: String,
    ): Uri? = withContext(Dispatchers.IO) {
        val srcUrl = sub.fileUrl ?: return@withContext null
        val dir = File(context.cacheDir, "subs").apply { mkdirs() }
        val out = File(dir, "${episodeId}_$targetLang.srt")
        if (out.exists() && out.length() > 0) return@withContext Uri.fromFile(out)
        try {
            val raw = get(srcUrl) ?: return@withContext null
            val cues = parseCues(raw, sub.format)
            if (cues.isEmpty()) return@withContext null
            val translated = translateCues(cues.map { it.text }, targetLang) ?: return@withContext null
            val sb = StringBuilder()
            cues.forEachIndexed { i, cue ->
                sb.append(i + 1).append('\n')
                    .append(cue.timing).append('\n')
                    .append(translated.getOrElse(i) { cue.text }).append("\n\n")
            }
            out.writeText(sb.toString())
            Uri.fromFile(out)
        } catch (e: Exception) {
            null
        }
    }

    private fun get(url: String): String? {
        val req = Request.Builder().url(url)
            .header("User-Agent", "Mozilla/5.0 (Android) SubSenpai")
            .build()
        http.newCall(req).execute().use { resp ->
            if (!resp.isSuccessful) return null
            return resp.body?.string()
        }
    }

    /** Urai SRT/VTT menjadi daftar cue (baris timing + teks). */
    private fun parseCues(raw: String, format: String): List<Cue> {
        val cues = mutableListOf<Cue>()
        val blocks = raw.replace("\r\n", "\n").split(Regex("\n\n+"))
        for (block in blocks) {
            val lines = block.lines().map { it.trim() }.filter { it.isNotEmpty() }
            val tIdx = lines.indexOfFirst { it.contains("-->") }
            if (tIdx < 0) continue
            var timing = lines[tIdx]
            if (format == "vtt") {
                // VTT memakai titik untuk milidetik; SRT memakai koma.
                timing = timing.replace(Regex("""(\d{2}:\d{2}:\d{2})\.(\d{3})"""), "$1,$2")
                    .replace(Regex("""(\d{2}:\d{2})\.(\d{3})"""), "00:$1,$2")
            }
            val text = lines.drop(tIdx + 1).joinToString("\n")
            if (text.isNotBlank()) cues.add(Cue(timing, text))
        }
        return cues
    }

    /** Terjemahkan teks per cue; batch ±1.200 karakter dengan pemisah @@@. */
    private fun translateCues(texts: List<String>, targetLang: String): List<String>? {
        val result = MutableList(texts.size) { texts[it] }
        var i = 0
        while (i < texts.size) {
            val batch = mutableListOf<Int>()
            var len = 0
            while (i < texts.size && (len == 0 || len + texts[i].length <= 1200)) {
                batch.add(i)
                len += texts[i].length + 5
                i++
            }
            val joined = batch.joinToString("\n@@@\n") { texts[it].replace("\n", " ") }
            val translated = gtx(joined, targetLang) ?: return null
            val parts = translated.split("@@@").map { it.trim() }
            if (parts.size == batch.size) {
                batch.forEachIndexed { k, idx -> result[idx] = parts[k] }
            } else {
                // Pemisah rusak — terjemahkan satu per satu untuk batch ini.
                for (idx in batch) {
                    result[idx] = gtx(texts[idx].replace("\n", " "), targetLang) ?: return null
                }
            }
        }
        return result
    }

    /** Satu panggilan endpoint gtx; mengembalikan teks terjemahan utuh. */
    private fun gtx(text: String, targetLang: String): String? {
        val url = "https://translate.googleapis.com/translate_a/single?client=gtx&sl=auto&tl=" +
            URLEncoder.encode(targetLang, "UTF-8") +
            "&dt=t&q=" + URLEncoder.encode(text, "UTF-8")
        val body = get(url) ?: return null
        return try {
            val root = JSONArray(body)
            val segments = root.getJSONArray(0)
            val sb = StringBuilder()
            for (i in 0 until segments.length()) {
                val seg = segments.getJSONArray(i)
                if (!seg.isNull(0)) sb.append(seg.getString(0))
            }
            sb.toString()
        } catch (e: Exception) {
            null
        }
    }
}
