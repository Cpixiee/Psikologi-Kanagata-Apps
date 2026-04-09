package controllers

import (
	"strconv"
	"strings"

	"psikologi_apps/models"

	"github.com/beego/beego/v2/client/orm"
	beego "github.com/beego/beego/v2/server/web"
)

type PageController struct {
	beego.Controller
}

// @router / [get]
func (c *PageController) LoginPage() {
	c.TplName = "modern_login.html"
}

// @router /register [get]
func (c *PageController) RegisterPage() {
	c.TplName = "modern_register.html"
}

// @router /home [get]
func (c *PageController) HomePage() {
	c.TplName = "home.html"
}

// @router /about [get]
func (c *PageController) AboutPage() {
	c.TplName = "about.html"
}

// @router /contact [get]
func (c *PageController) ContactPage() {
	c.TplName = "contact.html"
}

// @router /faq [get]
func (c *PageController) FAQPage() {
	c.TplName = "faq.html"
}

// @router /pricing [get]
func (c *PageController) PricingPage() {
	c.TplName = "pricing.html"
}

// @router /reset-password [get]
func (c *PageController) ResetPasswordPage() {
	c.TplName = "reset_password.html"
}

// @router /privacy [get]
func (c *PageController) PrivacyPage() {
	c.TplName = "privacy.html"
}

// @router /terms [get]
func (c *PageController) TermsPage() {
	c.TplName = "terms.html"
}

// @router /dashboard [get]
func (c *PageController) DashboardPage() {
	c.TplName = "dashboard.html"
}

// @router /profile [get]
func (c *PageController) ProfilePage() {
	c.TplName = "profile_main.html"
}

// @router /profile/ist [get]
func (c *PageController) ProfileISTPage() {
	c.TplName = "profile.html"
}

// @router /profile/holland [get]
func (c *PageController) ProfileHollandPage() {
	c.TplName = "profile_holland.html"
}

// @router /profile/learning-style [get]
func (c *PageController) ProfileLearningStylePage() {
	c.TplName = "profile_learning_style.html"
}

// @router /profile/kraepelin [get]
func (c *PageController) ProfileKraepelinPage() {
	c.TplName = "profile_kraepelin.html"
}

// @router /settings [get]
func (c *PageController) SettingsPage() {
	c.TplName = "settings.html"
}

// @router /admin/psychotest [get]
func (c *PageController) PsychotestAdminPage() {
	// Backward compatible: redirect old URL to the new list page.
	c.Redirect("/admin/psychotest/batches", 302)
}

// @router /admin/psychotest/batches [get]
func (c *PageController) PsychotestAdminBatchesPage() {
	// Reuse existing admin_psychotest.html as the batch listing page.
	c.TplName = "admin_psychotest.html"
}

// @router /admin/psychotest/batches/add [get]
func (c *PageController) PsychotestAdminAddBatchPage() {
	c.TplName = "admin_psychotest_add_batch.html"
}

// @router /profile/holland/start [get]
// Sub-page di dalam profile untuk memulai / melanjutkan Holland.
func (c *PageController) ProfileHollandStartPage() {
	userIDAny := c.GetSession("user_id")
	if userIDAny == nil {
		c.Redirect("/login?next=/profile", 302)
		return
	}
	userID, ok := userIDAny.(int)
	if !ok || userID == 0 {
		c.Redirect("/profile", 302)
		return
	}

	invIDStr := strings.TrimSpace(c.GetString("invId"))
	invID, err := strconv.Atoi(invIDStr)
	if err != nil || invID <= 0 {
		c.Redirect("/profile", 302)
		return
	}

	o := orm.NewOrm()

	var user models.User
	user.Id = userID
	if err := o.Read(&user); err != nil {
		c.Redirect("/profile", 302)
		return
	}

	var inv models.TestInvitation
	inv.Id = invID
	if err := o.Read(&inv); err != nil || inv.Id == 0 {
		c.Redirect("/profile", 302)
		return
	}

	// Ownership check
	allowed := false
	if inv.UserId != nil && *inv.UserId == userID {
		allowed = true
	}
	if !allowed && strings.TrimSpace(inv.Email) != "" && user.Email != "" && strings.EqualFold(inv.Email, user.Email) {
		allowed = true
	}
	if !allowed {
		c.Redirect("/profile", 302)
		return
	}

	if inv.BatchId == nil {
		c.Redirect("/profile", 302)
		return
	}

	// Ensure Holland enabled for the batch
	var batch models.TestBatch
	batch.Id = *inv.BatchId
	if err := o.Read(&batch); err != nil {
		c.Redirect("/profile", 302)
		return
	}
	if !batch.EnableHolland {
		c.Redirect("/profile", 302)
		return
	}

	// Set session so HollandTestController can run
	c.SetSession("current_invitation_id", inv.Id)
	c.SetSession("current_batch_id", *inv.BatchId)
	c.Redirect("/test/holland/start", 302)
}

// @router /profile/ist/start [get]
// Sub-page di dalam profile untuk memulai / melanjutkan IST.
func (c *PageController) ProfileISTStartPage() {
	userIDAny := c.GetSession("user_id")
	if userIDAny == nil {
		c.Redirect("/login?next=/profile", 302)
		return
	}
	userID, ok := userIDAny.(int)
	if !ok || userID == 0 {
		c.Redirect("/profile", 302)
		return
	}

	invIDStr := strings.TrimSpace(c.GetString("invId"))
	invID, err := strconv.Atoi(invIDStr)
	if err != nil || invID <= 0 {
		c.Redirect("/profile", 302)
		return
	}

	o := orm.NewOrm()

	var user models.User
	user.Id = userID
	if err := o.Read(&user); err != nil {
		c.Redirect("/profile", 302)
		return
	}

	var inv models.TestInvitation
	inv.Id = invID
	if err := o.Read(&inv); err != nil || inv.Id == 0 {
		c.Redirect("/profile", 302)
		return
	}

	// Ownership check
	allowed := false
	if inv.UserId != nil && *inv.UserId == userID {
		allowed = true
	}
	if !allowed && strings.TrimSpace(inv.Email) != "" && user.Email != "" && strings.EqualFold(inv.Email, user.Email) {
		allowed = true
	}
	if !allowed {
		c.Redirect("/profile", 302)
		return
	}

	if inv.BatchId == nil {
		c.Redirect("/profile", 302)
		return
	}

	// Ensure IST enabled for the batch
	var batch models.TestBatch
	batch.Id = *inv.BatchId
	if err := o.Read(&batch); err != nil {
		c.Redirect("/profile", 302)
		return
	}
	if !batch.EnableIST {
		c.Redirect("/profile", 302)
		return
	}

	// Set session so ISTTestController can run
	c.SetSession("current_invitation_id", inv.Id)
	c.SetSession("current_batch_id", *inv.BatchId)
	c.Redirect("/test/ist/start", 302)
}

// @router /profile/learning-style/start [get]
// Sub-page di dalam profile untuk memulai / melanjutkan Learning Style (VAK).
func (c *PageController) ProfileLearningStyleStartPage() {
	userIDAny := c.GetSession("user_id")
	if userIDAny == nil {
		c.Redirect("/login?next=/profile", 302)
		return
	}
	userID, ok := userIDAny.(int)
	if !ok || userID == 0 {
		c.Redirect("/profile", 302)
		return
	}

	invIDStr := strings.TrimSpace(c.GetString("invId"))
	invID, err := strconv.Atoi(invIDStr)
	if err != nil || invID <= 0 {
		c.Redirect("/profile", 302)
		return
	}

	o := orm.NewOrm()

	var user models.User
	user.Id = userID
	if err := o.Read(&user); err != nil {
		c.Redirect("/profile", 302)
		return
	}

	var inv models.TestInvitation
	inv.Id = invID
	if err := o.Read(&inv); err != nil || inv.Id == 0 {
		c.Redirect("/profile", 302)
		return
	}

	allowed := false
	if inv.UserId != nil && *inv.UserId == userID {
		allowed = true
	}
	if !allowed && strings.TrimSpace(inv.Email) != "" && user.Email != "" && strings.EqualFold(inv.Email, user.Email) {
		allowed = true
	}
	if !allowed {
		c.Redirect("/profile", 302)
		return
	}

	if inv.BatchId == nil {
		c.Redirect("/profile", 302)
		return
	}

	var batch models.TestBatch
	batch.Id = *inv.BatchId
	if err := o.Read(&batch); err != nil {
		c.Redirect("/profile", 302)
		return
	}
	if !batch.EnableLearningStyle {
		c.Redirect("/profile", 302)
		return
	}

	c.SetSession("current_invitation_id", inv.Id)
	c.SetSession("current_batch_id", *inv.BatchId)
	c.Redirect("/test/learning-style/start", 302)
}

// @router /notifications [get]
func (c *PageController) NotificationsPage() {
	c.TplName = "notifications.html"
}