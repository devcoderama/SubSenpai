plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
    id("org.jetbrains.kotlin.plugin.compose")
}

android {
    namespace = "com.subsenpai"
    compileSdk = 34

    defaultConfig {
        applicationId = "com.subsenpai"
        minSdk = 24
        targetSdk = 34
        versionCode = 8
        versionName = "0.6.1"
        // Alamat backend bawaan untuk pengembangan (emulator → host).
        // Rilis memakai alamat resmi dari pengaturan aplikasi.
        // Alamat API: bawaan emulator; build produksi menimpa lewat
        // -PapiBaseUrl=https://api.contoh.com (tanpa garis miring akhir).
        val apiBaseUrl = providers.gradleProperty("apiBaseUrl").orElse("http://10.0.2.2:8080").get()
        buildConfigField("String", "API_BASE_URL", "\"$apiBaseUrl\"")
        // OAuth client ID (tipe web) untuk Google Sign-In; diisi saat
        // proyek Google Cloud dibuat. Kosong = tombol masuk nonaktif.
        // Web Client ID Google untuk Sign-In; diisi saat build lewat
        // -PgoogleWebClientId=<id>.apps.googleusercontent.com (kosong = login nonaktif).
        val googleWebClientId = providers.gradleProperty("googleWebClientId").orElse("").get()
        buildConfigField("String", "GOOGLE_WEB_CLIENT_ID", "\"$googleWebClientId\"")
    }

    buildTypes {
        release {
            isMinifyEnabled = false
        }
    }
    buildFeatures {
        compose = true
        buildConfig = true
    }
    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
    kotlinOptions {
        jvmTarget = "17"
    }
}

dependencies {
    val composeBom = platform("androidx.compose:compose-bom:2024.09.00")
    implementation(composeBom)
    implementation("androidx.core:core-ktx:1.13.1")
    implementation("androidx.activity:activity-compose:1.9.2")
    implementation("androidx.compose.material3:material3")
    implementation("androidx.compose.ui:ui")
    implementation("androidx.compose.ui:ui-tooling-preview")
    implementation("androidx.lifecycle:lifecycle-viewmodel-compose:2.8.4")
    implementation("androidx.navigation:navigation-compose:2.7.7")

    // Jaringan: klien API backend Go
    implementation("com.squareup.okhttp3:okhttp:4.12.0")
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-android:1.8.1")

    // Kriptografi sesi xdata: X25519 tidak tersedia di API Android lama,
    // memakai Bouncy Castle (hanya kurva & HKDF yang dipakai).
    implementation("org.bouncycastle:bcprov-jdk18on:1.78.1")

    // Player HLS (Media3)
    implementation("androidx.media3:media3-exoplayer:1.4.1")
    implementation("androidx.media3:media3-exoplayer-hls:1.4.1")
    implementation("androidx.media3:media3-ui:1.4.1")

    // Gambar (poster & halaman)
    implementation("io.coil-kt:coil-compose:2.7.0")

    // Google Sign-In (Credential Manager) — Fase 3
    // OCR balon teks di perangkat (F31) — ML Kit, model terbundel.
    implementation("com.google.mlkit:text-recognition:16.0.1")
    implementation("com.google.mlkit:text-recognition-chinese:16.0.1")
    implementation("com.google.mlkit:text-recognition-japanese:16.0.1")
    implementation("com.google.mlkit:text-recognition-korean:16.0.1")
    implementation("androidx.credentials:credentials:1.3.0")
    implementation("androidx.credentials:credentials-play-services-auth:1.3.0")
    implementation("com.google.android.libraries.identity.googleid:googleid:1.1.1")
}
