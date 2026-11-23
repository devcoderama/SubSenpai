package httpapi

import (
	"context"
	"log"
	"time"

	"subsenpai/backend/internal/telegram"
)

// telegramNotify mengirim pesan ke satu topik grup Telegram
// komunitas secara fire-and-forget — cermin dari discordNotify.
// Kegagalan Telegram tidak pernah menggagalkan operasi utama.
// Token/grup/topik kosong = diam (fitur nonaktif). Konten gaya
// Discord (**tebal**) diubah ke HTML Telegram di sini.
func (a *API) telegramNotify(topicID, content string) {
	if a.cfg.TelegramBotToken == "" || a.cfg.TelegramGroupID == "" || topicID == "" {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if err := telegram.New(a.cfg.TelegramBotToken).Send(ctx,
			a.cfg.TelegramGroupID, topicID, telegram.ToHTML(content)); err != nil {
			log.Printf("telegram: kirim ke topik %s gagal: %v", topicID, err)
		}
	}()
}
