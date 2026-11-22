package com.subsenpai.data

import android.graphics.Bitmap
import android.graphics.BitmapFactory
import android.graphics.Canvas
import android.graphics.Color
import android.graphics.Paint
import android.graphics.RectF
import android.text.Layout
import android.text.StaticLayout
import android.text.TextPaint
import android.util.Base64
import android.webkit.CookieManager
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import org.json.JSONObject
import java.security.MessageDigest
import java.util.concurrent.TimeUnit
import javax.crypto.Cipher
import javax.crypto.spec.GCMParameterSpec
import javax.crypto.spec.SecretKeySpec

/**
 * Pembaca chapter manhuarmtl di perangkat — port protokol reader
 * "v9" yang sama dengan yang dipakai ekstensi Manhuarm di Mihon:
 * halaman chapter TIDAK berisi gambar/teks langsung. Halaman memuat
 * parameter sekali pakai (`const _v9` + garam DOM/CSS + fungsi
 * tantangan JS) yang diturunkan menjadi kunci sesi; server lalu
 * memberi manifes halaman + kotak dialog OCR lewat init terenkripsi
 * AES-GCM, dan setiap gambar halaman dikirim terenkripsi &
 * teracak 16 potong untuk tiketnya sendiri. Teks gelembung memang
 * terpisah dari gambar (teks Inggris dari situs) — aplikasi
 * menggambarnya kembali ke gambar pada koordinat kotaknya, dan
 * tombol Terjemahkan menggantinya dengan hasil Google Translate
 * di perangkat (pola ekstensi "Manhuarm (ID)").
 */

/** Satu kotak dialog: koordinat piksel pada gambar halaman + teks. */
class ManhuarmDialog(val x: Float, val y: Float, val w: Float, val h: Float, val text: String)

/** Satu entri manifes halaman chapter. */
class ManhuarmPage(
    val index: Int,
    val token: String,
    val ticket: String,
    val imageKey: String,
    val imageSeed: Long,
)

/** Sesi baca satu chapter (parameter halaman bersifat sekali pakai). */
class ManhuarmSession internal constructor(
    val chapterUrl: String,
    internal val keys: ReaderKeys,
    val pages: List<ManhuarmPage>,
    val dialogues: Map<String, List<ManhuarmDialog>>,
) {
    fun dialogsOf(page: ManhuarmPage): List<ManhuarmDialog> = dialogues[page.token].orEmpty()
}

private const val UINT32 = 0xFFFFFFFFL

private fun sha256Bytes(text: String): ByteArray =
    MessageDigest.getInstance("SHA-256").digest(text.toByteArray(Charsets.UTF_8))

private fun sha256Hex(text: String): String =
    sha256Bytes(text).joinToString("") { "%02x".format(it) }

private fun aesGcmDecrypt(key: ByteArray, iv: ByteArray, ctWithTag: ByteArray): ByteArray {
    val cipher = Cipher.getInstance("AES/GCM/NoPadding")
    cipher.init(Cipher.DECRYPT_MODE, SecretKeySpec(key, "AES"), GCMParameterSpec(128, iv))
    return cipher.doFinal(ctWithTag)
}

private fun xorShift32(seed: Long): () -> Long {
    var state = (seed and UINT32).takeIf { it != 0L } ?: 1L
    return {
        state = state xor (state shl 13 and UINT32)
        state = state xor (state ushr 17)
        state = state xor (state shl 5 and UINT32)
        state
    }
}

/** Garam kanvas: 24 goresan warna datar pada kanvas 16x1, di-hash. */
private fun canvasSalt(seed: Long): Long {
    val next = xorShift32(seed)
    val cells = IntArray(16)
    repeat(24) {
        val shift = (next() % 4).toInt()
        val x = (next() % (16 - shift)).toInt() + shift
        val width = (next() % 3).toInt() + 1
        val cell = ((next() % 4) shl 4 or ((next() % 4) shl 2) or (next() % 4)).toInt()
        for (pixel in x until minOf(x + width, 16)) cells[pixel] = cell
    }
    return cells.fold(0x811C9DC5L) { hash, cell -> (hash * 1664525L + cell + 1013904223L) and UINT32 }
}

/** Susun kembali gambar: potongan i hasil acakan adalah potongan order[i] asli; sisa tetap di akhir. */
private fun unshuffle(bytes: ByteArray, seed: Long): ByteArray {
    val next = xorShift32(seed)
    val order = IntArray(16) { it }
    for (i in 15 downTo 1) {
        val j = (next() % (i + 1)).toInt()
        val tmp = order[i]; order[i] = order[j]; order[j] = tmp
    }
    val source = IntArray(16)
    order.forEachIndexed { index, target -> source[target] = index }
    val chunk = bytes.size / 16
    val out = ByteArray(bytes.size)
    var pos = 0
    for (s in source) {
        System.arraycopy(bytes, s * chunk, out, pos, chunk)
        pos += chunk
    }
    System.arraycopy(bytes, 16 * chunk, out, pos, bytes.size - 16 * chunk)
    return out
}

/**
 * Penerjemah fungsi tantangan halaman: pernyataan lurus atas garam,
 * status tiga kata `r`, dan Math.imul — semua aritmetika uint32.
 * Ditulis ulang situs pada setiap muatan halaman.
 */
private class Challenge(salts: Map<String, Long>) {
    private val variables = salts.mapValuesTo(HashMap()) { it.value and UINT32 }
    private val state = LongArray(3)

    fun run(body: String): Long {
        for (statement in body.split(';')) {
            val text = statement.trim().removePrefix("var ")
            if (text.startsWith("return ")) return Expression(text.removePrefix("return ")).value()
            topLevel(text).forEach(::assign)
        }
        error("Tantangan tidak punya return")
    }

    private fun assign(part: String) {
        if (IDENTIFIER_REGEX.matches(part)) return
        val m = ASSIGNMENT_REGEX.matchEntire(part) ?: error("Pernyataan tantangan tak didukung: $part")
        val target = m.groupValues[1]
        val value = m.groupValues[2]
        when {
            target == "r" -> topLevel(value.trim().removeSurrounding("[", "]"))
                .forEachIndexed { i, item -> state[i] = Expression(item).value() }
            target.startsWith("r[") -> state[target[2].digitToInt()] = Expression(value).value()
            else -> variables[target] = Expression(value).value()
        }
    }

    private fun topLevel(text: String): List<String> = buildList {
        var depth = 0
        var start = 0
        text.forEachIndexed { i, char ->
            when (char) {
                '(', '[' -> depth++
                ')', ']' -> depth--
                ',' -> if (depth == 0) {
                    add(text.substring(start, i).trim())
                    start = i + 1
                }
            }
        }
        text.substring(start).trim().takeIf(String::isNotEmpty)?.let(::add)
    }

    private inner class Expression(text: String) {
        private val tokens = TOKEN_REGEX.findAll(text).map { it.value }.toList()
        private var position = 0

        init {
            check(tokens.sumOf(String::length) == text.count { !it.isWhitespace() }) {
                "Ekspresi tantangan tak didukung"
            }
        }

        fun value(): Long = or().also { check(position == tokens.size) { "Ekspresi tantangan tak didukung" } }

        private fun peek() = tokens.getOrNull(position)

        private fun take(expected: String? = null): String = tokens[position++].also {
            check(expected == null || it == expected) { "Ekspresi tantangan tak didukung" }
        }

        private fun or(): Long {
            var value = xor()
            while (peek() == "|") { take(); value = value or xor() }
            return value
        }

        private fun xor(): Long {
            var value = and()
            while (peek() == "^") { take(); value = value xor and() }
            return value
        }

        private fun and(): Long {
            var value = shift()
            while (peek() == "&") { take(); value = value and shift() }
            return value
        }

        private fun shift(): Long {
            var value = additive()
            while (peek() == "<<" || peek() == ">>>") {
                val operator = take()
                val amount = (additive() and 31).toInt()
                value = if (operator == "<<") value shl amount and UINT32 else value ushr amount
            }
            return value
        }

        private fun additive(): Long {
            var value = unary()
            while (peek() == "+" || peek() == "-") {
                val operator = take()
                val operand = unary()
                value = (if (operator == "+") value + operand else value - operand) and UINT32
            }
            return value
        }

        private fun unary(): Long = if (peek() == "-") {
            take()
            -unary() and UINT32
        } else {
            primary()
        }

        private fun primary(): Long {
            val token = take()
            return when {
                token == "Math.imul" -> {
                    take("(")
                    val left = or()
                    take(",")
                    val right = or()
                    take(")")
                    left * right and UINT32
                }
                token == "r" -> {
                    take("[")
                    state[or().toInt()].also { take("]") }
                }
                token[0].isDigit() -> token.toLong() and UINT32
                else -> variables[token] ?: error("Variabel tantangan tak dikenal: $token")
            }
        }
    }

    companion object {
        private val TOKEN_REGEX = Regex("""Math\.imul|[A-Za-z_$][\w$]*|\d+|>>>|<<|[-+^|&()\[\],]""")
        private val IDENTIFIER_REGEX = Regex("""[A-Za-z_$][\w$]*""")
        private val ASSIGNMENT_REGEX = Regex("""(r\[\d]|r|[A-Za-z_$][\w$]*)\s*=(.*)""", RegexOption.DOT_MATCHES_ALL)
    }
}

/** Kunci sesi yang diturunkan dari parameter sekali pakai halaman chapter. */
class ReaderKeys private constructor(
    private val payload: String,
    private val timestamp: Long,
    private val nonce: String,
    private val seed: String,
    private val signature: String,
    val endpoint: String,
    private val answer: Long,
    private val splitKey: String,
) {
    /** Geseran yang dikurangkan server dari setiap kotak OCR. */
    val boxOffset = intArrayOf(
        (answer and 0xFF).toInt() + 19,
        (answer shr 8 and 0xFF).toInt() + 37,
        (answer shr 16 and 0xFF).toInt() + 13,
        (answer shr 24 and 0xFF).toInt() + 29,
    )

    private fun proof(text: String) = sha256Hex(text)

    private fun baseHeaders(proof: String): Map<String, String> = mapOf(
        "X-Requested-With" to "XMLHttpRequest",
        "X-Mrm-Sig" to signature,
        "X-Mrm-Nonce" to nonce,
        "X-Mrm-Seed" to seed,
        "X-Mrm-Ts" to timestamp.toString(),
        "X-Mrm-Proof" to proof,
    )

    fun initHeaders(): Map<String, String> =
        baseHeaders(proof("$answer|$nonce|$seed|$splitKey|v9-chk"))

    fun imageHeaders(page: ManhuarmPage): Map<String, String> =
        baseHeaders(proof("$answer|${page.imageKey}|${page.imageSeed}|$nonce|v9-img"))

    fun initBody(): String = JSONObject().put("m", "init").put("p", payload).toString()

    fun imageBody(page: ManhuarmPage): String =
        JSONObject().put("m", "img").put("tk", page.ticket).toString()

    /** Buka segel init: AES-256-GCM, kunci = sha256 string konteks sesi. */
    fun decryptInit(sealed: JSONObject): JSONObject {
        val key = sha256Bytes("v9|$seed|$answer|$splitKey|$nonce|$timestamp")
        val iv = Base64.decode(sealed.getString("u"), Base64.DEFAULT)
        val data = Base64.decode(sealed.getString("v"), Base64.DEFAULT)
        val tag = Base64.decode(sealed.getString("w"), Base64.DEFAULT)
        return JSONObject(String(aesGcmDecrypt(key, iv, data + tag), Charsets.UTF_8))
    }

    /** Dekripsi gambar halaman: [IV 12][tag 16][ciphertext 16 potong teracak]. */
    fun decryptImage(body: ByteArray, page: ManhuarmPage): ByteArray {
        val key = sha256Bytes("v9-ib|$answer|${page.imageKey}|${page.imageSeed}|$splitKey|$nonce")
        val iv = body.copyOfRange(0, 12)
        val tag = body.copyOfRange(12, 28)
        val ct = body.copyOfRange(28, body.size)
        return unshuffle(aesGcmDecrypt(key, iv, ct + tag), answer xor page.imageSeed)
    }

    companion object {
        private val SCRIPT_RE = Regex("""<script[^>]*>(.*?)</script>""", setOf(RegexOption.DOT_MATCHES_ALL, RegexOption.IGNORE_CASE))
        private val STYLE_RE = Regex("""<style[^>]*>(.*?)</style>""", setOf(RegexOption.DOT_MATCHES_ALL, RegexOption.IGNORE_CASE))
        private val PARAMS_REGEX = Regex("""const _v9\s*=\s*(\{.*?\});""", RegexOption.DOT_MATCHES_ALL)
        private val CHALLENGE_REGEX = Regex("""function [\w$]+\(ds,cs,cvs,rkh,rkl\)\{([^}]*)\}""")
        private val CSS_RULE_REGEX = Regex("""#mrm-c1\s*\{([^}]*)\}""")
        private val Z_INDEX_REGEX = Regex("""z-index:\s*(\d+)""")
        private val CSS_KEY_REGEX = Regex("""--mrm-k1:\s*["']([0-9a-fA-F]{8})["']""")
        private val ATTR_RE = Regex("""([\w-]+)\s*=\s*["']([^"']*)["']""")

        private fun tagAttrs(html: String, id: String): Map<String, String> {
            val tag = Regex("""<[a-zA-Z][^>]*\bid=["']$id["'][^>]*>""", RegexOption.IGNORE_CASE)
                .find(html)?.value ?: error("Elemen #$id tidak ditemukan")
            return ATTR_RE.findAll(tag).associate { it.groupValues[1] to it.groupValues[2] }
        }

        fun fromHtml(html: String): ReaderKeys {
            val script = SCRIPT_RE.findAll(html).map { it.groupValues[1] }
                .firstOrNull { "const _v9" in it } ?: error("Parameter reader tidak ditemukan")
            val params = JSONObject(PARAMS_REGEX.find(script)?.groupValues?.get(1) ?: error("Parameter _v9 rusak"))
            // Aturan CSS terakhir yang menang, sama seperti di browser.
            val rule = STYLE_RE.findAll(html).flatMap { CSS_RULE_REGEX.findAll(it.groupValues[1]) }
                .lastOrNull()?.groupValues?.get(1) ?: error("Aturan #mrm-c1 tidak ditemukan")
            val d1 = tagAttrs(html, "mrm-d1")
            val d2 = tagAttrs(html, "mrm-d2")
            val domSalt = (d1.getValue("data-u").toLong(16) and 0xFFFF) shl 16 or
                (d2.getValue("data-v").toLong(16) and 0xFFFF)
            val splitKey = (CSS_KEY_REGEX.find(rule)?.groupValues?.get(1) + d2.getValue("data-w")).lowercase()
            val salts = mapOf(
                "ds" to domSalt,
                "cs" to (Z_INDEX_REGEX.find(rule)?.groupValues?.get(1)?.toLong() ?: error("z-index tidak ditemukan")),
                "cvs" to canvasSalt(params.getLong("cs")),
                "rkh" to splitKey.substring(0, 8).toLong(16),
                "rkl" to splitKey.substring(8, 16).toLong(16),
            )
            val answer = Challenge(salts).run(
                CHALLENGE_REGEX.find(script)?.groupValues?.get(1) ?: error("Fungsi tantangan tidak ditemukan"),
            )
            return ReaderKeys(
                payload = params.getString("p"),
                timestamp = params.getLong("t"),
                nonce = params.getString("n"),
                seed = params.getString("s"),
                signature = params.getString("g"),
                endpoint = params.getString("u"),
                answer = answer,
                splitKey = splitKey,
            )
        }
    }
}

/** Klien sesi baca: init terenkripsi + unduh gambar per tiket. */
object ManhuarmReader {
    private const val FALLBACK_UA =
        "Mozilla/5.0 (Linux; Android 13) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Mobile Safari/537.36"

    private val client = OkHttpClient.Builder()
        .connectTimeout(20, TimeUnit.SECONDS)
        .readTimeout(40, TimeUnit.SECONDS)
        .build()

    private fun post(url: String, chapterUrl: String, ua: String, headers: Map<String, String>, body: String): ByteArray {
        val cookie = CookieManager.getInstance().getCookie(chapterUrl) ?: ""
        val builder = Request.Builder().url(url)
            .header("User-Agent", ua.ifBlank { FALLBACK_UA })
            .header("Referer", chapterUrl)
            .header("Origin", "https://manhuarmtl.com")
            .header("Cookie", cookie)
            .post(body.toRequestBody("application/json".toMediaType()))
        headers.forEach { (k, v) -> builder.header(k, v) }
        client.newCall(builder.build()).execute().use { resp ->
            val bytes = resp.body?.bytes() ?: ByteArray(0)
            if (!resp.isSuccessful) throw IllegalStateException("HTTP ${resp.code} dari reader sumber")
            return bytes
        }
    }

    /** Ambil HTML halaman chapter lewat HTTP biasa (kuki dari WebView). */
    suspend fun fetchChapterHtml(chapterUrl: String, ua: String): String = withContext(Dispatchers.IO) {
        val cookie = CookieManager.getInstance().getCookie(chapterUrl) ?: ""
        val req = Request.Builder().url(chapterUrl)
            .header("User-Agent", ua.ifBlank { FALLBACK_UA })
            .header("Cookie", cookie)
            .build()
        client.newCall(req).execute().use { resp ->
            if (!resp.isSuccessful) throw IllegalStateException("HTTP ${resp.code} halaman chapter")
            resp.body?.string() ?: ""
        }
    }

    /** Buka sesi chapter dari HTML halamannya (parameter sekali pakai). */
    suspend fun openChapter(html: String, chapterUrl: String, ua: String): ManhuarmSession =
        withContext(Dispatchers.IO) {
            val keys = ReaderKeys.fromHtml(html)
            val sealed = JSONObject(
                String(post(keys.endpoint, chapterUrl, ua, keys.initHeaders(), keys.initBody()), Charsets.UTF_8),
            )
            val data = keys.decryptInit(sealed)
            val pages = mutableListOf<ManhuarmPage>()
            val m = data.getJSONArray("m")
            for (i in 0 until m.length()) {
                val o = m.getJSONObject(i)
                pages.add(
                    ManhuarmPage(
                        index = o.getInt("i"),
                        token = o.getString("k"),
                        ticket = o.getString("tk"),
                        imageKey = o.getString("ik"),
                        imageSeed = o.getLong("is"),
                    ),
                )
            }
            pages.sortBy { it.index }
            val off = keys.boxOffset
            val dialogues = HashMap<String, List<ManhuarmDialog>>()
            val d = data.getJSONArray("d")
            for (i in 0 until d.length()) {
                val page = d.getJSONObject(i)
                val boxes = mutableListOf<ManhuarmDialog>()
                val b = page.getJSONArray("b")
                for (j in 0 until b.length()) {
                    val box = b.getJSONObject(j)
                    val r = box.getJSONArray("r")
                    boxes.add(
                        ManhuarmDialog(
                            x = (r.getDouble(0) - off[0]).toFloat(),
                            y = (r.getDouble(1) - off[1]).toFloat(),
                            w = (r.getDouble(2) - off[2]).toFloat(),
                            h = (r.getDouble(3) - off[3]).toFloat(),
                            text = box.optString("c"),
                        ),
                    )
                }
                dialogues[page.getString("f")] = boxes
            }
            ManhuarmSession(chapterUrl, keys, pages, dialogues)
        }

    /** Unduh + dekripsi + susun ulang satu halaman gambar. */
    suspend fun fetchImage(session: ManhuarmSession, page: ManhuarmPage, ua: String): ByteArray =
        withContext(Dispatchers.IO) {
            val body = post(
                session.keys.endpoint, session.chapterUrl, ua,
                session.keys.imageHeaders(page), session.keys.imageBody(page),
            )
            session.keys.decryptImage(body, page)
        }

    /**
     * Gambar teks dialog ke bitmap halaman pada koordinat kotaknya —
     * teks hitam di atas latar putih, ukuran huruf menyusut sampai
     * muat di kotak (cara yang sama dengan penyusun gambar ekstensi).
     */
    fun composePage(imageBytes: ByteArray, boxes: List<Pair<ManhuarmDialog, String>>): Bitmap? {
        val decoded = BitmapFactory.decodeByteArray(imageBytes, 0, imageBytes.size) ?: return null
        val filled = boxes.filter { it.second.isNotBlank() && it.first.w > 4 && it.first.h > 4 }
        if (filled.isEmpty()) return decoded
        val bmp = if (decoded.config == Bitmap.Config.ARGB_8888 && decoded.isMutable) {
            decoded
        } else {
            decoded.copy(Bitmap.Config.ARGB_8888, true) ?: return decoded
        }
        val canvas = Canvas(bmp)
        val bg = Paint().apply { color = Color.WHITE }
        for ((box, text) in filled) {
            val paint = TextPaint().apply {
                color = Color.BLACK
                isAntiAlias = true
                typeface = android.graphics.Typeface.DEFAULT_BOLD
                textSize = (box.h / 2.2f).coerceIn(10f, 34f)
            }
            val width = box.w.toInt().coerceAtLeast(10)
            fun layout() = StaticLayout.Builder.obtain(text, 0, text.length, paint, width)
                .setAlignment(Layout.Alignment.ALIGN_CENTER)
                .setLineSpacing(0f, 1.05f)
                .build()
            var lay = layout()
            var guard = 0
            while (lay.height > box.h && paint.textSize > 6f && guard++ < 40) {
                paint.textSize -= 1f
                lay = layout()
            }
            val y = box.y + (box.h - lay.height) / 2f
            canvas.drawRect(RectF(box.x - 1f, y - 1f, box.x + width + 1f, y + lay.height + 1f), bg)
            canvas.save()
            canvas.translate(box.x, y)
            lay.draw(canvas)
            canvas.restore()
        }
        return bmp
    }
}
