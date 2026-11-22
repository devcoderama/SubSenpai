package com.subsenpai.data

import com.subsenpai.BuildConfig
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.Response
import okhttp3.WebSocket
import okhttp3.WebSocketListener
import org.json.JSONObject
import java.util.concurrent.TimeUnit

/**
 * Koneksi WebSocket chat & watch party ke backend (/ws).
 * Kejadian masuk diteruskan sebagai JSONObject ke onEvent.
 */
class ChatSocket(private val token: String) {
    private val http = OkHttpClient.Builder()
        .pingInterval(25, TimeUnit.SECONDS)
        .build()
    private var ws: WebSocket? = null

    var onEvent: ((JSONObject) -> Unit)? = null
    var onClosed: (() -> Unit)? = null

    fun connect() {
        val url = BuildConfig.API_BASE_URL.replaceFirst("http", "ws") + "/ws"
        val req = Request.Builder()
            .url(url)
            .header("Authorization", "Bearer $token")
            .build()
        ws = http.newWebSocket(req, object : WebSocketListener() {
            override fun onMessage(webSocket: WebSocket, text: String) {
                try {
                    onEvent?.invoke(JSONObject(text))
                } catch (e: Exception) {
                    // Abaikan bingkai tak terbaca.
                }
            }

            override fun onFailure(webSocket: WebSocket, t: Throwable, response: Response?) {
                onClosed?.invoke()
            }

            override fun onClosed(webSocket: WebSocket, code: Int, reason: String) {
                onClosed?.invoke()
            }
        })
    }

    private fun send(obj: JSONObject) {
        ws?.send(obj.toString())
    }

    fun join(channelId: String) {
        send(JSONObject().put("type", "join").put("channel_id", channelId))
    }

    fun sendMessage(channelId: String, body: String, replyToId: String?) {
        val o = JSONObject()
            .put("type", "message")
            .put("channel_id", channelId)
            .put("body", body)
        if (replyToId != null) o.put("reply_to_id", replyToId)
        send(o)
    }

    fun partyCreate(episodeId: String) {
        send(JSONObject().put("type", "party_create").put("episode_id", episodeId))
    }

    fun partyJoin(code: String) {
        send(JSONObject().put("type", "party_join").put("code", code))
    }

    fun partySync(code: String, action: String, positionSec: Int) {
        send(
            JSONObject()
                .put("type", "party_sync")
                .put("code", code)
                .put("action", action)
                .put("position_sec", positionSec)
        )
    }

    fun close() {
        ws?.close(1000, null)
        ws = null
    }
}
