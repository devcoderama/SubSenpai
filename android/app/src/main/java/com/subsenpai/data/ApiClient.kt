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

/**
 * Klien API backend Go. Semua panggilan API (selain handshake)
 * membawa header X-Session-Id dan response-nya berupa envelope xdata
 * yang dibuka di sini sebelum dikembalikan sebagai JSONObject.
 */
class ApiClient(
    private val baseUrl: String = BuildConfig.API_BASE_URL,
    private val session: XdataSession = XdataSession(),
) {
    private val http = OkHttpClient.Builder()
        .connectTimeout(15, TimeUnit.SECONDS)
        .readTimeout(30, TimeUnit.SECONDS)
        .build()

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

    /** GET terenkripsi; mengembalikan JSON asal yang sudah dibuka. */
    suspend fun get(path: String): JSONObject {
        ensureSession()
        val req = Request.Builder()
            .url("$baseUrl$path")
            .header("X-Session-Id", session.id!!)
            .build()
        val resp = withContext(Dispatchers.IO) { http.newCall(req).execute() }
        val text = resp.body?.string().orEmpty()
        if (resp.code == 401) {
            // Sesi kedaluwarsa di server — handshake ulang sekali.
            session.reset()
            ensureSession()
            return get(path)
        }
        check(resp.isSuccessful) { "GET $path gagal: HTTP ${resp.code}" }
        val env = JSONObject(text)
        return JSONObject(session.open(env.getString("xdata"), env.getLong("xtime")))
    }
}
