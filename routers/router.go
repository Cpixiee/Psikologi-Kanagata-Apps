package routers

import (
	"net/url"
	"strings"

	"psikologi_apps/controllers"

	beego "github.com/beego/beego/v2/server/web"
	"github.com/beego/beego/v2/server/web/context"
)

func init() {
	// Admin/Sekolah access filter:
	//   - role 'admin'  : full access ke semua route /admin/*
	//   - role 'sekolah': read-only ke /admin/*; method tulis (POST/PUT/DELETE)
	//                     dan route create batch/invitation diblokir.
	beego.InsertFilter("/admin/*", beego.BeforeRouter, func(ctx *context.Context) {
		roleVal := ctx.Input.Session("user_role")
		roleStr, _ := roleVal.(string)
		if roleStr == "admin" {
			return
		}
		if roleStr == "sekolah" {
			method := strings.ToUpper(ctx.Request.Method)
			path := ctx.Request.URL.Path
			// Sekolah TIDAK boleh:
			//   - membuat/ubah/hapus batch
			//   - membuat/ubah/hapus undangan (termasuk send-code & bulk)
			//   - mengelola data sekolah/guru (CRUD)
			writeBlocked := method != "GET" && method != "HEAD"
			adminOnlyPath := strings.HasPrefix(path, "/admin/schools") ||
				strings.HasPrefix(path, "/api/admin/schools")
			if adminOnlyPath {
				ctx.Output.SetStatus(403)
				ctx.Output.JSON(map[string]interface{}{
					"success": false,
					"message": "Akses ditolak, hanya admin yang boleh mengakses",
				}, false, false)
				return
			}
			if writeBlocked {
				ctx.Output.SetStatus(403)
				ctx.Output.JSON(map[string]interface{}{
					"success": false,
					"message": "Akun sekolah tidak diizinkan mengubah data (read-only).",
				}, false, false)
				return
			}
			return
		}
		ctx.Output.SetStatus(403)
		ctx.Output.JSON(map[string]interface{}{
			"success": false,
			"message": "Akses ditolak, hanya admin yang boleh mengakses",
		}, false, false)
	})

	// Auth filter: protect private pages & APIs (e.g. /dashboard) so user must login first
	beego.InsertFilter("/*", beego.BeforeRouter, func(ctx *context.Context) {
		path := ctx.Request.URL.Path

		// Allow static assets without auth
		if strings.HasPrefix(path, "/static/") || strings.HasPrefix(path, "/.well-known/") {
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

	beego.SetStaticPath("/.well-known", "static/.well-known")

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
	beego.Router("/dashboard/students", &controllers.PageController{}, "get:SchoolStudentsPage")
	beego.Router("/dashboard/batch/:id", &controllers.PageController{}, "get:DashboardBatchDetailPage")
	beego.Router("/dashboard/batch/result/:id", &controllers.PageController{}, "get:StudentBatchResultPage")
	beego.Router("/profile", &controllers.PageController{}, "get:ProfilePage")
	beego.Router("/profile/edit", &controllers.PageController{}, "get:ProfileEditPage")
	beego.Router("/hasil-tes", &controllers.PageController{}, "get:HasilTesPage")
	beego.Router("/profile/ist", &controllers.PageController{}, "get:ProfileISTPage")
	beego.Router("/profile/holland", &controllers.PageController{}, "get:ProfileHollandPage")
	beego.Router("/profile/learning-style", &controllers.PageController{}, "get:ProfileLearningStylePage")
	beego.Router("/profile/kraepelin", &controllers.PageController{}, "get:ProfileKraepelinPage")
	beego.Router("/profile/ist/start", &controllers.PageController{}, "get:ProfileISTStartPage")
	beego.Router("/profile/holland/start", &controllers.PageController{}, "get:ProfileHollandStartPage")
	beego.Router("/profile/learning-style/start", &controllers.PageController{}, "get:ProfileLearningStyleStartPage")
	beego.Router("/profile/rmib", &controllers.PageController{}, "get:ProfileRMIBPage")
	beego.Router("/profile/rmib/start", &controllers.PageController{}, "get:ProfileRMIBStartPage")
	beego.Router("/profile/papi", &controllers.PageController{}, "get:ProfilePAPIPage")
	beego.Router("/settings", &controllers.PageController{}, "get:SettingsPage")
	beego.Router("/settings/notifications", &controllers.PageController{}, "get:NotificationSettingsPage")
	beego.Router("/ai", &controllers.PageController{}, "get:AIPage")
	// Admin psychotest dashboard (only for admin via filter)
	beego.Router("/admin/psychotest", &controllers.PageController{}, "get:PsychotestAdminPage")
	beego.Router("/admin/psychotest/batches", &controllers.PageController{}, "get:PsychotestAdminBatchesPage")
	beego.Router("/admin/psychotest/batches/add", &controllers.PageController{}, "get:PsychotestAdminAddBatchPage")
	// Admin: kelola akun sekolah
	beego.Router("/admin/schools", &controllers.PageController{}, "get:AdminSchoolsPage")
	beego.Router("/admin/schools/add", &controllers.PageController{}, "get:AdminSchoolsAddPage")

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
	beego.Router("/test/ist/dev-autofill", &controllers.ISTTestController{}, "post:DevAutoFill")

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

	// RMIB test flow (peserta) - versi pria/wanita ditentukan otomatis dari gender user.
	beego.Router("/test/rmib/start", &controllers.RMIBTestController{}, "get:StartPage")
	beego.Router("/test/rmib/instruction", &controllers.RMIBTestController{}, "get:InstructionPage")
	beego.Router("/test/rmib/group/:n", &controllers.RMIBTestController{}, "get:GroupPage")
	beego.Router("/test/rmib/summary", &controllers.RMIBTestController{}, "get:SummaryPage")
	beego.Router("/test/rmib/submit", &controllers.RMIBTestController{}, "post:SubmitFinal")
	beego.Router("/test/rmib/finish", &controllers.RMIBTestController{}, "get:FinishPage")
	beego.Router("/test/rmib/result", &controllers.RMIBTestController{}, "get:ResultPage")
	beego.Router("/test/rmib/result/excel", &controllers.RMIBTestController{}, "get:ExportResultExcel")
	beego.Router("/api/test/rmib/answer", &controllers.RMIBTestController{}, "post:SaveAnswerAPI")
	beego.Router("/api/test/rmib/group/:n", &controllers.RMIBTestController{}, "post:SubmitGroupAPI")
	beego.Router("/test/rmib/dev-autofill", &controllers.RMIBTestController{}, "post:DevAutoFill")

	// PAPI test flow (peserta)
	beego.Router("/test/papi/start", &controllers.PAPITestController{}, "get:StartPage")
	beego.Router("/test/papi/instruction", &controllers.PAPITestController{}, "get:InstructionPage")
	beego.Router("/test/papi/questions", &controllers.PAPITestController{}, "get:QuestionsPage")
	beego.Router("/test/papi/submit", &controllers.PAPITestController{}, "post:SubmitFinal")
	beego.Router("/test/papi/finish", &controllers.PAPITestController{}, "get:FinishPage")
	beego.Router("/test/papi/result", &controllers.PAPITestController{}, "get:ResultPage")
	beego.Router("/test/papi/result/excel", &controllers.PAPITestController{}, "get:ExportResultExcel")
	beego.Router("/api/test/papi/answer", &controllers.PAPITestController{}, "post:SaveAnswerAPI")
	beego.Router("/test/papi/dev-autofill", &controllers.PAPITestController{}, "post:DevAutoFill")

	// API routes
	beego.Router("/api/auth/register", &controllers.AuthController{}, "post:Register")
	beego.Router("/api/auth/login", &controllers.AuthController{}, "post:Login")
	beego.Router("/api/auth/logout", &controllers.AuthController{}, "post:Logout")
	beego.Router("/api/auth/exit-impersonate", &controllers.AuthController{}, "post:ExitImpersonate")
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
	beego.Router("/api/profile/onboarding-status", &controllers.ProfileController{}, "get:OnboardingStatus")
	beego.Router("/api/profile/onboarding", &controllers.ProfileController{}, "post:SaveOnboarding")

	// Proxy data wilayah Indonesia (mengatasi mixed-content & CORS pada sumber publik)
	beego.Router("/api/wilayah/provinces", &controllers.WilayahController{}, "get:Provinces")
	beego.Router("/api/wilayah/regencies/:id", &controllers.WilayahController{}, "get:Regencies")
	beego.Router("/api/wilayah/districts/:id", &controllers.WilayahController{}, "get:Districts")
	beego.Router("/api/profile/upload", &controllers.ProfileController{}, "post:UploadFoto")
	beego.Router("/api/profile/tests", &controllers.ProfileController{}, "get:GetTestResults")
	beego.Router("/api/profile/test-summary", &controllers.ProfileController{}, "get:GetTestSummary")
	beego.Router("/api/profile/rmib", &controllers.ProfileController{}, "get:GetRMIBResults")
	beego.Router("/api/profile/papi-results", &controllers.ProfileController{}, "get:GetPAPIResults")
	
	// Settings routes
	beego.Router("/api/settings", &controllers.SettingsController{}, "get:GetSettings;put:UpdateSettings")

	// AI (Gemini) routes
	beego.Router("/api/ai/test-summary", &controllers.AIController{}, "post:TestSummary")
	beego.Router("/api/ai/batch-summary", &controllers.AIController{}, "post:BatchSummary")
	beego.Router("/api/ai/student-combined-summary", &controllers.AIController{}, "post:StudentCombinedSummary")
	beego.Router("/api/ai/chat", &controllers.AIController{}, "post:Chat")

	// Psychotest admin APIs (manage batches, invitations & export)
	beego.Router("/api/admin/test-batches", &controllers.PsychotestAdminController{}, "get:ListBatches;post:CreateBatch")
	beego.Router("/api/admin/test-batches/:id", &controllers.PsychotestAdminController{}, "get:GetBatchDetail;put:UpdateBatch;delete:DeleteBatch")
	beego.Router("/api/admin/test-batches/bulk", &controllers.PsychotestAdminController{}, "post:BulkBatches")
	beego.Router("/api/admin/test-batches/:id/invitations", &controllers.PsychotestAdminController{}, "get:ListInvitations;post:CreateInvitations")
	beego.Router("/api/admin/test-batches/:id/results", &controllers.PsychotestAdminController{}, "get:ListBatchResults")
	beego.Router("/api/admin/test-batches/:id/export-answers", &controllers.PsychotestAdminController{}, "get:ExportBatchAnswers")
	beego.Router("/api/admin/test-batches/:id/ranking/:test", &controllers.RankingController{}, "get:ExportRanking")
	// Export jawaban untuk satu anak (berdasarkan invitation)
	beego.Router("/api/admin/test-batches/:batchId/invitations/:invId/export", &controllers.PsychotestAdminController{}, "get:ExportInvitationAnswers")
	beego.Router("/api/results/export-zip", &controllers.PsychotestAdminController{}, "get:ExportSingleResultZIP")
	// Invitation CRUD & bulk actions
	beego.Router("/api/admin/test-invitations/:id", &controllers.PsychotestAdminController{}, "put:UpdateInvitation;delete:DeleteInvitation")
	beego.Router("/api/admin/test-invitations/:id/result", &controllers.PsychotestAdminController{}, "get:GetInvitationResult")
	beego.Router("/api/admin/test-invitations/:id/send-code", &controllers.PsychotestAdminController{}, "post:SendCode")
	beego.Router("/api/admin/test-invitations/bulk", &controllers.PsychotestAdminController{}, "post:BulkInvitations")
	// Admin user search (suggestion email)
	beego.Router("/api/admin/users/search", &controllers.AdminUserController{}, "get:Search")
	// Admin: CRUD akun sekolah & daftar guru
	beego.Router("/api/admin/schools", &controllers.AdminSchoolController{}, "get:List;post:Create")
	beego.Router("/api/admin/schools/:id", &controllers.AdminSchoolController{}, "get:Detail;delete:Delete")
	// Endpoint khusus akun sekolah: melihat daftar guru mereka sendiri
	beego.Router("/api/schools/my-teachers", &controllers.AdminSchoolController{}, "get:MyTeachers")
	beego.Router("/api/schools/students", &controllers.AdminSchoolController{}, "get:ListStudents")
	beego.Router("/api/schools/access-student/:id", &controllers.AdminSchoolController{}, "post:AccessStudent")
	
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
