package com.subsenpai

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.runtime.remember
import androidx.navigation.NavType
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController
import androidx.navigation.navArgument
import com.subsenpai.data.CatalogRepository
import com.subsenpai.player.PlayerScreen
import com.subsenpai.ui.DetailScreen
import com.subsenpai.ui.HomeScreen
import com.subsenpai.ui.ReaderScreen

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContent {
            MaterialTheme {
                Surface {
                    val nav = rememberNavController()
                    val repo = remember { CatalogRepository() }
                    NavHost(navController = nav, startDestination = "home") {
                        composable("home") {
                            HomeScreen(repo, onOpenTitle = { nav.navigate("detail/$it") })
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
                            )
                        }
                        composable(
                            "player/{episodeId}",
                            arguments = listOf(navArgument("episodeId") { type = NavType.StringType }),
                        ) { entry ->
                            PlayerScreen(repo, entry.arguments!!.getString("episodeId")!!) {
                                nav.popBackStack()
                            }
                        }
                        composable(
                            "reader/{chapterId}",
                            arguments = listOf(navArgument("chapterId") { type = NavType.StringType }),
                        ) { entry ->
                            ReaderScreen(repo, entry.arguments!!.getString("chapterId")!!) {
                                nav.popBackStack()
                            }
                        }
                    }
                }
            }
        }
    }
}
