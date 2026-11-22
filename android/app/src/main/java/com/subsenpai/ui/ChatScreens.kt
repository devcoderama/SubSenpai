package com.subsenpai.ui

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp
import com.subsenpai.data.CatalogRepository
import com.subsenpai.data.ChatChannelItem
import com.subsenpai.data.ChatMsg
import com.subsenpai.data.ChatSocket
import org.json.JSONObject

/** Daftar kanal chat: topik global, grup, dan DM. */
@Composable
fun ChatListScreen(
    repo: CatalogRepository,
    onOpenRoom: (String, String) -> Unit,
    onOpenFriends: () -> Unit,
    onOpenGroups: () -> Unit,
    onBack: () -> Unit,
) {
    var channels by remember { mutableStateOf<List<ChatChannelItem>>(emptyList()) }
    var error by remember { mutableStateOf<String?>(null) }

    LaunchedEffect(Unit) {
        try {
            channels = repo.chatChannels()
        } catch (e: Exception) {
            error = e.message
        }
    }

    LazyColumn(
        modifier = Modifier.fillMaxSize().padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        item { TextButton(onClick = onBack) { Text("← Kembali") } }
        item { Text("Chat", style = MaterialTheme.typography.headlineSmall) }
        item {
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Button(onClick = onOpenFriends) { Text("Teman") }
                Button(onClick = onOpenGroups) { Text("Grup") }
            }
        }
        if (error != null) {
            item { Text("Gagal memuat: $error", color = MaterialTheme.colorScheme.error) }
        }
        items(channels) { c ->
            Card(modifier = Modifier.fillMaxWidth().clickable { onOpenRoom(c.id, c.name) }) {
                Column(modifier = Modifier.padding(14.dp)) {
                    Text(c.name, style = MaterialTheme.typography.titleMedium)
                    Text(
                        when (c.kind) {
                            "global" -> "Kanal topik"
                            "group" -> "Grup"
                            else -> "Pesan langsung"
                        },
                        style = MaterialTheme.typography.bodySmall,
                    )
                }
            }
        }
    }
}

/** Ruang chat: riwayat REST + pesan langsung WebSocket, dukung balasan. */
@Composable
fun ChatRoomScreen(
    repo: CatalogRepository,
    channelId: String,
    channelName: String,
    onBack: () -> Unit,
) {
    val token = repo.auth?.token
    var messages by remember { mutableStateOf<List<ChatMsg>>(emptyList()) }
    var input by remember { mutableStateOf("") }
    var replyTo by remember { mutableStateOf<ChatMsg?>(null) }
    var error by remember { mutableStateOf<String?>(null) }
    var socket by remember { mutableStateOf<ChatSocket?>(null) }

    LaunchedEffect(channelId) {
        try {
            messages = repo.chatHistory(channelId)
        } catch (e: Exception) {
            error = e.message
        }
    }
    DisposableEffect(channelId, token) {
        val s = if (token != null) ChatSocket(token) else null
        socket = s
        if (s != null) {
            s.onEvent = { ev ->
                when (ev.optString("type")) {
                    "message" -> {
                        val m = ev.optJSONObject("message")
                        if (m != null) {
                            val msg = ChatMsg.fromJson(m)
                            if (messages.none { it.id == msg.id }) {
                                messages = messages + msg
                            }
                        }
                    }
                    "error" -> error = ev.optString("error")
                }
            }
            s.connect()
            s.join(channelId)
        }
        onDispose { s?.close() }
    }

    Column(modifier = Modifier.fillMaxSize().padding(16.dp)) {
        TextButton(onClick = onBack) { Text("← Kembali") }
        Text(channelName, style = MaterialTheme.typography.headlineSmall)
        if (error != null) {
            Text(error!!, color = MaterialTheme.colorScheme.error)
            Spacer(Modifier.height(4.dp))
        }
        LazyColumn(
            modifier = Modifier.weight(1f),
            verticalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            items(messages) { m ->
                Card(
                    modifier = Modifier.fillMaxWidth().clickable(enabled = !m.deleted) { replyTo = m },
                ) {
                    Column(modifier = Modifier.padding(10.dp)) {
                        val nameColor = try {
                            m.nameColor?.let { Color(android.graphics.Color.parseColor(it)) }
                        } catch (e: Exception) {
                            null
                        }
                        Text(
                            (m.nameEmoji?.let { "$it " } ?: "") + m.displayName,
                            style = MaterialTheme.typography.labelLarge,
                            color = nameColor ?: MaterialTheme.colorScheme.onSurface,
                        )
                        if (m.replyPreview != null) {
                            Text(
                                "↩ " + m.replyPreview!!.take(60),
                                style = MaterialTheme.typography.bodySmall,
                                color = MaterialTheme.colorScheme.onSurfaceVariant,
                            )
                        }
                        Text(if (m.deleted) "pesan telah dihapus" else (m.body ?: ""))
                    }
                }
            }
        }
        if (replyTo != null) {
            Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
                Text(
                    "Membalas: " + (replyTo!!.body ?: "").take(40),
                    style = MaterialTheme.typography.bodySmall,
                    modifier = Modifier.weight(1f),
                )
                TextButton(onClick = { replyTo = null }) { Text("Batal") }
            }
        }
        Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            OutlinedTextField(
                value = input,
                onValueChange = { if (it.length <= 500) input = it },
                label = { Text("Pesan (maks 500, tanpa tautan luar)") },
                modifier = Modifier.weight(1f),
            )
            Button(
                enabled = input.isNotBlank(),
                onClick = {
                    val body = input.trim()
                    socket?.sendMessage(channelId, body, replyTo?.id)
                    input = ""
                    replyTo = null
                    error = null
                },
            ) { Text("Kirim") }
        }
    }
}
