package com.subsenpai

import android.content.Intent
import android.net.Uri
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.NavigationBar
import androidx.compose.material3.NavigationBarItem
import androidx.compose.material3.NavigationBarItemDefaults
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.LaunchedEffect
import kotlinx.coroutines.delay
import android.Manifest
import android.content.pm.PackageManager
import android.os.Build
import androidx.core.app.ActivityCompat
import androidx.core.content.ContextCompat
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.unit.dp
import androidx.navigation.NavType
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.currentBackStackEntryAsState
import androidx.navigation.compose.rememberNavController
import androidx.navigation.navArgument
import com.subsenpai.data.AuthStore
import com.subsenpai.data.CatalogRepository
import com.subsenpai.player.PlayerScreen
import com.subsenpai.ui.ChatRoomScreen
import com.subsenpai.ui.CommentsScreen
import com.subsenpai.ui.FeedNotificationsScreen
import com.subsenpai.ui.FeedScreen
import com.subsenpai.ui.DetailScreen
import com.subsenpai.ui.FriendsScreen
import com.subsenpai.ui.GroupsScreen
import com.subsenpai.ui.HomeScreen
import com.subsenpai.ui.LoginScreen
import com.subsenpai.ui.PremiumScreen
import com.subsenpai.ui.WatchlistScreen

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        // Simpan laporan bila aplikasi crash; diunggah saat start.
        com.subsenpai.data.CrashReporter.install(this)
        setContent {
            com.subsenpai.ui.SubSenpaiTheme {
                val context = LocalContext.current
                val nav = rememberNavController()
                val repo = remember { CatalogRepository(auth = AuthStore(context)) }
                // Bagikan judul ke tab Post: isi prefill composer lalu
                // pindah ke tab Post (FeedScreen menerapkannya).
                val shareToPost: (com.subsenpai.data.TitleDetail) -> Unit = { d ->
                    val typeLabel = when (d.card.type) {
                        "anime" -> "Anime"
                        "donghua" -> "Donghua"
                        "film" -> "Film"
                        else -> "Umum"
                    }
                    val text = buildString {
                        append("🎬 Lagi nonton ${d.card.title}")
                        d.card.year?.let { append(" ($it)") }
                        append(" — $typeLabel")
                        d.synopsis?.let { s ->
                            append("\n\n")
                            append(s.take(280))
                        }
                    }
                    com.subsenpai.data.ShareToPostBus.send(
                        com.subsenpai.data.SharePrefill(
                            text = text,
                            posterUrl = d.card.posterUrl,
                            titleId = d.card.id,
                            category = when (d.card.type) {
                                "anime", "donghua", "film" -> d.card.type
                                else -> "umum"
                            },
                        ),
                    )
                    nav.navigate("chat") {
                        popUpTo("home") { saveState = true }
                        launchSingleTop = true
                        restoreState = true
                    }
                }
                var authVersion by remember { mutableStateOf(0) }
                var homeFatal by remember { mutableStateOf(false) }
                // Konfigurasi server (dashboard admin › Pengaturan):
                // mode perawatan, paksa perbarui, pengumuman. Gagal
                // memuat = aplikasi berjalan normal tanpa gerbang.
                var appConfig by remember { mutableStateOf<com.subsenpai.data.AppConfig?>(null) }
                var configLoaded by remember { mutableStateOf(false) }
                var announcementDismissed by remember { mutableStateOf(false) }
                // Izin notifikasi sistem (Android 13+) — diminta sekali
                // agar pengumuman admin bisa muncul di bilah status.
                LaunchedEffect(Unit) {
                    if (Build.VERSION.SDK_INT >= 33 &&
                        ContextCompat.checkSelfPermission(
                            context, Manifest.permission.POST_NOTIFICATIONS,
                        ) != PackageManager.PERMISSION_GRANTED
                    ) {
                        ActivityCompat.requestPermissions(
                            this@MainActivity,
                            arrayOf(Manifest.permission.POST_NOTIFICATIONS),
                            9001,
                        )
                    }
                }
                var prevUnread by remember { mutableStateOf(-1) }
                LaunchedEffect(Unit) {
                    com.subsenpai.data.CrashReporter.uploadPending(context, repo)
                    // Periksa konfigurasi server TIAP 60 DETIK (v0.22.2):
                    // mode perawatan / paksa perbarui / pengumuman dari
                    // dashboard admin berlaku langsung tanpa membuka
                    // ulang aplikasi — inilah "notifikasi" mode
                    // perawatan ke aplikasi yang sedang berjalan.
                    while (true) {
                        try {
                            appConfig = repo.appConfig()
                        } catch (e: Exception) {
                            // Abaikan: tanpa konfigurasi, tanpa gerbang.
                        }
                        configLoaded = true
                        // Notifikasi sistem (v0.22.4): bila jumlah belum
                        // dibaca BERTAMBAH (pengumuman admin / aktivitas
                        // sosial baru), tampilkan notifikasi Android.
                        // Poll pertama hanya jadi garis dasar.
                        try {
                            if (AuthStore(context).token != null) {
                                val (items, unread) = repo.feedNotifications()
                                if (prevUnread >= 0 && unread > prevUnread) {
                                    items.firstOrNull { !it.read }?.let { n ->
                                        com.subsenpai.data.Notifier.show(
                                            context,
                                            n.title.ifBlank { "SubSenpai" },
                                            n.body,
                                        )
                                    }
                                }
                                prevUnread = unread
                            }
                        } catch (e: Exception) {
                            // Abaikan: notifikasi sistem hanya bonus.
                        }
                        delay(60_000)
                    }
                }
                // Paksa perbarui: utamakan NAMA versi dari dashboard
                // (admin mengetik "0.22.2"); kode versi hanya cadangan.
                val minName = appConfig?.minVersionName?.takeIf { it.isNotBlank() }
                val outdated = appConfig != null && if (minName != null) {
                    com.subsenpai.data.compareVersionNames(BuildConfig.VERSION_NAME, minName) < 0
                } else {
                    BuildConfig.VERSION_CODE < (appConfig?.minVersionCode ?: 0)
                }
                val gated = !configLoaded || appConfig?.maintenanceMode == true || outdated
                val backEntry by nav.currentBackStackEntryAsState()
                val route = backEntry?.destination?.route
                // Bilah bawah ala Netflix: hanya di layar-layar utama.
                val mainRoutes = setOf("home", "chat", "schedule", "premium", "history", "more")
                val bottomItems = listOf(
                    Triple("home", "Beranda", R.drawable.ic_home),
                    Triple("chat", "Post", R.drawable.ic_chat),
                    Triple("schedule", "Jadwal", R.drawable.ic_calendar),
                    Triple("history", "Riwayat", R.drawable.ic_history),
                    Triple("more", "Lainnya", R.drawable.ic_menu),
                )
                Scaffold(
                    containerColor = MaterialTheme.colorScheme.background,
                    bottomBar = {
                        // Halaman peringatan fatal di Beranda tampil
                        // tanpa bilah navigasi (permintaan Rams).
                        if (!gated && route in mainRoutes && !(route == "home" && homeFatal)) {
                            NavigationBar(containerColor = Color(0xFF0B0B0B)) {
                                bottomItems.forEach { (dest, label, icon) ->
                                    NavigationBarItem(
                                        selected = route == dest,
                                        onClick = {
                                            nav.navigate(dest) {
                                                popUpTo("home") { saveState = true }
                                                launchSingleTop = true
                                                restoreState = true
                                            }
                                        },
                                        icon = {
                                            Icon(
                                                painter = painterResource(icon),
                                                contentDescription = label,
                                            )
                                        },
                                        label = {
                                            Text(label, style = MaterialTheme.typography.labelSmall)
                                        },
                                        colors = NavigationBarItemDefaults.colors(
                                            selectedIconColor = Color.White,
                                            selectedTextColor = Color.White,
                                            unselectedIconColor = Color(0xFFB3B3B3),
                                            unselectedTextColor = Color(0xFFB3B3B3),
                                            indicatorColor = Color(0xFF2A2A2A),
                                        ),
                                    )
                                }
                            }
                        }
                    },
                ) { padding ->
                    Box(Modifier.padding(padding)) {
                    when {
                        !configLoaded -> com.subsenpai.ui.BrandLoadingScreen()
                        appConfig?.maintenanceMode == true -> com.subsenpai.ui.GatePage(
                            iconRes = R.drawable.ic_settings,
                            title = "Sedang Dalam Perawatan",
                            message = appConfig?.maintenanceMessage
                                ?.takeIf { it.isNotBlank() }
                                ?: "SubSenpai sedang dalam perawatan. Coba lagi sebentar lagi ya.",
                        )
                        outdated -> com.subsenpai.ui.GatePage(
                            iconRes = R.drawable.ic_info,
                            title = "Perbarui SubSenpai",
                            message = "Versi aplikasi ini sudah terlalu lama dan tidak bisa digunakan lagi. " +
                                "Unduh versi terbaru untuk melanjutkan menonton.",
                            buttonLabel = "Unduh Versi Baru",
                            onButton = {
                                val url = appConfig?.updateUrl?.takeIf { it.isNotBlank() }
                                if (url != null) {
                                    context.startActivity(Intent(Intent.ACTION_VIEW, Uri.parse(url)))
                                }
                            },
                        )
                        else -> Column(Modifier.fillMaxSize()) {
                        val announcement = appConfig?.announcement?.takeIf { it.isNotBlank() }
                        if (announcement != null && !announcementDismissed) {
                            Row(
                                modifier = Modifier.fillMaxWidth()
                                    .background(Color(0xFF7C3AED))
                                    .padding(horizontal = 16.dp, vertical = 6.dp),
                                verticalAlignment = androidx.compose.ui.Alignment.CenterVertically,
                            ) {
                                Text(
                                    announcement,
                                    color = Color.White,
                                    style = MaterialTheme.typography.bodySmall,
                                    modifier = Modifier.weight(1f),
                                )
                                TextButton(onClick = { announcementDismissed = true }) {
                                    Text("Tutup", color = Color.White)
                                }
                            }
                        }
                    NavHost(navController = nav, startDestination = "home", modifier = Modifier.weight(1f)) {
                        composable("home") {
                            HomeScreen(
                                repo,
                                authVersion = authVersion,
                                onOpenTitle = { nav.navigate("detail/$it") },
                                onOpenEpisode = { nav.navigate("player/$it") },
                                onLogin = { nav.navigate("login") },
                                onOpenMore = {
                                    nav.navigate("more") {
                                        popUpTo("home") { saveState = true }
                                        launchSingleTop = true
                                        restoreState = true
                                    }
                                },
                                onOpenNotices = { nav.navigate("notices") },
                                onOpenPremium = {
                                    nav.navigate("premium") {
                                        popUpTo("home") { saveState = true }
                                        launchSingleTop = true
                                        restoreState = true
                                    }
                                },
                                onFatalChange = { homeFatal = it },
                            )
                        }
                        composable("more") {
                            com.subsenpai.ui.MoreScreen(
                                onOpenLogin = { nav.navigate("login") },
                                onOpenHistory = {
                                    nav.navigate("history") {
                                        popUpTo("home") { saveState = true }
                                        launchSingleTop = true
                                        restoreState = true
                                    }
                                },
                                onOpenWatchlist = { nav.navigate("watchlist") },
                                onOpenPremium = {
                                    nav.navigate("premium") {
                                        popUpTo("home") { saveState = true }
                                        launchSingleTop = true
                                        restoreState = true
                                    }
                                },
                                onOpenFriends = { nav.navigate("friends") },
                                onOpenGroups = { nav.navigate("groups") },
                                onOpenSettings = { nav.navigate("settings") },
                                onOpenTransactions = { nav.navigate("transactions") },
                                onOpenMyVouchers = { nav.navigate("myvouchers") },
                                onOpenSchedule = {
                                    nav.navigate("schedule") {
                                        popUpTo("home") { saveState = true }
                                        launchSingleTop = true
                                        restoreState = true
                                    }
                                },
                            )
                        }
                        composable("schedule") {
                            com.subsenpai.ui.ScheduleScreen(
                                repo,
                                onBack = { nav.popBackStack() },
                                onOpenTitle = { nav.navigate("detail/$it") },
                            )
                        }
                        composable("transactions") {
                            com.subsenpai.ui.TransactionsScreen(
                                repo,
                                onBack = { nav.popBackStack() },
                            )
                        }
                        composable("myvouchers") {
                            com.subsenpai.ui.MyVouchersScreen(
                                repo,
                                onBack = { nav.popBackStack() },
                            )
                        }
                        composable("history") {
                            com.subsenpai.ui.HistoryScreen(
                                repo,
                                onBack = { nav.popBackStack() },
                                onOpenEpisode = { nav.navigate("player/$it") },
                                onNeedLogin = { nav.navigate("login") },
                            )
                        }
                        composable("settings") {
                            com.subsenpai.ui.SettingsScreen(
                                onBack = { nav.popBackStack() },
                            )
                        }
                        composable("premium") {
                            PremiumScreen(
                                repo,
                                onBack = { nav.popBackStack() },
                                onNeedLogin = { nav.navigate("login") },
                            )
                        }
                        composable("chat") {
                            FeedScreen(
                                repo,
                                onNeedLogin = { nav.navigate("login") },
                                onOpenComments = { postId -> nav.navigate("comments/$postId") },
                                onOpenNotifications = { nav.navigate("feednotif") },
                                onBack = { nav.popBackStack() },
                                onOpenTitle = { nav.navigate("detail/$it") },
                            )
                        }
                        composable(
                            "comments/{postId}",
                            arguments = listOf(navArgument("postId") { type = NavType.StringType }),
                        ) { backStack ->
                            CommentsScreen(
                                repo,
                                postId = backStack.arguments?.getString("postId") ?: "",
                                onBack = { nav.popBackStack() },
                                onOpenTitle = { nav.navigate("detail/$it") },
                            )
                        }
                        composable("feednotif") {
                            FeedNotificationsScreen(
                                repo,
                                onBack = { nav.popBackStack() },
                                onOpenPost = { postId -> nav.navigate("comments/$postId") },
                                kind = "social",
                                title = "Notifikasi",
                            )
                        }
                        composable("notices") {
                            FeedNotificationsScreen(
                                repo,
                                onBack = { nav.popBackStack() },
                                onOpenPost = { postId -> nav.navigate("comments/$postId") },
                                kind = null,
                                title = "Notifikasi",
                                onOpenTitle = { nav.navigate("detail/$it") },
                                showTabs = true,
                            )
                        }
                        composable(
                            "room/{channelId}/{name}",
                            arguments = listOf(
                                navArgument("channelId") { type = NavType.StringType },
                                navArgument("name") { type = NavType.StringType },
                            ),
                        ) { entry ->
                            ChatRoomScreen(
                                repo,
                                entry.arguments!!.getString("channelId")!!,
                                entry.arguments!!.getString("name")!!,
                                onBack = { nav.popBackStack() },
                            )
                        }
                        composable("friends") {
                            FriendsScreen(
                                repo,
                                onOpenDm = { id, name -> nav.navigate("room/$id/$name") },
                                onBack = { nav.popBackStack() },
                            )
                        }
                        composable("groups") {
                            GroupsScreen(
                                repo,
                                onOpenRoom = { id, name -> nav.navigate("room/$id/$name") },
                                onBack = { nav.popBackStack() },
                            )
                        }
                        composable("login") {
                            LoginScreen(
                                repo,
                                onBack = { nav.popBackStack() },
                                onAuthChanged = { authVersion++ },
                            )
                        }
                        composable("watchlist") {
                            WatchlistScreen(
                                repo,
                                onOpenTitle = { nav.navigate("detail/$it") },
                                onBack = { nav.popBackStack() },
                            )
                        }
                        composable(
                            "detail/{id}",
                            arguments = listOf(navArgument("id") { type = NavType.StringType }),
                        ) { entry ->
                            val id = entry.arguments!!.getString("id")!!
                            DetailScreen(
                                repo, id,
                                onPlay = { nav.navigate("player/$it") },
                                onBack = { nav.popBackStack() },
                                onNeedLogin = { nav.navigate("login") },
                                onOpenTitle = { nav.navigate("detail/$it") },
                                onShareToPost = shareToPost,
                                onOpenPremium = {
                                    nav.navigate("premium") {
                                        popUpTo("home") { saveState = true }
                                        launchSingleTop = true
                                        restoreState = true
                                    }
                                },
                            )
                        }
                        composable(
                            "player/{episodeId}",
                            arguments = listOf(navArgument("episodeId") { type = NavType.StringType }),
                        ) { entry ->
                            PlayerScreen(
                                repo,
                                entry.arguments!!.getString("episodeId")!!,
                                onBack = { nav.popBackStack() },
                                onNeedLogin = { nav.navigate("login") },
                                onOpenTitle = { nav.navigate("detail/$it") },
                                onShareToPost = shareToPost,
                                onOpenPremium = {
                                    nav.navigate("premium") {
                                        popUpTo("home") { saveState = true }
                                        launchSingleTop = true
                                        restoreState = true
                                    }
                                },
                            )
                        }
                    }
                        }
                    }
                    }
                }
            }
        }
    }
}
