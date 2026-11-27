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
import android.widget.Toast
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
import com.subsenpai.ui.AdUnlockScreen
import com.subsenpai.ui.ChatRoomScreen
import com.subsenpai.ui.CommentsScreen
import com.subsenpai.ui.FeedNotificationsScreen
import com.subsenpai.ui.FeedScreen
import com.subsenpai.ui.DetailScreen
import com.subsenpai.ui.FriendsScreen
import com.subsenpai.ui.GroupsScreen
import com.subsenpai.ui.HomeScreen
import com.subsenpai.ui.HomeSession
import com.subsenpai.ui.ScheduleSession
import com.subsenpai.ui.LoginScreen
import com.subsenpai.ui.PremiumScreen
import com.subsenpai.ui.WatchlistScreen

class MainActivity : ComponentActivity() {
    // Deep link subsenpai://title/{id} dari halaman web resmi:
    // id judul yang menunggu dibuka (dikonsumsi komposisi di bawah).
    private val deepLinkTitle = mutableStateOf<String?>(null)

    // Deep link subsenpai://episode/{id} (tautan /nonton/ dari
    // notifikasi jadwal rilis): id episode yang menunggu diputar.
    private val deepLinkEpisode = mutableStateOf<String?>(null)

    private fun captureDeepLink(intent: Intent?) {
        val data = intent?.data ?: return
        // Dua bentuk tautan resmi:
        // 1) subsenpai://title/{id} — dari poster halaman download.
        // 2) https://www.subsenpai.web.id/judul/{id} — tautan yang
        //    dibagikan dari halaman detail (Android App Links,
        //    terverifikasi lewat assetlinks.json di domain itu).
        val id = when {
            data.scheme == "subsenpai" && data.host == "title" ->
                data.pathSegments.firstOrNull().orEmpty()
            data.scheme == "https" &&
                (data.host == "www.subsenpai.web.id" || data.host == "subsenpai.web.id") &&
                data.pathSegments.firstOrNull() == "judul" ->
                data.pathSegments.getOrNull(1).orEmpty()
            else -> ""
        }
        if (id.matches(Regex("[A-Za-z0-9-]{8,}"))) deepLinkTitle.value = id
        // Bentuk tautan nonton episode:
        // 3) subsenpai://episode/{id} — dari halaman /nonton.
        // 4) https://www.subsenpai.web.id/nonton/{id} — tautan
        //    notifikasi jadwal rilis (App Links).
        val epId = when {
            data.scheme == "subsenpai" && data.host == "episode" ->
                data.pathSegments.firstOrNull().orEmpty()
            data.scheme == "https" &&
                (data.host == "www.subsenpai.web.id" || data.host == "subsenpai.web.id") &&
                data.pathSegments.firstOrNull() == "nonton" ->
                data.pathSegments.getOrNull(1).orEmpty()
            else -> ""
        }
        if (epId.matches(Regex("[A-Za-z0-9-]{8,}"))) deepLinkEpisode.value = epId
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        setIntent(intent)
        captureDeepLink(intent)
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        captureDeepLink(intent)
        // Simpan laporan bila aplikasi crash; diunggah saat start.
        com.subsenpai.data.CrashReporter.install(this)
        // Sembuh sendiri dari lingkaran crash (v0.24.12): bila
        // aplikasi crash 2x beruntun tanpa sempat sehat — pola
        // "tidak bisa dibuka sampai hapus cache" yang dilaporkan
        // Rams — bersihkan cache gambar SEBELUM UI tampil, lalu
        // mulai dari nol. Penghitung direset begitu aplikasi
        // terbukti hidup 8 detik.
        if (com.subsenpai.data.CrashReporter.crashStreak(this) >= 2) {
            val clearer = Thread {
                try {
                    val loader = coil.Coil.imageLoader(this)
                    loader.memoryCache?.clear()
                    loader.diskCache?.clear()
                } catch (_: Exception) {
                }
            }
            clearer.start()
            try {
                clearer.join(900)
            } catch (_: Exception) {
            }
            com.subsenpai.data.CrashReporter.resetStreak(this)
        }
        android.os.Handler(android.os.Looper.getMainLooper()).postDelayed({
            com.subsenpai.data.CrashReporter.markHealthy(this)
        }, 8000)
        setContent {
            com.subsenpai.ui.SubSenpaiTheme {
                val context = LocalContext.current
                val nav = rememberNavController()
                val repo = remember { CatalogRepository(auth = AuthStore(context)) }
                // Sesi Beranda hidup di sini (di luar NavHost) agar
                // tab kategori + posisi scroll Beranda selamat dari
                // navigasi ke Detail/Pemutar (permintaan Rams
                // 2026-11-24) — lihat HomeSession di HomeScreen.kt.
                val homeSession = remember { HomeSession() }
                // Sesi Jadwal Rilis: pola yang sama (kategori + hari
                // + posisi scroll selamat dari navigasi).
                val scheduleSession = remember { ScheduleSession() }
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
                // Status akun (v0.24.15): pasangan (status, catatan
                // admin) bila akun di-ban/ditangguhkan — diperiksa
                // lewat /me pada poll konfigurasi yang sama; layar
                // blokir tampil selama status ini terisi.
                var accountBlocked by remember { mutableStateOf<Pair<String, String>?>(null) }
                // Catatan perangkat dikeluarkan (batas perangkat):
                // diisi oleh poll /me saat server menolak dengan
                // "perangkat_dicabut:..." atau oleh LoginScreen
                // sesudah login menendang perangkat lain — tampil
                // sekali sebagai Toast, lalu dibersihkan.
                var deviceRevokedNotice by remember { mutableStateOf<String?>(null) }
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
                // Catatan perangkat dikeluarkan: tampilkan sekali
                // sebagai Toast di layar mana pun yang sedang tampil,
                // lalu bersihkan agar tidak muncul berulang (mis. saat
                // rotasi layar) — pola sekali pakai yang sama dengan
                // takeRevokedDevice() di repositori.
                LaunchedEffect(deviceRevokedNotice) {
                    val notice = deviceRevokedNotice ?: return@LaunchedEffect
                    Toast.makeText(context, notice, Toast.LENGTH_LONG).show()
                    deviceRevokedNotice = null
                }
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
                        // Status akun (v0.24.15): akun yang di-ban /
                        // ditangguhkan admin ditolak server di semua
                        // panggilan login — periksa /me tiap siklus
                        // agar layar blokir tampil paling lambat 60
                        // detik setelah status berubah, dan lepas
                        // sendiri begitu admin mengaktifkan lagi.
                        try {
                            if (AuthStore(context).token != null) {
                                repo.me()
                                accountBlocked = null
                            }
                        } catch (e: com.subsenpai.data.ApiException) {
                            val blocked = com.subsenpai.data.parseBlockedMessage(e.message)
                            if (blocked != null) accountBlocked = blocked
                            // Perangkat ini dicabut karena akun login di
                            // perangkat lain (batas perangkat: premium
                            // maks 2, gratis maks 1): hapus sesi, kembali
                            // ke layar masuk, beri tahu sekali. Cerminan
                            // alur akun_nonaktif — bedanya akun_nonaktif
                            // hanya menampilkan layar blokir, sedangkan
                            // perangkat dicabut = sesi berakhir.
                            val revokedName =
                                com.subsenpai.data.parseRevokedDeviceMessage(e.message)
                            if (revokedName != null) {
                                repo.logout()
                                accountBlocked = null
                                deviceRevokedNotice =
                                    com.subsenpai.data.revokedDeviceMessage(revokedName)
                                authVersion++
                                if (nav.currentDestination?.route != "login") {
                                    nav.navigate("login") {
                                        popUpTo("home") { saveState = false }
                                        launchSingleTop = true
                                    }
                                }
                            }
                        } catch (e: Exception) {
                            // Abaikan: gangguan jaringan bukan blokir.
                        }
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
                        accountBlocked != null -> com.subsenpai.ui.GatePage(
                            iconRes = R.drawable.ic_info,
                            title = com.subsenpai.data.blockedTitle(accountBlocked!!.first),
                            message = com.subsenpai.data.blockedDescription(
                                accountBlocked!!.first,
                                accountBlocked!!.second,
                            ),
                            buttonLabel = "Keluar dari Akun",
                            onButton = {
                                repo.logout()
                                accountBlocked = null
                                authVersion++
                            },
                        )
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
                                session = homeSession,
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
                                onOpenDevices = { nav.navigate("devices") },
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
                            )
                        }
                        composable("schedule") {
                            com.subsenpai.ui.ScheduleScreen(
                                repo,
                                session = scheduleSession,
                                onOpenTitle = { nav.navigate("detail/$it") },
                                onPlayEpisode = { nav.navigate("player/$it") },
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
                                onRevokedDeviceNote = { deviceRevokedNotice = it },
                            )
                        }
                        composable("watchlist") {
                            WatchlistScreen(
                                repo,
                                onOpenTitle = { nav.navigate("detail/$it") },
                                onBack = { nav.popBackStack() },
                            )
                        }
                        composable("devices") {
                            com.subsenpai.ui.DevicesScreen(
                                repo,
                                onBack = { nav.popBackStack() },
                                onNeedLogin = { nav.navigate("login") },
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
                                onOpenAdUnlock = { nav.navigate("adunlock/$it") },
                            )
                        }
                        composable(
                            "adunlock/{episodeId}",
                            arguments = listOf(navArgument("episodeId") { type = NavType.StringType }),
                        ) { entry ->
                            val epId = entry.arguments!!.getString("episodeId")!!
                            AdUnlockScreen(
                                repo,
                                epId,
                                onUnlocked = {
                                    nav.navigate("player/$it") {
                                        popUpTo("adunlock/{episodeId}") { inclusive = true }
                                    }
                                },
                                onBack = { nav.popBackStack() },
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
                                onOpenAdUnlock = { nav.navigate("adunlock/$it") },
                            )
                        }
                    }
                    // Deep link web: navigasi HARUS di sini, SETELAH
                    // NavHost terkomposisi (graf navigasi sudah
                    // terpasang). Di v0.24.5 efek ini ditaruh di samping
                    // rememberNavController dan menyebabkan crash
                    // "Navigation graph has not been set" saat aplikasi
                    // dibuka dingin lewat deep link — jangan dipindah
                    // ke atas lagi.
                    val pendingTitle = deepLinkTitle.value
                    LaunchedEffect(pendingTitle) {
                        if (pendingTitle != null) {
                            nav.navigate("detail/$pendingTitle")
                            deepLinkTitle.value = null
                        }
                    }
                    // Deep link episode: langsung ke pemutar (rute
                    // player menerima id episode). Posisi sesudah
                    // NavHost sama seperti deep link judul di atas.
                    val pendingEpisode = deepLinkEpisode.value
                    LaunchedEffect(pendingEpisode) {
                        if (pendingEpisode != null) {
                            nav.navigate("player/$pendingEpisode")
                            deepLinkEpisode.value = null
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
