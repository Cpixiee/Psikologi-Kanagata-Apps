package models

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/beego/beego/v2/client/orm"
)

type PasswordReset struct {
	Id        int       `orm:"auto;pk" json:"id"`
	Email     string    `orm:"size(255);index" json:"email"`
	OtpCode   string    `orm:"size(10);unique" json:"otp_code"`
	Token     string    `orm:"size(255);unique" json:"token"`
	ExpiresAt time.Time `orm:"type(datetime)" json:"expires_at"`
	Used      bool      `orm:"default(false)" json:"used"`
	CreatedAt time.Time `orm:"auto_now_add;type(datetime)" json:"created_at"`
}

func (p *PasswordReset) TableName() string {
	return "password_resets"
}

// GenerateOTP generates a 6-digit OTP code
func (p *PasswordReset) GenerateOTP() error {
	// Generate 6 random digits
	otp := ""
	for i := 0; i < 6; i++ {
		bytes := make([]byte, 1)
		if _, err := rand.Read(bytes); err != nil {
			return err
		}
		otp += string(rune('0' + (bytes[0] % 10)))
	}
	p.OtpCode = otp
	return nil
}

// GenerateToken generates a secure token
func (p *PasswordReset) GenerateToken() error {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return err
	}
	p.Token = hex.EncodeToString(bytes)
	return nil
}

// IsExpired checks if the OTP has expired
func (p *PasswordReset) IsExpired() bool {
	return time.Now().After(p.ExpiresAt)
}

// IsValid checks if the OTP is valid and not used
func (p *PasswordReset) IsValid() bool {
	return !p.Used && !p.IsExpired()
}

func init() {
	orm.RegisterModel(new(PasswordReset))
}
