package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	_ "github.com/lib/pq"
	beego "github.com/beego/beego/v2/server/web"
	"github.com/beego/beego/v2/client/orm"
	_ "psikologi_apps/models"
	"psikologi_apps/models"
	"psikologi_apps/utils"
)

func main() {
	if err := beego.LoadAppConfig("ini", "conf/app.conf"); err != nil {
		log.Printf("Warning loading app.conf: %v", err)
	}

	o := orm.NewOrm()
	var invs []models.TestInvitation
	_, err := o.QueryTable(new(models.TestInvitation)).Filter("Status", models.StatusInvitationPending).All(&invs)
	if err != nil {
		log.Fatalf("Error querying invitations: %v", err)
	}

	fmt.Printf("Ditemukan %d undangan pending. Memulai pengiriman token ke email...\n", len(invs))
	sent := 0
	for i := range invs {
		if i > 0 {
			time.Sleep(350 * time.Millisecond)
		}
		inv := &invs[i]
		if inv.BatchId == nil {
			continue
		}
		var batch models.TestBatch
		batch.Id = *inv.BatchId
		if err := o.Read(&batch); err != nil {
			continue
		}

		displayName := ""
		if inv.UserId != nil && *inv.UserId != 0 {
			var u models.User
			u.Id = *inv.UserId
			if err := o.Read(&u); err == nil {
				displayName = u.NamaLengkap
			}
		}

		config := utils.GetEmailConfig()
		appURL := utils.GetAppBaseURL()
		link := fmt.Sprintf("%s/test?token=%s", appURL, inv.Token)
		subject := fmt.Sprintf("Kode Tes Psikologi - %s", batch.Name)
		body := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
	<meta charset="UTF-8" />
	<style>
		body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; margin: 0; padding: 0; }
		.container { max-width: 600px; margin: 0 auto; padding: 20px; }
		.header { background-color: #696cff; color: white; padding: 24px; text-align: center; border-radius: 12px 12px 0 0; }
		.content { background-color: #f8f9fa; padding: 30px; border-radius: 0 0 12px 12px; }
		.token-box { background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%); border-radius: 16px; padding: 25px; text-align: center; margin: 20px 0; }
		.token-code { font-size: 32px; font-weight: bold; color: #ffffff; letter-spacing: 8px; font-family: monospace; }
		.button { display: inline-block; padding: 14px 28px; background-color: #696cff; color: white; text-decoration: none; border-radius: 8px; font-weight: 600; }
	</style>
</head>
<body>
	<div class="container">
		<div class="header"><h2>Undangan Tes Psikologi</h2></div>
		<div class="content">
			<p>Halo <strong>%s</strong>,</p>
			<p>Berikut adalah Token Undangan Tes Anda:</p>
			<div class="token-box"><div class="token-code">%s</div></div>
			<p style="text-align: center;"><a class="button" href="%s" target="_blank">Buka Halaman Tes</a></p>
			<p style="font-size: 12px; color: #777;">Gunakan token di atas untuk mengakses tes psikologi Anda.</p>
		</div>
	</div>
</body>
</html>`, strings.TrimSpace(displayName), inv.Token, link)

		err := utils.SendEmail(config, utils.EmailData{To: inv.Email, Subject: subject, Body: body})
		if err == nil {
			sent++
			fmt.Printf("[%d/%d] BERHASIL -> Email: %s, Token: %s\n", sent, len(invs), inv.Email, inv.Token)
		} else {
			fmt.Printf("[%d/%d] GAGAL -> Email: %s: %v\n", i+1, len(invs), inv.Email, err)
		}
	}
	fmt.Printf("\n=== SELESAI ===\nTotal %d dari %d token berhasil dikirim ke email peserta.\n", sent, len(invs))
	os.Exit(0)
}
