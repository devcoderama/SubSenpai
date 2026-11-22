package com.subsenpai.data

import com.subsenpai.BuildConfig
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.coroutines.withContext
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import org.json.JSONObject
import java.util.concurrent.TimeUnit

/** Kesalahan API dengan kode HTTP dari backend (setelah envelope dibuka). */
class ApiException(val code: Int, message: String) : Exception(message)

/**
 * Klien API backend Go. Semua panggilan API (selain handshake)
 * membawa header X-Session-Id dan response-nya berupa envelope xdata
 * yang dibuka di sini sebelum dikembalikan sebagai JSONObject.
 * Bila user sudah login, JWT backend ikut sebagai Authorization: Bearer.
 */
class ApiClient(
    private val baseUrl: String = BuildConfig.API_BASE_URL,
    private val session: XdataSession = XdataSession(),
) {
    private val http = OkHttpClient.Builder()
        .connectTimeout(15, TimeUnit.SECONDS)
        .readTimeout(30, TimeUnit.SECONDS)
        .build()

    /** Penyedia JWT backend; diisi CatalogRepository dari AuthStore. */
    var tokenProvider: (() -> String?)? = null

    /** Mengunci handshake agar hanya satu yang berjalan pada satu waktu. */
    private val sessionMutex = Mutex()

    /**
     * Pastikan sesi xdata ada dan kembalikan snapshot-nya. Banyak
     * permintaan berjalan bersamaan (Beranda memuat semua baris
     * sekaligus): tanpa kunci, dua handshake bisa saling menimpa dan
     * respons yang dienkripsi kunci lama gagal dibuka (BAD_DECRYPT).
     */
    private suspend fun ensureSession(): XdataSession.Snapshot {
        session.snapshot()?.let { return it }
        return sessionMutex.withLock {
            session.snapshot()?.let { return@withLock it }
            val prepared = session.prepare()
            val body = JSONObject().put("client_public_key", prepared.sendPublicKeyB64)
            val req = Request.Builder()
                .url("$baseUrl/api/v1/session")
                .post(body.toString().toRequestBody("application/json".toMediaType()))
                .build()
            val resp = withContext(Dispatchers.IO) { http.newCall(req).execute() }
            val text = resp.body?.string().orEmpty()
            check(resp.isSuccessful) { "handshake gagal: HTTP ${resp.code}" }
            val json = JSONObject(text)
            session.complete(prepared, json.getString("session_id"), json.getString("server_public_key"))
            session.snapshot() ?: error("sesi tidak terbentuk")
        }
    }

    private fun isEnvelope(text: String): Boolean = try {
        JSONObject(text).has("xdata")
    } catch (e: Exception) {
        false
    }

    private suspend fun rawRequest(
        method: String,
        path: String,
        body: JSONObject?,
        snap: XdataSession.Snapshot,
    ): Pair<Int, String> {
        val builder = Request.Builder()
            .url("$baseUrl$path")
            .header("X-Session-Id", snap.id)
        tokenProvider?.invoke()?.let { builder.header("Authorization", "Bearer $it") }
        when (method) {
            "POST" -> builder.post((body ?: JSONObject()).toString().toRequestBody("application/json".toMediaType()))
            "DELETE" -> builder.delete()
            else -> builder.get()
        }
        val resp = withContext(Dispatchers.IO) { http.newCall(builder.build()).execute() }
        return resp.code to resp.body?.string().orEmpty()
    }

    /**
     * Satu panggilan API terenkripsi. 401 polos (bukan envelope) berarti
     * sesi xdata kedaluwarsa → handshake ulang sekali; 401 dari dalam
     * envelope adalah penolakan auth dan diteruskan sebagai ApiException.
     */
    suspend fun request(method: String, path: String, body: JSONObject? = null): JSONObject {
        var snap = ensureSession()
        var (code, text) = rawRequest(method, path, body, snap)
        if (code == 401 && !isEnvelope(text)) {
            session.resetIfCurrent(snap)
            snap = ensureSession()
            val retry = rawRequest(method, path, body, snap)
            code = retry.first
            text = retry.second
        }
        if (isEnvelope(text)) {
            val env = JSONObject(text)
            val plain = session.open(snap, env.getString("xdata"), env.getLong("xtime"))
            val json = JSONObject(plain)
            if (code !in 200..299) {
                throw ApiException(code, json.optString("error").ifBlank { "HTTP $code" })
            }
            return json
        }
        if (code !in 200..299) throw ApiException(code, text.ifBlank { "HTTP $code" })
        return JSONObject(text.ifBlank { "{}" })
    }

    suspend fun get(path: String): JSONObject = request("GET", path)

    suspend fun post(path: String, body: JSONObject? = null): JSONObject = request("POST", path, body)

    suspend fun delete(path: String): JSONObject = request("DELETE", path)
}
