package com.subsenpai.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Button
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
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import androidx.credentials.CredentialManager
import androidx.credentials.CustomCredential
import androidx.credentials.GetCredentialRequest
import com.google.android.libraries.identity.googleid.GetGoogleIdOption
import com.google.android.libraries.identity.googleid.GoogleIdTokenCredential
import com.subsenpai.BuildConfig
import com.subsenpai.data.CatalogRepository
import kotlinx.coroutines.launch

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
        modifier = Modifier.fillMaxSize().padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        TextButton(onClick = onBack) { Text("← Kembali") }
        Text("Akun", style = MaterialTheme.typography.headlineSmall)
        if (loggedIn && auth != null) {
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
            ) { Text("Keluar") }
        } else {
            Text(
                "Masuk dengan akun Google untuk watchlist, riwayat, dan melanjutkan tontonan di perangkat mana pun.",
                style = MaterialTheme.typography.bodyMedium,
            )
            Spacer(Modifier.height(8.dp))
            Button(
                enabled = clientConfigured && !busy,
                onClick = {
                    busy = true
                    error = null
                    scope.launch {
                        try {
                            val manager = CredentialManager.create(context)
                            val option = GetGoogleIdOption.Builder()
                                .setServerClientId(BuildConfig.GOOGLE_WEB_CLIENT_ID)
                                .setFilterByAuthorizedAccounts(false)
                                .build()
                            val request = GetCredentialRequest.Builder()
                                .addCredentialOption(option)
                                .build()
                            val result = manager.getCredential(context, request)
                            val credential = result.credential
                            if (credential is CustomCredential &&
                                credential.type == GoogleIdTokenCredential.TYPE_GOOGLE_ID_TOKEN_CREDENTIAL
                            ) {
                                val idToken = GoogleIdTokenCredential.createFrom(credential.data).idToken
                                repo.loginGoogle(idToken)
                                loggedIn = true
                                onAuthChanged()
                            } else {
                                error = "Kredensial Google tidak dikenal."
                            }
                        } catch (e: Exception) {
                            error = e.message ?: "Masuk gagal."
                        } finally {
                            busy = false
                        }
                    }
                },
                modifier = Modifier.fillMaxWidth(),
            ) { Text(if (busy) "Memproses…" else "Masuk dengan Google") }
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
