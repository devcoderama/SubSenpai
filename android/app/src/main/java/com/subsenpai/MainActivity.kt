package com.subsenpai

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.platform.LocalContext
import androidx.navigation.NavType
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController
import androidx.navigation.navArgument
import com.subsenpai.data.AuthStore
import com.subsenpai.data.CatalogRepository
import com.subsenpai.player.PlayerScreen
import com.subsenpai.ui.ChatListScreen
import com.subsenpai.ui.ComicBrowseScreen
import com.subsenpai.ui.ComicDetailScreen
import com.subsenpai.ui.ComicReaderScreen
import com.subsenpai.ui.ChatRoomScreen
import com.subsenpai.ui.DetailScreen
import com.subsenpai.ui.DeviceReaderScreen
import com.subsenpai.ui.FriendsScreen
import com.subsenpai.ui.GroupsScreen
import com.subsenpai.ui.HomeScreen
import com.subsenpai.ui.LoginScreen
import com.subsenpai.ui.PremiumScreen
import com.subsenpai.ui.ReaderScreen
import com.subsenpai.ui.WatchlistScreen

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContent {
            com.subsenpai.ui.SubSenpaiTheme {
                Surface(color = MaterialTheme.colorScheme.background) {
                    val context = LocalContext.current
                    val nav = rememberNavController()
                    val repo = remember { CatalogRepository(auth = AuthStore(context)) }
                    var authVersion by remember { mutableStateOf(0) }
                    NavHost(navController = nav, startDestination = "home") {
                        composable("home") {
                            HomeScreen(
                                repo,
                                authVersion = authVersion,
                                onOpenTitle = { nav.navigate("detail/$it") },
                                onOpenEpisode = { nav.navigate("player/$it") },
                                onLogin = { nav.navigate("login") },
                                onOpenWatchlist = { nav.navigate("watchlist") },
                                onOpenPremium = { nav.navigate("premium") },
                                onOpenChat = { nav.navigate("chat") },
                                onOpenComics = { nav.navigate("comics") },
                            )
                        }
                        composable("comics") {
                            ComicBrowseScreen(
                                onBack = { nav.popBackStack() },
                                onOpenSeries = { s ->
                                    com.subsenpai.data.ComicNav.series = s
                                    nav.navigate("comicdetail")
                                },
                            )
                        }
                        composable("comicdetail") {
                            ComicDetailScreen(
                                onBack = { nav.popBackStack() },
                                onRead = { nav.navigate("comicreader") },
                            )
                        }
                        composable("comicreader") {
                            ComicReaderScreen(
                                repo,
                                onBack = { nav.popBackStack() },
                                onNeedLogin = { nav.navigate("login") },
                            )
                        }
                        composable("premium") {
                            PremiumScreen(repo, onBack = { nav.popBackStack() })
                        }
                        composable("chat") {
                            ChatListScreen(
                                repo,
                                onOpenRoom = { id, name -> nav.navigate("room/$id/$name") },
                                onOpenFriends = { nav.navigate("friends") },
                                onOpenGroups = { nav.navigate("groups") },
                                onBack = { nav.popBackStack() },
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
                                onRead = { nav.navigate("reader/$it") },
                                onBack = { nav.popBackStack() },
                                onNeedLogin = { nav.navigate("login") },
                                onOpenDeviceSource = { nav.navigate("devicesrc/$it") },
                            )
                        }
                        composable(
                            "devicesrc/{id}",
                            arguments = listOf(navArgument("id") { type = NavType.StringType }),
                        ) { entry ->
                            DeviceReaderScreen(
                                repo,
                                entry.arguments!!.getString("id")!!,
                                onBack = { nav.popBackStack() },
                                onNeedLogin = { nav.navigate("login") },
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
                            )
                        }
                        composable(
                            "reader/{chapterId}",
                            arguments = listOf(navArgument("chapterId") { type = NavType.StringType }),
                        ) { entry ->
                            ReaderScreen(
                                repo,
                                entry.arguments!!.getString("chapterId")!!,
                                onBack = { nav.popBackStack() },
                                onOpenChapter = { target ->
                                    nav.navigate("reader/$target") {
                                        popUpTo("reader/{chapterId}") { inclusive = true }
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
