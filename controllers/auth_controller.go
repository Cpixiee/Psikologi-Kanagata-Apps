package controllers

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"psikologi_apps/models"
	"psikologi_apps/utils"
	"strings"

	"github.com/beego/beego/v2/client/orm"
	beego "github.com/beego/beego/v2/server/web"
	"github.com/beego/beego/v2/core/logs"
	"github.com/dchest/captcha"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type AuthController struct {
	beego.Controller
}

type RegisterRequest struct {
	NamaLengkap  string `json:"nama_lengkap"`
	Alamat       string `json:"alamat"`
	JenisKelamin string `json:"jenis_kelamin"`
	Email        string `json:"email"`
	NoHandphone  string `json:"no_handphone"`
	Password     string `json:"password"`
	CaptchaId    string `json:"captcha_id"`
	CaptchaValue string `json:"captcha_value"`
	Role         string `json:"role"` // siswa, guru, pekerja; admin biasanya di-set manual
}

type LoginRequest struct {
	Email        string `json:"email"`
	Password     string `json:"password"`
	CaptchaId    string `json:"captcha_id"`
	CaptchaValue string `json:"captcha_value"`
}

type GoogleUserInfo struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

type Response struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ChangePasswordRequest represents payload for password update.
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// @router /api/auth/register [post]
func (c *AuthController) Register() {
	var req RegisterRequest
	json.Unmarshal(c.Ctx.Input.RequestBody, &req)

	// Validate CAPTCHA
	if !captcha.VerifyString(req.CaptchaId, req.CaptchaValue) {
		c.Data["json"] = Response{
			Success: false,
			Message: "CAPTCHA tidak valid",
		}
		c.ServeJSON()
		return
	}

	// Validate required fields
	if req.NamaLengkap == "" || req.Email == "" || req.Password == "" {
		c.Data["json"] = Response{
			Success: false,
			Message: "Semua field wajib diisi",
		}
		c.ServeJSON()
		return
	}

	// Validate gender
	if req.JenisKelamin != "laki_laki" && req.JenisKelamin != "perempuan" {
		c.Data["json"] = Response{
			Success: false,
			Message: "Jenis kelamin harus laki_laki atau perempuan",
		}
		c.ServeJSON()
		return
	}

	// Check if email already exists
	o := orm.NewOrm()
	existingUser := models.User{Email: req.Email}
	err := o.Read(&existingUser, "Email")
	if err == nil {
		c.Data["json"] = Response{
			Success: false,
			Message: "Email sudah terdaftar",
		}
		c.ServeJSON()
		return
	}

	// Determine role (default: siswa)
	role := req.Role
	if role == "" {
		role = string(models.RoleSiswa)
	}
	if role != string(models.RoleSiswa) &&
		role != string(models.RoleGuru) &&
		role != string(models.RolePekerja) &&
		role != string(models.RoleMahasiswa) &&
		role != string(models.RoleUmum) &&
		role != string(models.RoleAdmin) {
		c.Data["json"] = Response{
			Success: false,
			Message: "Role tidak valid. Gunakan: siswa, guru, pekerja, mahasiswa, umum, atau admin",
		}
		c.ServeJSON()
		return
	}

	// Create new user
	user := models.User{
		NamaLengkap:  req.NamaLengkap,
		Alamat:       req.Alamat,
		JenisKelamin: models.Gender(req.JenisKelamin),
		Email:        req.Email,
		NoHandphone:  req.NoHandphone,
		Password:     req.Password,
		Role:         models.Role(role),
	}

	// Hash password
	if err := user.HashPassword(); err != nil {
		c.Data["json"] = Response{
			Success: false,
			Message: "Gagal memproses password",
		}
		c.ServeJSON()
		return
	}

	// Insert user
	_, err = o.Insert(&user)
	if err != nil {
		c.Data["json"] = Response{
			Success: false,
			Message: "Gagal mendaftarkan user: " + err.Error(),
		}
		c.ServeJSON()
		return
	}

	c.Data["json"] = Response{
		Success: true,
		Message: "Registrasi berhasil",
		Data: map[string]interface{}{
			"id":    user.Id,
			"email": user.Email,
		},
	}
	c.ServeJSON()
}

// @router /api/auth/login [post]
func (c *AuthController) Login() {
	var req LoginRequest
	json.Unmarshal(c.Ctx.Input.RequestBody, &req)

	// Validate CAPTCHA
	if !captcha.VerifyString(req.CaptchaId, req.CaptchaValue) {
		c.Data["json"] = Response{
			Success: false,
			Message: "CAPTCHA tidak valid",
		}
		c.ServeJSON()
		return
	}

	// Validate required fields
	if req.Email == "" || req.Password == "" {
		c.Data["json"] = Response{
			Success: false,
			Message: "Email dan password wajib diisi",
		}
		c.ServeJSON()
		return
	}

	// Find user by email
	o := orm.NewOrm()
	user := models.User{Email: req.Email}
	err := o.Read(&user, "Email")
	if err != nil {
		c.Data["json"] = Response{
			Success: false,
			Message: "Email atau password salah",
		}
		c.ServeJSON()
		return
	}

	// Verify password
	if !user.CheckPassword(req.Password) {
		c.Data["json"] = Response{
			Success: false,
			Message: "Email atau password salah",
		}
		c.ServeJSON()
		return
	}

	isNewDevice, blocked := c.establishLoginSession(&user)
	if blocked {
		c.Data["json"] = Response{
			Success: false,
			Message: "Perangkat ini telah diblokir. Silakan hubungi administrator.",
		}
		c.ServeJSON()
		return
	}

	c.Data["json"] = Response{
		Success: true,
		Message: "Login berhasil",
		Data: map[string]interface{}{
			"id":           user.Id,
			"nama_lengkap": user.NamaLengkap,
			"email":        user.Email,
			"role":         user.Role,
			"is_new_device": isNewDevice,
		},
	}
	c.ServeJSON()
}

// @router /api/auth/google/login [get]
func (c *AuthController) GoogleLogin() {
	oauthConfig := getGoogleOAuthConfig()
	if oauthConfig.ClientID == "" || oauthConfig.ClientSecret == "" || oauthConfig.RedirectURL == "" {
		c.Redirect("/login?google_error=config", 302)
		return
	}

	state, err := randomBase64URL(24)
	if err != nil {
		c.Redirect("/login?google_error=state", 302)
		return
	}

	next := strings.TrimSpace(c.GetString("next"))
	if next == "" {
		next = "/dashboard"
	}
	if !strings.HasPrefix(next, "/") {
		next = "/dashboard"
	}

	c.SetSession("google_oauth_state", state)
	c.SetSession("google_oauth_next", next)

	authURL := oauthConfig.AuthCodeURL(state, oauth2.AccessTypeOnline, oauth2.SetAuthURLParam("prompt", "select_account"))
	c.Redirect(authURL, 302)
}

// @router /api/auth/google/callback [get]
func (c *AuthController) GoogleCallback() {
	oauthConfig := getGoogleOAuthConfig()
	if oauthConfig.ClientID == "" || oauthConfig.ClientSecret == "" || oauthConfig.RedirectURL == "" {
		c.Redirect("/login?google_error=config", 302)
		return
	}

	state := c.GetString("state")
	code := c.GetString("code")
	sessionState, _ := c.GetSession("google_oauth_state").(string)
	next, _ := c.GetSession("google_oauth_next").(string)
	if next == "" || !strings.HasPrefix(next, "/") {
		next = "/dashboard"
	}

	c.DelSession("google_oauth_state")
	c.DelSession("google_oauth_next")

	if state == "" || sessionState == "" || state != sessionState || code == "" {
		c.Redirect("/login?google_error=state", 302)
		return
	}

	token, err := oauthConfig.Exchange(context.Background(), code)
	if err != nil {
		logs.Error("Google token exchange failed: %v", err)
		c.Redirect("/login?google_error=token", 302)
		return
	}

	client := oauthConfig.Client(context.Background(), token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		logs.Error("Google userinfo request failed: %v", err)
		c.Redirect("/login?google_error=userinfo", 302)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logs.Error("Google userinfo bad status: %d", resp.StatusCode)
		c.Redirect("/login?google_error=userinfo", 302)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.Redirect("/login?google_error=userinfo", 302)
		return
	}

	var gUser GoogleUserInfo
	if err := json.Unmarshal(body, &gUser); err != nil || strings.TrimSpace(gUser.Email) == "" {
		c.Redirect("/login?google_error=userdata", 302)
		return
	}

	o := orm.NewOrm()
	user := models.User{Email: strings.TrimSpace(gUser.Email)}
	err = o.Read(&user, "Email")
	if err == orm.ErrNoRows {
		displayName := strings.TrimSpace(gUser.Name)
		if displayName == "" {
			displayName = strings.Split(strings.TrimSpace(gUser.Email), "@")[0]
		}

		randomPass, randErr := randomBase64URL(18)
		if randErr != nil {
			c.Redirect("/login?google_error=internal", 302)
			return
		}

		newUser := models.User{
			NamaLengkap:  displayName,
			Alamat:       "-",
			JenisKelamin: models.GenderLakiLaki,
			Email:        strings.TrimSpace(gUser.Email),
			NoHandphone:  "-",
			Password:     randomPass,
			Role:         models.RoleUmum,
		}
		if hashErr := newUser.HashPassword(); hashErr != nil {
			c.Redirect("/login?google_error=internal", 302)
			return
		}
		if _, insertErr := o.Insert(&newUser); insertErr != nil {
			logs.Error("Google user insert failed: %v", insertErr)
			c.Redirect("/login?google_error=internal", 302)
			return
		}
		user = newUser
	} else if err != nil {
		c.Redirect("/login?google_error=internal", 302)
		return
	}

	_, blocked := c.establishLoginSession(&user)
	if blocked {
		c.DestroySession()
		c.Redirect("/login?google_error=device", 302)
		return
	}

	c.Redirect(next, 302)
}

// @router /api/auth/change-password [post]
func (c *AuthController) ChangePassword() {
	userID := c.GetSession("user_id")
	if userID == nil {
		c.Ctx.Output.SetStatus(401)
		c.Data["json"] = Response{
			Success: false,
			Message: "Silakan login terlebih dahulu",
		}
		c.ServeJSON()
		return
	}

	var req ChangePasswordRequest
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &req); err != nil {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = Response{
			Success: false,
			Message: "Format data tidak valid",
		}
		c.ServeJSON()
		return
	}

	if req.CurrentPassword == "" || req.NewPassword == "" {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = Response{
			Success: false,
			Message: "Password saat ini dan password baru wajib diisi",
		}
		c.ServeJSON()
		return
	}

	o := orm.NewOrm()
	user := models.User{Id: userID.(int)}
	if err := o.Read(&user); err != nil {
		c.Ctx.Output.SetStatus(404)
		c.Data["json"] = Response{
			Success: false,
			Message: "User tidak ditemukan",
		}
		c.ServeJSON()
		return
	}

	// Verify current password
	if !user.CheckPassword(req.CurrentPassword) {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = Response{
			Success: false,
			Message: "Password saat ini tidak sesuai",
		}
		c.ServeJSON()
		return
	}

	user.Password = req.NewPassword
	if err := user.HashPassword(); err != nil {
		c.Ctx.Output.SetStatus(500)
		c.Data["json"] = Response{
			Success: false,
			Message: "Gagal memproses password baru",
		}
		c.ServeJSON()
		return
	}

	if _, err := o.Update(&user, "Password"); err != nil {
		c.Ctx.Output.SetStatus(500)
		c.Data["json"] = Response{
			Success: false,
			Message: "Gagal menyimpan password baru",
		}
		c.ServeJSON()
		return
	}

	c.Data["json"] = Response{
		Success: true,
		Message: "Password berhasil diubah",
	}
	c.ServeJSON()
}

// @router /api/auth/logout [post]
func (c *AuthController) Logout() {
	// Destroy all session data for current user
	c.DestroySession()

	c.Data["json"] = Response{
		Success: true,
		Message: "Berhasil logout",
	}
	c.ServeJSON()
}

// @router /api/auth/captcha [get]
func (c *AuthController) GetCaptcha() {
	captchaId := captcha.New()
	c.Data["json"] = Response{
		Success: true,
		Data: map[string]string{
			"captcha_id": captchaId,
		},
	}
	c.ServeJSON()
}

// @router /api/auth/captcha/:id [get]
func (c *AuthController) CaptchaImage() {
	captchaId := c.Ctx.Input.Param(":id")
	c.Ctx.Output.Header("Content-Type", "image/png")
	captcha.WriteImage(c.Ctx.ResponseWriter, captchaId, 200, 60)
}

func getGoogleOAuthConfig() *oauth2.Config {
	clientID, _ := beego.AppConfig.String("GOOGLE_CLIENT_ID")
	clientSecret, _ := beego.AppConfig.String("GOOGLE_CLIENT_SECRET")
	redirectURL, _ := beego.AppConfig.String("GOOGLE_REDIRECT_URL")
	if strings.TrimSpace(redirectURL) == "" {
		redirectURL = "http://localhost:112/api/auth/google/callback"
	}

	return &oauth2.Config{
		ClientID:     strings.TrimSpace(clientID),
		ClientSecret: strings.TrimSpace(clientSecret),
		RedirectURL:  strings.TrimSpace(redirectURL),
		Scopes:       []string{"openid", "email", "profile"},
		Endpoint:     google.Endpoint,
	}
}

func randomBase64URL(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (c *AuthController) establishLoginSession(user *models.User) (bool, bool) {
	userAgent := c.Ctx.Input.UserAgent()
	ipAddress := c.Ctx.Input.IP()

	isNewDevice := false
	var device *models.UserDevice
	deviceCheck, devicePtr, err := utils.CheckAndRegisterDevice(user.Id, userAgent, ipAddress)
	if err != nil {
		logs.Error("Error checking device: %v", err)
	} else {
		isNewDevice = deviceCheck
		device = devicePtr
	}

	if device != nil && device.IsBlocked {
		return false, true
	}

	c.SetSession("user_id", user.Id)
	c.SetSession("user_email", user.Email)
	c.SetSession("user_role", string(user.Role))
	if device != nil {
		c.SetSession("device_id", device.DeviceId)
	}

	go func(uid int, newDev bool, dev *models.UserDevice, ua string) {
		utils.SendNewForYouNotification(uid, "Login Berhasil", "Anda berhasil masuk ke akun Anda.")

		if newDev && dev != nil {
			deviceInfo := dev.DeviceName
			if dev.IpAddress != "" {
				deviceInfo += " (" + dev.IpAddress + ")"
			}
			utils.SendDeviceLinkNotification(uid, deviceInfo)
		} else if dev != nil {
			browserInfo := utils.ParseUserAgent(ua)
			utils.SendBrowserLoginNotification(uid, browserInfo)
		}
	}(user.Id, isNewDevice, device, userAgent)

	return isNewDevice, false
}
