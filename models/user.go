package models

import (
	"time"

	"github.com/beego/beego/v2/client/orm"
	"golang.org/x/crypto/bcrypt"
)

type Gender string

type Role string

const (
	GenderLakiLaki  Gender = "laki_laki"
	GenderPerempuan Gender = "perempuan"

	RoleSiswa     Role = "siswa"
	RoleGuru      Role = "guru"
	RolePekerja   Role = "pekerja"
	RoleMahasiswa Role = "mahasiswa"
	RoleUmum      Role = "umum"
	RoleAdmin     Role = "admin"
)

type User struct {
	Id           int       `orm:"auto;pk" json:"id"`
	NamaLengkap  string    `orm:"size(255)" json:"nama_lengkap"`
	TanggalLahir *time.Time `orm:"null;column(tanggal_lahir);type(date)" json:"tanggal_lahir,omitempty"`
	Alamat       string    `orm:"type(text)" json:"alamat"`
	Kota         string    `orm:"size(100);null" json:"kota"`
	Provinsi     string    `orm:"size(100);null" json:"provinsi"`
	Kodepos      string    `orm:"size(10);null" json:"kodepos"`
	JenisKelamin Gender    `orm:"size(20)" json:"jenis_kelamin"`
	Email        string    `orm:"size(255);unique" json:"email"`
	NoHandphone  string    `orm:"size(20)" json:"no_handphone"`
	AsalInstansi string    `orm:"column(asal_instansi);size(255);null" json:"asal_instansi"`
	FotoProfil   string    `orm:"size(255);null" json:"foto_profil"`
	Password     string    `orm:"size(255)" json:"-"`
	Role         Role      `orm:"size(20)" json:"role"`
	CreatedAt    time.Time `orm:"auto_now_add;type(datetime)" json:"created_at"`
	UpdatedAt    time.Time `orm:"auto_now;type(datetime)" json:"updated_at"`
}

func (u *User) TableName() string {
	return "users"
}

// HashPassword hashes the user's password
func (u *User) HashPassword() error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.Password = string(hashedPassword)
	return nil
}

// CheckPassword verifies the password
func (u *User) CheckPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password))
	return err == nil
}

func init() {
	orm.RegisterModel(new(User))
}
