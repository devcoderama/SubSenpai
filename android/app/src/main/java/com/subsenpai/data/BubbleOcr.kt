package com.subsenpai.data

import android.content.Context
import android.graphics.Bitmap
import android.graphics.drawable.BitmapDrawable
import coil.ImageLoader
import coil.request.ImageRequest
import coil.request.SuccessResult
import com.google.mlkit.vision.common.InputImage
import com.google.mlkit.vision.text.TextRecognition
import com.google.mlkit.vision.text.chinese.ChineseTextRecognizerOptions
import com.google.mlkit.vision.text.japanese.JapaneseTextRecognizerOptions
import com.google.mlkit.vision.text.korean.KoreanTextRecognizerOptions
import com.google.mlkit.vision.text.latin.TextRecognizerOptions
import kotlinx.coroutines.suspendCancellableCoroutine
import kotlin.coroutines.resume

/** Satu blok teks hasil OCR + terjemahannya; koordinat pecahan 0..1 dari ukuran gambar. */
data class OcrBubble(
    val original: String,
    val translated: String,
    val left: Float,
    val top: Float,
    val right: Float,
    val bottom: Float,
)

/**
 * F31 di perangkat: OCR balon teks halaman komik memakai ML Kit
 * (di perangkat, tanpa kunci API), lalu tiap blok diterjemahkan memakai
 * penerjemah gtx yang sama dengan subtitle. Hasil hanya di memori —
 * kegagalan di titik mana pun menghasilkan daftar kosong.
 */
object BubbleOcr {

    private val translator = GtxTranslator()

    suspend fun translatePage(
        context: Context,
        imageUrl: String,
        script: String,
        targetLang: String,
    ): List<OcrBubble> {
        val bitmap = loadBitmap(context, imageUrl) ?: return emptyList()
        val blocks = recognize(bitmap, script)
        if (blocks.isEmpty()) return emptyList()
        val w = bitmap.width.toFloat()
        val h = bitmap.height.toFloat()
        return blocks.mapNotNull { (text, rect) ->
            val translated = translator.translateText(text, targetLang) ?: return@mapNotNull null
            OcrBubble(
                original = text,
                translated = translated,
                left = rect.left / w,
                top = rect.top / h,
                right = rect.right / w,
                bottom = rect.bottom / h,
            )
        }
    }

    private suspend fun loadBitmap(context: Context, url: String): Bitmap? {
        return try {
            val loader = ImageLoader(context)
            val req = ImageRequest.Builder(context)
                .data(url)
                .allowHardware(false)
                .build()
            val result = loader.execute(req)
            ((result as? SuccessResult)?.drawable as? BitmapDrawable)?.bitmap
        } catch (e: Exception) {
            null
        }
    }

    private data class Block(val text: String, val rect: android.graphics.Rect)

    private suspend fun recognize(bitmap: Bitmap, script: String): List<Block> =
        suspendCancellableCoroutine { cont ->
            val options = when (script) {
                "zh" -> ChineseTextRecognizerOptions.Builder().build()
                "ja" -> JapaneseTextRecognizerOptions.Builder().build()
                "ko" -> KoreanTextRecognizerOptions.Builder().build()
                else -> TextRecognizerOptions.DEFAULT_OPTIONS
            }
            val recognizer = TextRecognition.getClient(options)
            val image = InputImage.fromBitmap(bitmap, 0)
            recognizer.process(image)
                .addOnSuccessListener { visionText ->
                    val blocks = visionText.textBlocks.mapNotNull { b ->
                        val r = b.boundingBox ?: return@mapNotNull null
                        Block(b.text.replace("\n", " "), r)
                    }
                    recognizer.close()
                    if (cont.isActive) cont.resume(blocks)
                }
                .addOnFailureListener {
                    recognizer.close()
                    if (cont.isActive) cont.resume(emptyList())
                }
        }
}
