package com.subsenpai.ui

import android.content.Context
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
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
import androidx.compose.runtime.LaunchedEffect
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
import com.google.android.gms.auth.api.signin.GoogleSignIn
import com.google.android.gms.auth.api.signin.GoogleSignInOptions
import com.google.android.gms.common.api.ApiException
import com.google.android.libraries.identity.googleid.GetGoogleIdOption
import com.google.android.libraries.identity.googleid.GetSignInWithGoogleOption
import com.google.android.libraries.identity.googleid.GoogleIdTokenCredential
import com.subsenpai.BuildConfig
import com.subsenpai.data.CatalogRepository
import com.subsenpai.data.PremiumStatus
import kotlinx.coroutines.launch

/** Kegagalan berantai login Google — membawa rincian tiap jalur. */
private class GoogleLoginException(val detail: String) : Exception(detail)

private fun describeCredentialError(e: Exception): String = when (e) {
    is GetCredentialCancellationException ->
        "alur Google dibatalkan (" + (e.message ?: "tanpa keterangan") + ")"
    is NoCredentialException ->
        "Credential Manager tidak menemukan akun Google untuk aplikasi ini"
    else -> e.message ?: e.javaClass.simpleName
}

/**
 * Mengambil ID token Google lewat Credential Manager dengan dua jalur
 * berantai dalam satu ketukan:
 *  1) GetSignInWithGoogleOption — alur masuk Google layar penuh.
 *  2) GetGoogleIdOption — bottom sheet pemilih akun.
 * Bila keduanya gagal, GoogleLoginException membawa rincian keduanya
 * dan pemanggil melanjutkan ke jalur ketiga (intent klasik).
 */
private suspend fun fetchGoogleIdToken(context: Context): String {
    val manager = CredentialManager.create(context)
    val webId = BuildConfig.GOOGLE_WEB_CLIENT_ID
    var firstDetail: String
    try {
        val option = GetSignInWithGoogleOption.Builder(webId).build()
        val result = manager.getCredential(
            context,
            GetCredentialRequest.Builder().addCredentialOption(option).build(),
        )
        return extractIdToken(result.credential)
    } catch (first: Exception) {
        firstDetail = describeCredentialError(first)
    }
    try {
        val option = GetGoogleIdOption.Builder()
            .setServerClientId(webId)
            .setFilterByAuthorizedAccounts(false)
            .build()
        val result = manager.getCredential(
            context,
            GetCredentialRequest.Builder().addCredentialOption(option).build(),
        )
        return extractIdToken(result.credential)
    } catch (second: Exception) {
        throw GoogleLoginException(
            "Layar penuh: " + firstDetail + ". Bottom sheet: " +
                describeCredentialError(second) + ".",
        )
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

/** Memetakan kode status ApiException jalur klasik ke teks diagnosis. */
private fun describeApiException(e: ApiException): String = when (e.statusCode) {
    12501 -> "kamu membatalkan pilihan akun di alur klasik"
    10 -> "konfigurasi Google tidak cocok (kode 10 DEVELOPER_ERROR) — " +
        "paket aplikasi atau sidik jari SHA-1 di Google Cloud tidak sama dengan aplikasi ini"
    12500 -> "alur klasik gagal di sisi Google (kode 12500)"
    7 -> "gangguan jaringan saat menghubungi Google (kode 7)"
    else -> "alur klasik gagal (kode " + e.statusCode + ": " + (e.message ?: "-") + ")"
}

/**
 * Layar akun: masuk dengan Google (satu-satunya cara daftar/masuk),
 * tukar token ID Google dengan JWT backend, tampilkan ringkasan akun.
 * Tiga jalur login dicoba berurutan dalam satu ketukan: dua alur
 * Credential Manager, lalu intent klasik Play Services yang paling
 * toleran di perangkat yang Credential Manager-nya rewel (mis. sebagian
 * HyperOS/MIUI). Tombol aktif hanya bila GOOGLE_WEB_CLIENT_ID terisi.
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
    var cmDetail by remember { mutableStateOf<String?>(null) }
    var loggedIn by remember { mutableStateOf(repo.isLoggedIn) }
    var premium by remember { mutableStateOf<PremiumStatus?>(null) }
    val auth = repo.auth

    // Masa aktif premium ditampilkan di halaman profil ini
    // (permintaan Rams 2026-11-22 — dipindah dari halaman Premium).
    LaunchedEffect(loggedIn) {
        premium = if (loggedIn) {
            try {
                repo.premiumStatus()
            } catch (e: Exception) {
                null
            }
        } else {
            null
        }
    }
    val clientConfigured = BuildConfig.GOOGLE_WEB_CLIENT_ID.isNotBlank()

    // Jalur ketiga: intent klasik Google Sign-In (Play Services Auth).
    val legacyLauncher = rememberLauncherForActivityResult(
        ActivityResultContracts.StartActivityForResult(),
    ) { result ->
        try {
            val task = GoogleSignIn.getSignedInAccountFromIntent(result.data)
            val account = task.getResult(ApiException::class.java)
            val token = account.idToken
            if (token.isNullOrBlank()) {
                error = "Google tidak memberikan token. " + (cmDetail ?: "")
                busy = false
            } else {
                scope.launch {
                    try {
                        repo.loginGoogle(token)
                        loggedIn = true
                        onAuthChanged()
                    } catch (e: Exception) {
                        error = e.message ?: "Masuk gagal."
                    } finally {
                        busy = false
                    }
                }
            }
        } catch (e: ApiException) {
            error = "Login belum berhasil — " + describeApiException(e) +
                (cmDetail?.let { "\n\nRincian jalur lain: $it" } ?: "")
            busy = false
        } catch (e: Exception) {
            error = e.message ?: "Masuk gagal."
            busy = false
        }
    }

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
            premium?.let { ps ->
                Spacer(Modifier.height(8.dp))
                if (ps.isPremium) {
                    Text(
                        "★ Premium aktif" +
                            (ps.premiumUntil?.let { " sampai " + it.take(10) } ?: ""),
                        style = MaterialTheme.typography.titleSmall,
                        color = Color(0xFF4CAF50),
                    )
                } else {
                    Text("Akun gratis", style = MaterialTheme.typography.titleSmall)
                    if (ps.trialAvailable) {
                        OutlinedButton(
                            onClick = {
                                scope.launch {
                                    try {
                                        val until = repo.claimTrial()
                                        premium = PremiumStatus(
                                            tier = "premium",
                                            isPremium = true,
                                            premiumUntil = until,
                                            trialAvailable = false,
                                        )
                                    } catch (e: Exception) {
                                        error = e.message
                                    }
                                }
                            },
                            modifier = Modifier.fillMaxWidth(),
                        ) { Text("Klaim trial premium gratis 3 hari") }
                    }
                }
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
                    cmDetail = null
                    scope.launch {
                        try {
                            val idToken = fetchGoogleIdToken(context)
                            repo.loginGoogle(idToken)
                            loggedIn = true
                            onAuthChanged()
                            busy = false
                        } catch (e: GoogleLoginException) {
                            // Dua jalur Credential Manager gagal — lanjut
                            // ke intent klasik; busy tetap true sampai
                            // callback launcher selesai.
                            cmDetail = e.detail
                            val gso = GoogleSignInOptions.Builder(GoogleSignInOptions.DEFAULT_SIGN_IN)
                                .requestIdToken(BuildConfig.GOOGLE_WEB_CLIENT_ID)
                                .requestEmail()
                                .build()
                            val client = GoogleSignIn.getClient(context, gso)
                            client.signOut().addOnCompleteListener {
                                legacyLauncher.launch(client.signInIntent)
                            }
                        } catch (e: Exception) {
                            error = e.message ?: "Masuk gagal."
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
