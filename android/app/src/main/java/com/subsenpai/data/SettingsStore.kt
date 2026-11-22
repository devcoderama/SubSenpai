package com.subsenpai.data

import android.content.Context

/** Preferensi tampilan lokal (SharedPreferences, tanpa server). */
class SettingsStore(context: Context) {
    private val prefs = context.getSharedPreferences("subsenpai-settings", Context.MODE_PRIVATE)

    /** Poster resolusi tinggi (TMDB 780px) di hero & kepala detail. */
    var posterHiRes: Boolean
        get() = prefs.getBoolean("poster_hires", true)
        set(v) {
            prefs.edit().putBoolean("poster_hires", v).apply()
        }
}
