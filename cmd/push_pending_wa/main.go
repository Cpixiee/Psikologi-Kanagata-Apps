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

func resolveName(name, fallback string) string {
	if name != "" {
		return name
	}
	return fallback
}

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

	fmt.Printf("Ditemukan %d undangan pending. Memulai pengiriman token via WHATSAPP (Fonnte)...\n", len(invs))
	sentWA := 0
	waConfig := utils.GetWhatsAppConfig()

	for i := range invs {
		if i > 0 {
			time.Sleep(1200 * time.Millisecond) // 1.2s delay between messages for Fonnte stability
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
		phoneToUse := strings.TrimSpace(inv.Phone)
		if inv.UserId != nil && *inv.UserId != 0 {
			var u models.User
			u.Id = *inv.UserId
			if err := o.Read(&u); err == nil {
				displayName = u.NamaLengkap
				if phoneToUse == "" {
					phoneToUse = strings.TrimSpace(u.NoHandphone)
				}
			}
		}

		if phoneToUse == "" {
			fmt.Printf("[%d/%d] SKIP WA -> Email: %s (Tidak ada nomor HP)\n", i+1, len(invs), inv.Email)
			continue
		}

		link := fmt.Sprintf("%s/test?token=%s", utils.GetAppBaseURL(), inv.Token)
		msg := fmt.Sprintf("Halo %s,\n\nBerikut Kode (Token) untuk mengikuti tes psikologi *%s*:\n\n*KODE: %s*\n\nBuka link berikut untuk mulai mengerjakan:\n%s\n\nToken berlaku hingga: %s",
			resolveName(displayName, inv.Email), batch.Name, inv.Token, link, inv.ExpiresAt.Format("02 Jan 2006 15:04"))

		var lastErr error
		success := false
		for attempt := 1; attempt <= 3; attempt++ {
			if attempt > 1 {
				time.Sleep(2 * time.Second)
			}
			if err := utils.SendWhatsApp(waConfig, phoneToUse, msg); err == nil {
				sentWA++
				fmt.Printf("[%d/%d] BERHASIL WA -> HP: %s (%s), Token: %s (attempt %d)\n", sentWA, len(invs), phoneToUse, inv.Email, inv.Token, attempt)
				success = true
				break
			} else {
				lastErr = err
			}
		}

		if !success {
			fmt.Printf("[%d/%d] GAGAL WA -> HP: %s: %v\n", i+1, len(invs), phoneToUse, lastErr)
		}
	}
	fmt.Printf("\n=== SELESAI ===\nTotal %d token berhasil dikirim via WhatsApp.\n", sentWA)
	os.Exit(0)
}
