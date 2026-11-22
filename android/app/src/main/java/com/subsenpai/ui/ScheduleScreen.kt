package com.subsenpai.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import coil.compose.AsyncImage
import com.subsenpai.R
import com.subsenpai.data.CatalogRepository
import com.subsenpai.data.ScheduleGroup
import java.util.Calendar
import kotlinx.coroutines.launch

/**
 * Jadwal Rilis (permintaan Rams 2026-11-23, menu di tab Lainnya):
 * 3 kategori — Anime (jadwal harian Kuronime + Update Acak),
 * Donghua (jadwal Anichin), Film (rilis terbaru katalog). Backend
 * yang mengambil & meng-cache jadwal dari sumber; entri yang ada
 * di katalog bisa diketuk menuju halaman detailnya.
 */
private fun todayLabel(): String = when (Calendar.getInstance().get(Calendar.DAY_OF_WEEK)) {
    Calendar.MONDAY -> "Senin"
    Calendar.TUESDAY -> "Selasa"
    Calendar.WEDNESDAY -> "Rabu"
    Calendar.THURSDAY -> "Kamis"
    Calendar.FRIDAY -> "Jumat"
    Calendar.SATURDAY -> "Sabtu"
    else -> "Minggu"
}

@Composable
fun ScheduleScreen(
    repo: CatalogRepository,
    onBack: () -> Unit,
    onOpenTitle: (String) -> Unit,
) {
    val types = listOf("anime" to "Anime", "donghua" to "Donghua", "film" to "Film")
    var selectedType by remember { mutableStateOf("anime") }
    var cache by remember { mutableStateOf<Map<String, List<ScheduleGroup>>>(emptyMap()) }
    var loading by remember { mutableStateOf(false) }
    var error by remember { mutableStateOf<String?>(null) }
    var selectedDay by remember { mutableStateOf<String?>(null) }
    val scope = rememberCoroutineScope()

    fun load(type: String) {
        scope.launch {
            loading = true
            error = null
            try {
                cache = cache + (type to repo.schedule(type))
            } catch (e: Exception) {
                error = e.message ?: "Gagal memuat jadwal"
            }
            loading = false
        }
    }

    LaunchedEffect(selectedType) {
        if (!cache.containsKey(selectedType)) load(selectedType)
    }

    val groups = cache[selectedType].orEmpty()
    val effectiveDay = selectedDay?.takeIf { d -> groups.any { it.day == d } }
        ?: groups.firstOrNull { it.day == todayLabel() }?.day
        ?: groups.firstOrNull()?.day
    val dayItems = groups.firstOrNull { it.day == effectiveDay }?.items.orEmpty()

    Column(modifier = Modifier.fillMaxSize()) {
        Row(
            verticalAlignment = Alignment.CenterVertically,
            modifier = Modifier.padding(horizontal = 4.dp, vertical = 8.dp),
        ) {
            IconButton(onClick = onBack) {
                Icon(painterResource(R.drawable.ic_arrow_back), contentDescription = "Kembali", tint = Color.White)
            }
            Text(
                "Jadwal Rilis",
                style = MaterialTheme.typography.headlineSmall,
                fontWeight = FontWeight.Bold,
                color = Color.White,
            )
        }

        Row(modifier = Modifier.fillMaxWidth().padding(horizontal = 16.dp)) {
            types.forEachIndexed { i, (key, label) ->
                if (i > 0) Spacer(Modifier.width(8.dp))
                val onClick = {
                    selectedType = key
                    selectedDay = null
                }
                if (key == selectedType) {
                    Button(onClick = onClick, modifier = Modifier.weight(1f)) { Text(label) }
                } else {
                    OutlinedButton(onClick = onClick, modifier = Modifier.weight(1f)) { Text(label) }
                }
            }
        }
        Spacer(Modifier.height(10.dp))

        when {
            loading -> Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                CircularProgressIndicator(color = NetflixRed)
            }
            error != null && groups.isEmpty() -> Column(
                modifier = Modifier.fillMaxWidth().padding(24.dp),
                horizontalAlignment = Alignment.CenterHorizontally,
            ) {
                Text(error ?: "", color = Color(0xFFFF8080))
                Spacer(Modifier.height(12.dp))
                Button(onClick = { load(selectedType) }) { Text("Coba Lagi") }
            }
            groups.isEmpty() -> Text(
                "Belum ada jadwal.",
                color = Color(0xFFB3B3B3),
                modifier = Modifier.padding(24.dp),
            )
            else -> {
                if (groups.size > 1) {
                    LazyRow(modifier = Modifier.padding(horizontal = 16.dp)) {
                        items(groups) { g ->
                            val sel = g.day == effectiveDay
                            Box(
                                modifier = Modifier.padding(end = 8.dp)
                                    .clip(RoundedCornerShape(16.dp))
                                    .background(if (sel) NetflixRed else Color(0xFF2A2A2A))
                                    .clickable { selectedDay = g.day }
                                    .padding(horizontal = 14.dp, vertical = 7.dp),
                            ) {
                                Text(g.day, color = Color.White, style = MaterialTheme.typography.bodyMedium)
                            }
                        }
                    }
                    Spacer(Modifier.height(8.dp))
                }
                Text(
                    when (selectedType) {
                        "anime" -> "Jadwal estimasi dari Kuronime — bisa lebih cepat atau terlambat."
                        "donghua" -> "Jadwal dari Anichin."
                        else -> "Film yang paling baru masuk katalog SubSenpai."
                    },
                    style = MaterialTheme.typography.bodySmall,
                    color = Color(0xFF808080),
                    modifier = Modifier.padding(horizontal = 16.dp, vertical = 4.dp),
                )
                if (dayItems.isEmpty()) {
                    Text(
                        "Tidak ada jadwal untuk hari ini.",
                        color = Color(0xFFB3B3B3),
                        modifier = Modifier.padding(24.dp),
                    )
                } else {
                    LazyColumn(modifier = Modifier.fillMaxSize()) {
                        items(dayItems) { item ->
                            Row(
                                modifier = Modifier.fillMaxWidth()
                                    .then(
                                        if (item.titleId != null) {
                                            Modifier.clickable { onOpenTitle(item.titleId) }
                                        } else {
                                            Modifier
                                        },
                                    )
                                    .padding(horizontal = 16.dp, vertical = 8.dp),
                                verticalAlignment = Alignment.CenterVertically,
                            ) {
                                AsyncImage(
                                    model = item.poster,
                                    contentDescription = null,
                                    contentScale = ContentScale.Crop,
                                    modifier = Modifier.size(width = 54.dp, height = 76.dp)
                                        .clip(RoundedCornerShape(6.dp))
                                        .background(Color(0xFF2A2A2A)),
                                )
                                Spacer(Modifier.width(12.dp))
                                Column(modifier = Modifier.weight(1f)) {
                                    Text(
                                        item.title,
                                        color = Color.White,
                                        style = MaterialTheme.typography.bodyLarge,
                                        maxLines = 2,
                                        overflow = TextOverflow.Ellipsis,
                                    )
                                    Spacer(Modifier.height(3.dp))
                                    Row(verticalAlignment = Alignment.CenterVertically) {
                                        if (item.episode > 0) {
                                            Text(
                                                "Episode ${item.episode}",
                                                color = Color(0xFFB3B3B3),
                                                style = MaterialTheme.typography.bodySmall,
                                            )
                                            Spacer(Modifier.width(10.dp))
                                        }
                                        Text(
                                            when {
                                                selectedType == "film" -> item.time
                                                item.time == "Sudah Rilis" -> "Sudah Rilis"
                                                item.time == "??" -> "Jadwal acak"
                                                item.time.isNotBlank() -> "Jam ${item.time}"
                                                else -> ""
                                            },
                                            color = if (item.time == "Sudah Rilis") {
                                                Color(0xFF46D369)
                                            } else {
                                                Color(0xFFB3B3B3)
                                            },
                                            style = MaterialTheme.typography.bodySmall,
                                            fontWeight = if (item.time == "Sudah Rilis") FontWeight.Bold else FontWeight.Normal,
                                        )
                                    }
                                }
                                if (item.titleId != null) {
                                    Icon(
                                        painterResource(R.drawable.ic_chevron_right),
                                        contentDescription = null,
                                        tint = Color(0xFF808080),
                                    )
                                }
                            }
                        }
                        item { Spacer(Modifier.height(16.dp)) }
                    }
                }
            }
        }
    }
}
