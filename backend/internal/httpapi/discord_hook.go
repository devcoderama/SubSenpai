package httpapi

import (
	"context"
	"log"
	"time"

	"subsenpai/backend/internal/discord"
)

// discordNotify mengirim pesan ke channel Discord komunitas secara
// fire-and-forget: kegagalan Discord tidak pernah menggagalkan
// operasi utama (pengumuman aplikasi, simpan pengaturan, dst).
// Token/channel kosong = diam (fitur nonaktif).
func (a *API) discordNotify(channelID, content string) {
	if a.cfg.DiscordBotToken == "" || channelID == "" {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if err := discord.New(a.cfg.DiscordBotToken).Send(ctx, channelID, content); err != nil {
			log.Printf("discord: kirim ke channel %s gagal: %v", channelID, err)
		}
	}()
}
