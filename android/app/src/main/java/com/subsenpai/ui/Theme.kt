package com.subsenpai.ui

import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color

/** Palet tema gelap ala Netflix: hitam pekat + aksen merah. */
val NetflixRed = Color(0xFFE50914)
val NetflixBlack = Color(0xFF141414)
val NetflixSurface = Color(0xFF1F1F1F)
val NetflixSurfaceVar = Color(0xFF2A2A2A)
val NetflixGrey = Color(0xFFB3B3B3)

private val SubSenpaiColors = darkColorScheme(
    primary = NetflixRed,
    onPrimary = Color.White,
    secondary = Color.White,
    onSecondary = Color.Black,
    background = NetflixBlack,
    onBackground = Color.White,
    surface = NetflixSurface,
    onSurface = Color.White,
    surfaceVariant = NetflixSurfaceVar,
    onSurfaceVariant = NetflixGrey,
    error = Color(0xFFFF7A72),
    onError = Color.Black,
)

@Composable
fun SubSenpaiTheme(content: @Composable () -> Unit) {
    MaterialTheme(colorScheme = SubSenpaiColors, content = content)
}
