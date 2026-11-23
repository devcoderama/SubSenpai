package com.subsenpai.data

import android.content.Context
import android.os.Build
import com.subsenpai.BuildConfig
import org.json.JSONObject
import java.io.File
import java.io.PrintWriter
import java.io.StringWriter

/**
 * Pelapor crash sederhana: saat aplikasi crash, laporan (pesan +
 * stack trace + info perangkat) disimpan ke berkas lokal, lalu
 * diunggah ke server pada peluncuran berikutnya dan tampil di
 * dashboard admin › Laporan Crash. Handler bawaan tetap dipanggil
 * agar perilaku crash tidak berubah.
 */
object CrashReporter {
    private const val FILE_NAME = "pending_crash.json"

    fun install(context: Context) {
        val appContext = context.applicationContext
        val previous = Thread.getDefaultUncaughtExceptionHandler()
        Thread.setDefaultUncaughtExceptionHandler { thread, error ->
            try {
                val sw = StringWriter()
                error.printStackTrace(PrintWriter(sw))
                val body = JSONObject()
                    .put("app_version", BuildConfig.VERSION_NAME)
                    .put("version_code", BuildConfig.VERSION_CODE)
                    .put(
                        "android_version",
                        "Android ${Build.VERSION.RELEASE} (SDK ${Build.VERSION.SDK_INT})",
                    )
                    .put("device", "${Build.BRAND} ${Build.MODEL}".trim())
                    .put("message", error.toString())
                    .put("stack_trace", sw.toString().take(8000))
                File(appContext.filesDir, FILE_NAME).writeText(body.toString())
            } catch (_: Exception) {
                // Jangan pernah mengganggu jalur crash bawaan.
            }
            previous?.uncaughtException(thread, error)
        }
    }

    /** Unggah laporan tertunda (bila ada); hapus berkas bila terkirim. */
    suspend fun uploadPending(context: Context, repo: CatalogRepository) {
        val file = File(context.applicationContext.filesDir, FILE_NAME)
        if (!file.exists()) return
        try {
            val body = JSONObject(file.readText())
            repo.uploadCrashReport(body)
            file.delete()
        } catch (_: Exception) {
            // Gagal unggah (mis. offline) — coba lagi peluncuran berikut.
        }
    }
}
