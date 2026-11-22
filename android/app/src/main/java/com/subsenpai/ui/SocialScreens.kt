package com.subsenpai.ui

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
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import com.subsenpai.data.CatalogRepository
import com.subsenpai.data.FriendItem
import com.subsenpai.data.GroupItem
import kotlinx.coroutines.launch

/** Layar Teman: kode saya, tambah via kode, permintaan masuk, daftar + DM. */
@Composable
fun FriendsScreen(
    repo: CatalogRepository,
    onOpenDm: (String, String) -> Unit,
    onBack: () -> Unit,
) {
    val scope = rememberCoroutineScope()
    var friends by remember { mutableStateOf<List<FriendItem>>(emptyList()) }
    var code by remember { mutableStateOf("") }
    var info by remember { mutableStateOf<String?>(null) }
    var error by remember { mutableStateOf<String?>(null) }
    var refresh by remember { mutableStateOf(0) }

    LaunchedEffect(refresh) {
        try {
            friends = repo.friends()
        } catch (e: Exception) {
            error = e.message
        }
    }

    LazyColumn(
        modifier = Modifier.fillMaxSize().padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        item { TextButton(onClick = onBack) { Text("← Kembali") } }
        item { Text("Teman", style = MaterialTheme.typography.headlineSmall) }
        item {
            Text("Kode temanmu: ${repo.auth?.friendCode ?: "-"}")
            Text(
                "Bagikan kode ini agar teman bisa menambahkanmu (juga kode referral premium).",
                style = MaterialTheme.typography.bodySmall,
            )
        }
        item {
            Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                OutlinedTextField(
                    value = code,
                    onValueChange = { code = it },
                    label = { Text("Kode teman") },
                    modifier = Modifier.weight(1f),
                    singleLine = true,
                )
                Button(
                    enabled = code.isNotBlank(),
                    onClick = {
                        scope.launch {
                            try {
                                repo.friendRequest(code.trim())
                                info = "Permintaan pertemanan terkirim."
                                code = ""
                                refresh++
                            } catch (e: Exception) {
                                error = e.message
                            }
                        }
                    },
                ) { Text("Tambah") }
            }
        }
        if (info != null) item { Text(info!!) }
        if (error != null) item { Text(error!!, color = MaterialTheme.colorScheme.error) }
        items(friends) { f ->
            Card(modifier = Modifier.fillMaxWidth()) {
                Column(modifier = Modifier.padding(12.dp)) {
                    Text(f.displayName ?: f.email, style = MaterialTheme.typography.titleMedium)
                    Text(
                        when (f.status) {
                            "accepted" -> "Teman"
                            "pending_in" -> "Meminta pertemanan"
                            else -> "Permintaan terkirim"
                        },
                        style = MaterialTheme.typography.bodySmall,
                    )
                    Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        if (f.status == "pending_in") {
                            Button(onClick = {
                                scope.launch {
                                    try {
                                        repo.friendAccept(f.userId)
                                        refresh++
                                    } catch (e: Exception) {
                                        error = e.message
                                    }
                                }
                            }) { Text("Terima") }
                        }
                        if (f.status == "accepted") {
                            Button(onClick = {
                                scope.launch {
                                    try {
                                        val channelId = repo.dmOpen(f.userId)
                                        onOpenDm(channelId, f.displayName ?: f.email)
                                    } catch (e: Exception) {
                                        error = e.message
                                    }
                                }
                            }) { Text("Kirim pesan") }
                        }
                        TextButton(onClick = {
                            scope.launch {
                                try {
                                    repo.friendBlock(f.userId)
                                    refresh++
                                } catch (e: Exception) {
                                    error = e.message
                                }
                            }
                        }) { Text("Blokir") }
                    }
                }
            }
        }
    }
}

/** Layar Grup: grup saya, grup publik, buat grup, gabung via kode. */
@Composable
fun GroupsScreen(
    repo: CatalogRepository,
    onOpenRoom: (String, String) -> Unit,
    onBack: () -> Unit,
) {
    val scope = rememberCoroutineScope()
    var groups by remember { mutableStateOf<List<GroupItem>>(emptyList()) }
    var name by remember { mutableStateOf("") }
    var isPublic by remember { mutableStateOf(true) }
    var joinCode by remember { mutableStateOf("") }
    var info by remember { mutableStateOf<String?>(null) }
    var error by remember { mutableStateOf<String?>(null) }
    var refresh by remember { mutableStateOf(0) }

    LaunchedEffect(refresh) {
        try {
            groups = repo.groups()
        } catch (e: Exception) {
            error = e.message
        }
    }

    LazyColumn(
        modifier = Modifier.fillMaxSize().padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        item { TextButton(onClick = onBack) { Text("← Kembali") } }
        item { Text("Grup", style = MaterialTheme.typography.headlineSmall) }
        item {
            Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                OutlinedTextField(
                    value = name,
                    onValueChange = { name = it },
                    label = { Text("Nama grup baru") },
                    modifier = Modifier.fillMaxWidth(),
                    singleLine = true,
                )
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    Text("Publik")
                    Switch(checked = isPublic, onCheckedChange = { isPublic = it })
                }
                Button(
                    enabled = name.isNotBlank(),
                    onClick = {
                        scope.launch {
                            try {
                                val o = repo.groupCreate(name.trim(), isPublic)
                                info = "Grup dibuat! Kode undangan: " + o.optString("invite_code")
                                name = ""
                                refresh++
                            } catch (e: Exception) {
                                error = e.message
                            }
                        }
                    },
                ) { Text("Buat grup") }
            }
        }
        item {
            Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                OutlinedTextField(
                    value = joinCode,
                    onValueChange = { joinCode = it },
                    label = { Text("Kode undangan grup") },
                    modifier = Modifier.weight(1f),
                    singleLine = true,
                )
                Button(
                    enabled = joinCode.isNotBlank(),
                    onClick = {
                        scope.launch {
                            try {
                                val o = repo.groupJoinByCode(joinCode.trim())
                                info = "Bergabung ke " + o.optString("group_name")
                                joinCode = ""
                                refresh++
                            } catch (e: Exception) {
                                error = e.message
                            }
                        }
                    },
                ) { Text("Gabung") }
            }
        }
        if (info != null) item { Text(info!!) }
        if (error != null) item { Text(error!!, color = MaterialTheme.colorScheme.error) }
        items(groups) { g ->
            Card(modifier = Modifier.fillMaxWidth()) {
                Column(modifier = Modifier.padding(12.dp)) {
                    Text(g.name, style = MaterialTheme.typography.titleMedium)
                    Text(
                        (if (g.isPublic) "Publik" else "Privat") +
                            (if (g.isMember) " · anggota" else "") +
                            (g.inviteCode?.let { " · kode: $it" } ?: ""),
                        style = MaterialTheme.typography.bodySmall,
                    )
                    Spacer(Modifier.height(4.dp))
                    if (g.isMember && g.channelId != null) {
                        Button(onClick = { onOpenRoom(g.channelId!!, g.name) }) { Text("Buka chat grup") }
                    } else if (!g.isMember) {
                        OutlinedButton(onClick = {
                            scope.launch {
                                try {
                                    repo.groupJoin(g.id)
                                    refresh++
                                } catch (e: Exception) {
                                    error = e.message
                                }
                            }
                        }) { Text("Gabung grup") }
                    }
                }
            }
        }
    }
}
