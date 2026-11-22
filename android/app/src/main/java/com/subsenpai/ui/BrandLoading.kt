package com.subsenpai.ui

import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.height
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/** Warna aksen merek (merah ala Netflix) untuk indikator muat. */
val BrandRed = Color(0xFFE50914)

/**
 * Animasi memuat ala Netflix: wordmark "SubSenpai" yang berdenyut
 * halus di atas indikator lingkaran beraksen merek, di tengah
 * wadah. Dipakai menggantikan halaman kosong selama data/video
 * dimuat (Beranda, Detail, dan wadah player). [compact] memakai
 * ukuran lebih kecil untuk wadah 16:9 player.
 */
@Composable
fun BrandLoading(
    modifier: Modifier = Modifier,
    accent: Color = BrandRed,
    compact: Boolean = false,
) {
    val transition = rememberInfiniteTransition(label = "brandPulse")
    val pulse by transition.animateFloat(
        initialValue = 0.35f,
        targetValue = 1f,
        animationSpec = infiniteRepeatable(
            animation = tween(durationMillis = 900),
            repeatMode = RepeatMode.Reverse,
        ),
        label = "brandPulseAlpha",
    )
    Box(modifier = modifier, contentAlignment = Alignment.Center) {
        Column(horizontalAlignment = Alignment.CenterHorizontally) {
            Text(
                "SubSenpai",
                color = Color.White,
                fontSize = if (compact) 20.sp else 30.sp,
                fontWeight = FontWeight.Bold,
                modifier = Modifier.alpha(pulse),
            )
            Spacer(Modifier.height(if (compact) 12.dp else 20.dp))
            CircularProgressIndicator(
                color = accent,
                modifier = Modifier.height(if (compact) 30.dp else 44.dp),
            )
        }
    }
}

/** BrandLoading layar penuh di atas latar gelap aplikasi. */
@Composable
fun BrandLoadingScreen(accent: Color = BrandRed) {
    Box(
        modifier = Modifier.fillMaxSize().background(Color(0xFF141414)),
        contentAlignment = Alignment.Center,
    ) {
        BrandLoading(accent = accent)
    }
}
