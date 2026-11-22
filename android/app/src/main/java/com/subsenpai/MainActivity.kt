package com.subsenpai

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.NavigationBar
import androidx.compose.material3.NavigationBarItem
import androidx.compose.material3.NavigationBarItemDefaults
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.painterResource
import androidx.navigation.NavType
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.currentBackStackEntryAsState
import androidx.navigation.compose.rememberNavController
import androidx.navigation.navArgument
import com.subsenpai.data.AuthStore
import com.subsenpai.data.CatalogRepository
import com.subsenpai.player.PlayerScreen
import com.subsenpai.ui.ChatListScreen
import com.subsenpai.ui.ChatRoomScreen
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
        setContent {
            com.subsenpai.ui.SubSenpaiTheme {
                val context = LocalContext.current
                val nav = rememberNavController()
                val repo = remember { CatalogRepository(auth = AuthStore(context)) }
                var authVersion by remember { mutableStateOf(0) }
                val backEntry by nav.currentBackStackEntryAsState()
                val route = backEntry?.destination?.route
                // Bilah bawah ala Netflix: hanya di layar-layar utama.
                val mainRoutes = setOf("home", "chat", "premium", "watchlist", "login")
                val bottomItems = listOf(
                    Triple("home", "Beranda", R.drawable.ic_home),
                    Triple("chat", "Chat", R.drawable.ic_chat),
                    Triple("premium", "Premium", R.drawable.ic_star),
                    Triple("watchlist", "Daftar Saya", R.drawable.ic_bookmark),
                    Triple("login", "Akun", R.drawable.ic_person),
                )
                Scaffold(
                    containerColor = MaterialTheme.colorScheme.background,
                    bottomBar = {
                        if (route in mainRoutes) {
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
                    NavHost(navController = nav, startDestination = "home") {
                        composable("home") {
                            HomeScreen(
                                repo,
                                authVersion = authVersion,
                                onOpenTitle = { nav.navigate("detail/$it") },
                                onOpenEpisode = { nav.navigate("player/$it") },
                                onLogin = { nav.navigate("login") },
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
                                onOpenTitle = { nav.navigate("detail/$it") },
                            )
                        }
                    }
                    }
                }
            }
        }
    }
}
