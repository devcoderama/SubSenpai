package com.subsenpai.data

import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

/** Satu postingan yang menunggu diunggah di latar belakang. */
class PendingPost(
    val body: String,
    val category: String,
    val visibility: String,
    val groupId: String?,
    val mediaBytes: ByteArray?,
    val isVideo: Boolean,
)

/** Keadaan antrean unggahan feed untuk spanduk status. */
sealed interface FeedUploadState {
    data object Idle : FeedUploadState
    data object Uploading : FeedUploadState
    data object Done : FeedUploadState
    data class Failed(val message: String) : FeedUploadState
}

/**
 * Antrean unggah postingan di latar belakang (permintaan Rams
 * 2026-11-22, ala Facebook/Instagram): pengguna mengetuk Posting,
 * composer langsung bebas, dan unggahan berjalan di cakupan milik
 * manajer ini — tidak terikat layar — sambil statusnya tampil
 * sebagai spanduk di atas feed. Video diunggah langsung dari
 * perangkat ke image2url; kegagalan menyimpan muatan terakhir
 * agar bisa dicoba lagi dari spanduk.
 */
object FeedUploadManager {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private val _state = MutableStateFlow<FeedUploadState>(FeedUploadState.Idle)
    val state: StateFlow<FeedUploadState> = _state.asStateFlow()

    private val queue = ArrayDeque<PendingPost>()
    private var running = false
    private var lastFailed: PendingPost? = null

    fun enqueue(repo: CatalogRepository, post: PendingPost) {
        queue.addLast(post)
        pump(repo)
    }

    fun retry(repo: CatalogRepository) {
        val post = lastFailed ?: return
        lastFailed = null
        _state.value = FeedUploadState.Idle
        enqueue(repo, post)
    }

    fun dismiss() {
        if (_state.value is FeedUploadState.Failed) {
            lastFailed = null
        }
        if (_state.value !is FeedUploadState.Uploading) {
            _state.value = FeedUploadState.Idle
        }
    }

    private fun pump(repo: CatalogRepository) {
        if (running) return
        val next = queue.removeFirstOrNull() ?: return
        running = true
        scope.launch {
            _state.value = FeedUploadState.Uploading
            try {
                val url = next.mediaBytes?.let { bytes ->
                    if (next.isVideo) {
                        repo.feedUploadVideoDirect(bytes)
                            ?: repo.feedUpload(bytes, "feed-${System.currentTimeMillis()}.mp4")
                    } else {
                        repo.feedUpload(bytes, "feed-${System.currentTimeMillis()}.jpg")
                    }
                }
                repo.feedCreate(next.body, url, next.category, next.visibility, next.groupId)
                lastFailed = null
                _state.value = FeedUploadState.Done
            } catch (e: Exception) {
                lastFailed = next
                _state.value = FeedUploadState.Failed(e.message ?: "unggahan gagal")
            } finally {
                running = false
                pump(repo)
            }
        }
    }
}
