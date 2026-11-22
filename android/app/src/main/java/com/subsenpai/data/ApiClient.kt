package com.subsenpai.data

import com.subsenpai.BuildConfig
import kotlinx.coroutines.Dispatchers
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

    private suspend fun ensureSession() {
        if (session.isReady) return
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
    }

    private fun isEnvelope(text: String): Boolean = try {
        JSONObject(text).has("xdata")
    } catch (e: Exception) {
        false
    }

    private suspend fun rawRequest(method: String, path: String, body: JSONObject?): Pair<Int, String> {
        val builder = Request.Builder()
            .url("$baseUrl$path")
            .header("X-Session-Id", session.id!!)
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
        ensureSession()
        var (code, text) = rawRequest(method, path, body)
        if (code == 401 && !isEnvelope(text)) {
            session.reset()
            ensureSession()
            val retry = rawRequest(method, path, body)
            code = retry.first
            text = retry.second
        }
        if (isEnvelope(text)) {
            val env = JSONObject(text)
            val plain = session.open(env.getString("xdata"), env.getLong("xtime"))
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
