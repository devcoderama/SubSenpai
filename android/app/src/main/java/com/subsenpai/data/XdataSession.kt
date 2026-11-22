package com.subsenpai.data

import android.util.Base64
import org.bouncycastle.crypto.agreement.X25519Agreement
import org.bouncycastle.crypto.generators.HKDFBytesGenerator
import org.bouncycastle.crypto.generators.X25519KeyPairGenerator
import org.bouncycastle.crypto.params.HKDFParameters
import org.bouncycastle.crypto.params.X25519KeyGenerationParameters
import org.bouncycastle.crypto.params.X25519PrivateKeyParameters
import org.bouncycastle.crypto.params.X25519PublicKeyParameters
import org.bouncycastle.crypto.digests.SHA256Digest
import java.security.SecureRandom
import javax.crypto.Cipher
import javax.crypto.spec.GCMParameterSpec
import javax.crypto.spec.SecretKeySpec

/**
 * Sesi enkripsi xdata sisi aplikasi — pasangan handshake ECDH di backend
 * (STRUKTUR.md Bagian 11). Kunci sesi hanya hidup di memori proses.
 *
 * - X25519 sementara per handshake (Bouncy Castle; API platform tidak
 *   menyediakan X25519 untuk semua versi Android yang didukung).
 * - Kunci sesi = HKDF-SHA256(rahasia bersama, info "subsenpai-xdata-v1").
 * - Dekripsi envelope {xdata, xtime}: AES-256-GCM,
 *   xdata = base64url-nopad(IV 12 byte ‖ ciphertext ‖ tag), AAD = teks xtime.
 */
class XdataSession {
    /**
     * Pasangan id sesi + kunci yang tak terpisahkan. Permintaan API
     * mengambil satu snapshot lalu memakai id-nya untuk header dan
     * kuncinya untuk dekripsi responsnya sendiri — jadi handshake
     * ulang oleh permintaan lain yang berjalan bersamaan tidak bisa
     * merusak dekripsi yang sedang berlangsung (BAD_DECRYPT).
     */
    class Snapshot(val id: String, val key: ByteArray)

    @Volatile
    private var current: Snapshot? = null

    val isReady: Boolean get() = current != null
    val id: String? get() = current?.id

    fun snapshot(): Snapshot? = current

    /** Buang sesi (dipakai saat server menjawab 401 — handshake ulang). */
    fun reset() {
        current = null
    }

    /** Buang sesi hanya bila yang tersimpan masih snapshot ini. */
    fun resetIfCurrent(snap: Snapshot) {
        if (current?.id == snap.id) current = null
    }

    /** Hasil persiapan handshake: kunci publik klien untuk dikirim ke server. */
    class Prepared(val sendPublicKeyB64: String, internal val priv: X25519PrivateKeyParameters)

    fun prepare(): Prepared {
        val gen = X25519KeyPairGenerator()
        gen.init(X25519KeyGenerationParameters(SecureRandom()))
        val kp = gen.generateKeyPair()
        val pub = kp.public as X25519PublicKeyParameters
        return Prepared(b64encode(pub.encoded), kp.private as X25519PrivateKeyParameters)
    }

    /** Selesaikan handshake dengan kunci publik server dari response. */
    fun complete(prepared: Prepared, newSessionId: String, serverPublicKeyB64: String) {
        val serverPub = X25519PublicKeyParameters(b64decode(serverPublicKeyB64))
        val agreement = X25519Agreement()
        agreement.init(prepared.priv)
        val shared = ByteArray(agreement.agreementSize)
        agreement.calculateAgreement(serverPub, shared, 0)
        val hkdf = HKDFBytesGenerator(SHA256Digest())
        hkdf.init(HKDFParameters(shared, null, "subsenpai-xdata-v1".toByteArray(Charsets.UTF_8)))
        val derived = ByteArray(32)
        hkdf.generateBytes(derived, 0, derived.size)
        current = Snapshot(newSessionId, derived)
    }

    /** Buka envelope dengan snapshot sesi yang dipakai permintaannya. */
    fun open(snap: Snapshot, xdata: String, xtime: Long): String {
        val k = snap.key
        val payload = b64decode(xdata)
        require(payload.size > 12) { "xdata terlalu pendek" }
        val nonce = payload.copyOfRange(0, 12)
        val ct = payload.copyOfRange(12, payload.size)
        val cipher = Cipher.getInstance("AES/GCM/NoPadding")
        cipher.init(Cipher.DECRYPT_MODE, SecretKeySpec(k, "AES"), GCMParameterSpec(128, nonce))
        cipher.updateAAD(xtime.toString().toByteArray(Charsets.UTF_8))
        return String(cipher.doFinal(ct), Charsets.UTF_8)
    }

    companion object {
        fun b64encode(b: ByteArray): String =
            Base64.encodeToString(b, Base64.URL_SAFE or Base64.NO_PADDING or Base64.NO_WRAP)

        fun b64decode(s: String): ByteArray =
            Base64.decode(s, Base64.URL_SAFE or Base64.NO_PADDING or Base64.NO_WRAP)
    }
}
