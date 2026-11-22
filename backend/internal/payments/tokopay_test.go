package payments

import (
	"crypto/md5"
	"encoding/hex"
	"testing"
)

func TestTokopaySignatureFormula(t *testing.T) {
	// Rumus dokumentasi: md5 dari "merchant_id:secret:reff_id".
	sum := md5.Sum([]byte("M20230422TEST26949756:rahasia:TXO20230523TEST13820"))
	want := hex.EncodeToString(sum[:])
	got := TokopaySignature("M20230422TEST26949756", "rahasia", "TXO20230523TEST13820")
	if got != want {
		t.Fatalf("signature = %s, mau %s", got, want)
	}
	if !VerifyTokopayCallback("M20230422TEST26949756", "rahasia", "TXO20230523TEST13820", want) {
		t.Fatal("signature sah harus lolos verifikasi")
	}
	if VerifyTokopayCallback("M20230422TEST26949756", "rahasia", "TXO20230523TEST13820", "00000000000000000000000000000000") {
		t.Fatal("signature palsu tidak boleh lolos")
	}
	if VerifyTokopayCallback("M20230422TEST26949756", "secret-lain", "TXO20230523TEST13820", want) {
		t.Fatal("secret lain tidak boleh lolos")
	}
}

func TestTokopayPaidStatus(t *testing.T) {
	for _, s := range []string{"Success", "Completed"} {
		if !TokopayPaid(s) {
			t.Errorf("%s harus dianggap lunas", s)
		}
	}
	for _, s := range []string{"Unpaid", "Failed", "Proses Callback", ""} {
		if TokopayPaid(s) {
			t.Errorf("%s tidak boleh dianggap lunas", s)
		}
	}
}

func TestTokopayChannels(t *testing.T) {
	for _, code := range []string{"QRIS", "BRIVA", "BCAVA", "DANA", "GOPAY", "SHOPEEPAY", "ALFAMART", "INDOMARET"} {
		if _, ok := TokopayChannels[code]; !ok {
			t.Errorf("channel %s hilang dari daftar", code)
		}
	}
}
