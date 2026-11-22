package com.subsenpai.data

import android.content.Context

/**
 * Penyimpanan sesi login di perangkat: JWT backend + ringkasan profil.
 * JWT adalah kredensial sesi — disimpan di preferensi privat aplikasi.
 */
class AuthStore(context: Context) {
    private val prefs = context.getSharedPreferences("subsenpai-auth", Context.MODE_PRIVATE)

    val token: String? get() = prefs.getString("token", null)
    val viewingProfileId: String? get() = prefs.getString("vp_id", null)
    val email: String? get() = prefs.getString("email", null)
    val displayName: String? get() = prefs.getString("display_name", null)
    val friendCode: String? get() = prefs.getString("friend_code", null)
    val isLoggedIn: Boolean get() = token != null

    fun save(token: String, viewingProfileId: String?, email: String?, displayName: String?, friendCode: String?) {
        prefs.edit()
            .putString("token", token)
            .putString("vp_id", viewingProfileId)
            .putString("email", email)
            .putString("display_name", displayName)
            .putString("friend_code", friendCode)
            .apply()
    }

    fun clear() {
        prefs.edit().clear().apply()
    }
}

data class UserProfile(
    val id: String,
    val email: String,
    val displayName: String?,
    val avatarUrl: String?,
    val role: String,
    val isPremium: Boolean,
    val appLanguage: String,
    val friendCode: String?,
)

data class WatchHistoryItem(
    val episodeId: String,
    val titleId: String,
    val title: String,
    val posterUrl: String?,
    val episodeNo: Int,
    val positionSec: Int,
)
