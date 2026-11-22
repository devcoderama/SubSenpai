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

// ---------- Fase 4: premium, sosial ----------

data class PlanItem(val id: String, val days: Int, val price: Int, val label: String)

/** Metode pembayaran Tokopay dari database (migrasi 0015). */
data class PaymentMethodItem(val code: String, val name: String, val group: String, val iconUrl: String)

data class PremiumStatus(
    val tier: String,
    val isPremium: Boolean,
    val premiumUntil: String?,
    val trialAvailable: Boolean,
)

data class VoucherItem(
    val code: String,
    val days: Int,
    val source: String,
    val status: String = "active",
    val createdByMe: Boolean = false,
    val forMe: Boolean = false,
    val paymentStatus: String? = null,
    val redeemedByName: String? = null,
)

/** Satu baris riwayat transaksi (sesi pembayaran). */
data class TransactionItem(
    val id: String,
    val purpose: String,
    val provider: String,
    val planLabel: String,
    val amount: Int,
    val status: String,
    val createdAt: String,
)

data class ChatChannelItem(val id: String, val name: String, val kind: String)

data class ChatMsg(
    val id: String,
    val userId: String,
    val displayName: String,
    val nameColor: String?,
    val nameEmoji: String?,
    val body: String?,
    val replyToId: String?,
    val replyPreview: String?,
    val deleted: Boolean,
) {
    companion object {
        fun fromJson(o: org.json.JSONObject): ChatMsg = ChatMsg(
            id = o.getString("id"),
            userId = o.optString("user_id"),
            displayName = o.optString("display_name", "Pengguna"),
            nameColor = o.optString("name_color").ifBlank { null },
            nameEmoji = o.optString("name_emoji").ifBlank { null },
            body = if (o.isNull("body")) null else o.optString("body"),
            replyToId = o.optString("reply_to_id").ifBlank { null },
            replyPreview = o.optString("reply_preview").ifBlank { null },
            deleted = o.optBoolean("deleted"),
        )
    }
}

data class FriendItem(
    val userId: String,
    val email: String,
    val displayName: String?,
    val status: String, // accepted | pending_in | pending_out
)

data class GroupItem(
    val id: String,
    val name: String,
    val isPublic: Boolean,
    val isMember: Boolean,
    val inviteCode: String?,
    val channelId: String?,
)
