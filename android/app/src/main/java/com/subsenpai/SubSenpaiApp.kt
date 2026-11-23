package com.subsenpai

import android.app.Application
import coil.ImageLoader
import coil.ImageLoaderFactory
import coil.disk.DiskCache
import coil.memory.MemoryCache

/**
 * Loader gambar global dengan batas cache yang tegas
 * (v0.24.12 — perbaikan nge-lag/crash saat menggulir yang
 * dilaporkan Rams 2026-11-23 malam): cache memori maks 18% RAM
 * dan cache disk maks 200 MB, agar poster resolusi raksasa dari
 * situs sumber tidak menumpuk tanpa batas di perangkat.
 */
class SubSenpaiApp : Application(), ImageLoaderFactory {
    override fun newImageLoader(): ImageLoader =
        ImageLoader.Builder(this)
            .memoryCache {
                MemoryCache.Builder(this).maxSizePercent(0.18).build()
            }
            .diskCache {
                DiskCache.Builder()
                    .directory(cacheDir.resolve("image_cache"))
                    .maxSizeBytes(200L * 1024 * 1024)
                    .build()
            }
            .crossfade(true)
            .build()
}
