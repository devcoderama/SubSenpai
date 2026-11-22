package com.subsenpai.ui

import android.content.Context
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.credentials.Credential
import androidx.credentials.CredentialManager
import androidx.credentials.CustomCredential
import androidx.credentials.GetCredentialRequest
import androidx.credentials.exceptions.GetCredentialCancellationException
import androidx.credentials.exceptions.NoCredentialException
import com.google.android.libraries.identity.googleid.GetGoogleIdOption
import com.google.android.libraries.identity.googleid.GoogleIdTokenCredential
import com.google.android.libraries.identity.googleid.GetSignInWithGoogleOption
import com.subsenpai.BuildConfig
import com.subsenpai.data.CatalogRepository
import kotlinx.coroutines.launch

/**
 * Mengambil ID token Google lewat Credential Manager.
 * Jalur utama: bottom sheet GetGoogleIdOption. Bila tidak ada kredensial
 * yang tersedia (mis. client OAuth baru dibuat & belum tersebar, atau akun
 * belum pernah mengizinkan aplikasi), jatuh ke alur GetSignInWithGoogleOption
 * (layar masuk Google penuh) yang lebih toleran untuk izin pertama.
 */
private suspend fun fetchGoogleIdToken(context: Context): String {
    val manager = CredentialManager.create(context)
    try {
        val option = GetGoogleIdOption.Builder()
            .setServerClientId(BuildConfig.GOOGLE_WEB_CLIENT_ID)
            .setFilterByAuthorizedAccounts(false)
            .build()
        val result = manager.getCredential(
            context,
            GetCredentialRequest.Builder().addCredentialOption(option).build(),
        )
        return extractIdToken(result.credential)
    } catch (e: NoCredentialException) {
        val option = GetSignInWithGoogleOption.Builder(BuildConfig.GOOGLE_WEB_CLIENT_ID).build()
        val result = manager.getCredential(
            context,
            GetCredentialRequest.Builder().addCredentialOption(option).build(),
        )
        return extractIdToken(result.credential)
    }
}

private fun extractIdToken(credential: Credential): String {
    if (credential is CustomCredential &&
        credential.type == GoogleIdTokenCredential.TYPE_GOOGLE_ID_TOKEN_CREDENTIAL
    ) {
        return GoogleIdTokenCredential.createFrom(credential.data).idToken
    }
    throw IllegalStateException("Kredensial Google tidak dikenal.")
}

/**
 * Layar akun: masuk dengan Google (satu-satunya cara daftar/masuk),
 * tukar token ID Google dengan JWT backend, tampilkan ringkasan akun.
 * Tombol masuk aktif hanya bila GOOGLE_WEB_CLIENT_ID terisi saat build.
 */
@Composable
fun LoginScreen(
    repo: CatalogRepository,
    onBack: () -> Unit,
    onAuthChanged: () -> Unit,
) {
    val context = LocalContext.current
    val scope = rememberCoroutineScope()
    var busy by remember { mutableStateOf(false) }
    var error by remember { mutableStateOf<String?>(null) }
    var loggedIn by remember { mutableStateOf(repo.isLoggedIn) }
    val auth = repo.auth
    val clientConfigured = BuildConfig.GOOGLE_WEB_CLIENT_ID.isNotBlank()

    Column(
        modifier = Modifier.fillMaxSize().padding(20.dp),
        verticalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        TextButton(onClick = onBack) { Text("← Kembali", color = Color.White) }
        Text(
            "SUBSENPAI",
            color = MaterialTheme.colorScheme.primary,
            style = MaterialTheme.typography.headlineMedium,
            fontWeight = FontWeight.Black,
        )
        if (loggedIn && auth != null) {
            Text("Akun", style = MaterialTheme.typography.headlineSmall, fontWeight = FontWeight.Bold)
            Text(auth.displayName ?: "Pengguna SubSenpai", style = MaterialTheme.typography.titleMedium)
            auth.email?.let { Text(it, style = MaterialTheme.typography.bodyMedium) }
            auth.friendCode?.let {
                Text("Kode teman: $it", style = MaterialTheme.typography.bodyMedium)
            }
            Spacer(Modifier.height(8.dp))
            OutlinedButton(
                onClick = {
                    repo.logout()
                    loggedIn = false
                    onAuthChanged()
                },
                modifier = Modifier.fillMaxWidth(),
            ) { Text("Keluar", color = Color.White) }
        } else {
            Text("Masuk", style = MaterialTheme.typography.headlineSmall, fontWeight = FontWeight.Bold)
            Text(
                "Masuk dengan akun Google untuk watchlist, riwayat, dan melanjutkan tontonan di perangkat mana pun.",
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            Spacer(Modifier.height(14.dp))
            Button(
                enabled = clientConfigured && !busy,
                onClick = {
                    busy = true
                    error = null
                    scope.launch {
                        try {
                            val idToken = fetchGoogleIdToken(context)
                            repo.loginGoogle(idToken)
                            loggedIn = true
                            onAuthChanged()
                        } catch (e: GetCredentialCancellationException) {
                            error = "Login dibatalkan."
                        } catch (e: NoCredentialException) {
                            error = "Tidak ada akun Google yang tersedia untuk aplikasi ini. " +
                                "Pastikan ada akun Google yang masuk di HP ini. Bila client Google " +
                                "baru saja dibuat, pengaturannya bisa butuh beberapa menit hingga " +
                                "beberapa jam untuk tersebar — coba lagi nanti."
                        } catch (e: Exception) {
                            error = e.message ?: "Masuk gagal."
                        } finally {
                            busy = false
                        }
                    }
                },
                colors = ButtonDefaults.buttonColors(containerColor = Color.White, contentColor = Color.Black),
                modifier = Modifier.fillMaxWidth(),
            ) { Text(if (busy) "Memproses…" else "Masuk dengan Google", fontWeight = FontWeight.Bold) }
            if (!clientConfigured) {
                Text(
                    "Google Sign-In belum dikonfigurasi pada build ini (GOOGLE_WEB_CLIENT_ID kosong) — " +
                        "perlu client ID dari proyek Google Cloud agar tombol aktif.",
                    style = MaterialTheme.typography.bodySmall,
                )
            }
        }
        if (error != null) {
            Text(error!!, color = MaterialTheme.colorScheme.error)
        }
    }
}
