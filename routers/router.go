package routers

import (
	"net/url"
	"strings"

	"psikologi_apps/controllers"

	beego "github.com/beego/beego/v2/server/web"
	"github.com/beego/beego/v2/server/web/context"
)

func init() {
	// Simple role-based access filter (example: only admin can access /admin/*)
	beego.InsertFilter("/admin/*", beego.BeforeRouter, func(ctx *context.Context) {
		roleVal := ctx.Input.Session("user_role")
		roleStr, _ := roleVal.(string)
		if roleStr != "admin" {
			ctx.Output.SetStatus(403)
			ctx.Output.JSON(map[string]interface{}{
				"success": false,
				"message": "Akses ditolak, hanya admin yang boleh mengakses",
			}, false, false)
		}
	})

	// Auth filter: protect private pages & APIs (e.g. /dashboard) so user must login first
	beego.InsertFilter("/*", beego.BeforeRouter, func(ctx *context.Context) {
		path := ctx.Request.URL.Path

		// Allow static assets without auth
		if strings.HasPrefix(path, "/static/") {
			return
		}

		// Public pages that don't require login
		publicPages := map[string]bool{
			"/":               true,
			"/home":           true,
			"/about":          true,
			"/contact":        true,
			"/faq":            true,
			"/pricing":        true,
			"/login":          true,
			"/register":       true,
			"/reset-password": true,
			"/privacy":        true,
			"/terms":          true,
			// Device verification links from email harus bisa diakses tanpa login
			"/verify-device":  true,
			"/reject-device":  true,
		}

		// Public APIs (auth & contact & captcha & reset password)
		if strings.HasPrefix(path, "/api/auth/") ||
			path == "/api/contact" {
			return
		}

		// If path is explicitly public, skip auth check
		if publicPages[path] {
			return
		}

		// Check session
		userID := ctx.Input.Session("user_id")
		if userID == nil {
			// If it's an API request, return JSON 401
			if strings.HasPrefix(path, "/api/") || ctx.Input.IsAjax() {
				ctx.Output.SetStatus(401)
				ctx.Output.JSON(map[string]interface{}{
					"success": false,
					"message": "Silakan login terlebih dahulu",
				}, false, false)
				return
			}

			// For normal page request, redirect to login with next parameter
			next := url.QueryEscape(path)
			ctx.Redirect(302, "/login?next="+next)
			return
		}
	})

	// Page routes
	beego.Router("/", &controllers.PageController{}, "get:HomePage")
	beego.Router("/home", &controllers.PageController{}, "get:HomePage")
	beego.Router("/about", &controllers.PageController{}, "get:AboutPage")
	beego.Router("/contact", &controllers.PageController{}, "get:ContactPage")
	beego.Router("/faq", &controllers.PageController{}, "get:FAQPage")
	beego.Router("/pricing", &controllers.PageController{}, "get:PricingPage")
	beego.Router("/login", &controllers.PageController{}, "get:LoginPage")
	beego.Router("/register", &controllers.PageController{}, "get:RegisterPage")
	beego.Router("/reset-password", &controllers.PageController{}, "get:ResetPasswordPage")
	beego.Router("/privacy", &controllers.PageController{}, "get:PrivacyPage")
	beego.Router("/terms", &controllers.PageController{}, "get:TermsPage")
	beego.Router("/dashboard", &controllers.PageController{}, "get:DashboardPage")
	beego.Router("/profile", &controllers.PageController{}, "get:ProfilePage")
	beego.Router("/profile/ist", &controllers.PageController{}, "get:ProfileISTPage")
	beego.Router("/profile/holland", &controllers.PageController{}, "get:ProfileHollandPage")
	beego.Router("/profile/learning-style", &controllers.PageController{}, "get:ProfileLearningStylePage")
	beego.Router("/profile/kraepelin", &controllers.PageController{}, "get:ProfileKraepelinPage")
	beego.Router("/profile/ist/start", &controllers.PageController{}, "get:ProfileISTStartPage")
	beego.Router("/profile/holland/start", &controllers.PageController{}, "get:ProfileHollandStartPage")
	beego.Router("/profile/learning-style/start", &controllers.PageController{}, "get:ProfileLearningStyleStartPage")
	beego.Router("/settings", &controllers.PageController{}, "get:SettingsPage")
	// Admin psychotest dashboard (only for admin via filter)
	beego.Router("/admin/psychotest", &controllers.PageController{}, "get:PsychotestAdminPage")
	beego.Router("/admin/psychotest/batches", &controllers.PageController{}, "get:PsychotestAdminBatchesPage")
	beego.Router("/admin/psychotest/batches/add", &controllers.PageController{}, "get:PsychotestAdminAddBatchPage")

	// Psychotest client routes (peserta)
	beego.Router("/test", &controllers.PsychotestClientController{}, "get:TokenPage")
	beego.Router("/test/start", &controllers.PsychotestClientController{}, "post:StartTest")

	// IST test flow (peserta)
	beego.Router("/test/ist/start", &controllers.ISTTestController{}, "get:StartISTPage;post:SubmitStartIST")
	beego.Router("/test/ist/announcement", &controllers.ISTTestController{}, "get:AnnouncementPage")
	beego.Router("/test/ist/instruction/:code", &controllers.ISTTestController{}, "get:InstructionPage")
	beego.Router("/test/ist/subtest/:code", &controllers.ISTTestController{}, "get:SubtestPage")
	beego.Router("/test/ist/finish", &controllers.ISTTestController{}, "get:FinishPage")
	beego.Router("/test/ist/result", &controllers.ISTTestController{}, "get:ResultPage")
	beego.Router("/test/ist/result/pdf", &controllers.ISTTestController{}, "get:ExportResultPDF")
	beego.Router("/test/ist/result/excel", &controllers.ISTTestController{}, "get:ExportResultExcel")
	beego.Router("/api/test/ist/subtest/:code", &controllers.ISTTestController{}, "post:SubmitSubtestAPI")
	beego.Router("/api/test/ist/violation", &controllers.ISTTestController{}, "post:ReportViolationAPI")

	// Holland test flow (peserta)
	beego.Router("/test/holland/start", &controllers.HollandTestController{}, "get:StartHollandPage")
	beego.Router("/test/holland/instruction", &controllers.HollandTestController{}, "get:HollandInstructionPage")
	beego.Router("/test/holland/page1", &controllers.HollandTestController{}, "get:HollandPage1")
	beego.Router("/test/holland/page2", &controllers.HollandTestController{}, "get:HollandPage2")
	beego.Router("/test/holland/page3", &controllers.HollandTestController{}, "get:HollandPage3")
	beego.Router("/test/holland/finish", &controllers.HollandTestController{}, "get:HollandFinishPage")
	beego.Router("/test/holland/result/excel", &controllers.HollandTestController{}, "get:ExportResultExcel")
	beego.Router("/api/test/holland/page1", &controllers.HollandTestController{}, "post:SubmitPage1API")
	beego.Router("/api/test/holland/page2", &controllers.HollandTestController{}, "post:SubmitPage2API")
	beego.Router("/api/test/holland/page3", &controllers.HollandTestController{}, "post:SubmitPage3API")

	// Learning Style (VAK) test flow (peserta)
	beego.Router("/test/learning-style/start", &controllers.LearningStyleTestController{}, "get:StartPage;post:SubmitStart")
	beego.Router("/test/learning-style/instruction", &controllers.LearningStyleTestController{}, "get:InstructionPage")
	beego.Router("/test/learning-style/questions", &controllers.LearningStyleTestController{}, "get:QuestionsPage")
	beego.Router("/test/learning-style/finish", &controllers.LearningStyleTestController{}, "get:FinishPage")
	beego.Router("/test/learning-style/result/excel", &controllers.LearningStyleTestController{}, "get:ExportResultExcel")
	beego.Router("/api/test/learning-style/submit", &controllers.LearningStyleTestController{}, "post:SubmitAnswersAPI")

	// Kraepelin test flow (peserta)
	beego.Router("/test/kraepelin/start", &controllers.KraepelinTestController{}, "get:StartPage;post:SubmitStart")
	beego.Router("/test/kraepelin/instruction", &controllers.KraepelinTestController{}, "get:InstructionPage")
	beego.Router("/test/kraepelin/questions", &controllers.KraepelinTestController{}, "get:QuestionsPage")
	beego.Router("/test/kraepelin/finish", &controllers.KraepelinTestController{}, "get:FinishPage")
	beego.Router("/test/kraepelin/result/excel", &controllers.KraepelinTestController{}, "get:ExportResultExcel")
	beego.Router("/api/test/kraepelin/submit", &controllers.KraepelinTestController{}, "post:SubmitAnswersAPI")
	// API routes
	beego.Router("/api/auth/register", &controllers.AuthController{}, "post:Register")
	beego.Router("/api/auth/login", &controllers.AuthController{}, "post:Login")
	beego.Router("/api/auth/logout", &controllers.AuthController{}, "post:Logout")
	beego.Router("/api/auth/change-password", &controllers.AuthController{}, "post:ChangePassword")
	beego.Router("/api/auth/captcha", &controllers.AuthController{}, "get:GetCaptcha")
	beego.Router("/api/auth/captcha/:id", &controllers.AuthController{}, "get:CaptchaImage")
	beego.Router("/api/auth/google/login", &controllers.AuthController{}, "get:GoogleLogin")
	beego.Router("/api/auth/google/callback", &controllers.AuthController{}, "get:GoogleCallback")
	beego.Router("/api/auth/request-reset", &controllers.PasswordResetController{}, "post:RequestOTP")
	beego.Router("/api/auth/verify-reset", &controllers.PasswordResetController{}, "post:VerifyOTP")
	
	// Contact routes
	beego.Router("/api/contact", &controllers.ContactController{}, "post:SendMessage")
	
	// Profile routes
	beego.Router("/api/profile", &controllers.ProfileController{}, "get:GetProfile;put:UpdateProfile")
	beego.Router("/api/profile/upload", &controllers.ProfileController{}, "post:UploadFoto")
	beego.Router("/api/profile/tests", &controllers.ProfileController{}, "get:GetTestResults")
	beego.Router("/api/profile/test-summary", &controllers.ProfileController{}, "get:GetTestSummary")
	
	// Settings routes
	beego.Router("/api/settings", &controllers.SettingsController{}, "get:GetSettings;put:UpdateSettings")

	// Psychotest admin APIs (manage batches, invitations & export)
	beego.Router("/api/admin/test-batches", &controllers.PsychotestAdminController{}, "get:ListBatches;post:CreateBatch")
	beego.Router("/api/admin/test-batches/:id", &controllers.PsychotestAdminController{}, "put:UpdateBatch;delete:DeleteBatch")
	beego.Router("/api/admin/test-batches/bulk", &controllers.PsychotestAdminController{}, "post:BulkBatches")
	beego.Router("/api/admin/test-batches/:id/invitations", &controllers.PsychotestAdminController{}, "get:ListInvitations;post:CreateInvitations")
	beego.Router("/api/admin/test-batches/:id/results", &controllers.PsychotestAdminController{}, "get:ListBatchResults")
	beego.Router("/api/admin/test-batches/:id/export-answers", &controllers.PsychotestAdminController{}, "get:ExportBatchAnswers")
	// Export jawaban untuk satu anak (berdasarkan invitation)
	beego.Router("/api/admin/test-batches/:batchId/invitations/:invId/export", &controllers.PsychotestAdminController{}, "get:ExportInvitationAnswers")
	// Invitation CRUD & bulk actions
	beego.Router("/api/admin/test-invitations/:id", &controllers.PsychotestAdminController{}, "put:UpdateInvitation;delete:DeleteInvitation")
	beego.Router("/api/admin/test-invitations/bulk", &controllers.PsychotestAdminController{}, "post:BulkInvitations")
	// Admin user search (suggestion email)
	beego.Router("/api/admin/users/search", &controllers.AdminUserController{}, "get:Search")
	
	// Notification routes
	beego.Router("/api/notifications", &controllers.NotificationController{}, "get:GetNotifications")
	beego.Router("/api/notifications/:id/read", &controllers.NotificationController{}, "put:MarkAsRead")
	beego.Router("/api/notifications/read-all", &controllers.NotificationController{}, "put:MarkAllAsRead")
	
	// Notifications page
	beego.Router("/notifications", &controllers.PageController{}, "get:NotificationsPage")
	
	// Device verification routes
	beego.Router("/verify-device", &controllers.DeviceVerificationController{}, "get:VerifyDevice")
	beego.Router("/reject-device", &controllers.DeviceVerificationController{}, "get:RejectDevice")
}
