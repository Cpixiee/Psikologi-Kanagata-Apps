package controllers

import beego "github.com/beego/beego/v2/server/web"

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
	c.TplName = "profile.html"
}

// @router /settings [get]
func (c *PageController) SettingsPage() {
	c.TplName = "settings.html"
}

// @router /admin/psychotest [get]
func (c *PageController) PsychotestAdminPage() {
	c.TplName = "admin_psychotest.html"
}

// @router /notifications [get]
func (c *PageController) NotificationsPage() {
	c.TplName = "notifications.html"
}