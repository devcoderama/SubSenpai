package com.subsenpai.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import com.subsenpai.data.CatalogRepository
import com.subsenpai.data.TitleCard

/** Daftar watchlist akun untuk profil tontonan aktif. */
@Composable
fun WatchlistScreen(
    repo: CatalogRepository,
    onOpenTitle: (String) -> Unit,
    onBack: () -> Unit,
) {
    var items by remember { mutableStateOf<List<TitleCard>>(emptyList()) }
    var error by remember { mutableStateOf<String?>(null) }
    var loaded by remember { mutableStateOf(false) }

    LaunchedEffect(Unit) {
        try {
            items = repo.watchlist()
        } catch (e: Exception) {
            error = e.message
        } finally {
            loaded = true
        }
    }

    LazyColumn(
        modifier = Modifier.fillMaxSize().padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        item { TextButton(onClick = onBack) { Text("← Kembali") } }
        item { Text("Watchlist Saya", style = MaterialTheme.typography.headlineSmall) }
        if (error != null) {
            item { Text("Gagal memuat: $error", color = MaterialTheme.colorScheme.error) }
        }
        if (loaded && items.isEmpty() && error == null) {
            item { Text("Watchlist masih kosong — tambahkan dari halaman detail judul.") }
        }
        items(items) { TitleRow(it, onOpenTitle) }
    }
}
