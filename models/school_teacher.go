package models

import (
	"time"

	"github.com/beego/beego/v2/client/orm"
)

// SchoolTeacher merepresentasikan satu guru yang terikat ke sebuah akun
// sekolah. Guru login menggunakan email mereka sendiri (kolom Email pada
// tabel ini), namun password yang diverifikasi adalah password milik akun
// sekolah induk (User dengan id = SchoolId).
type SchoolTeacher struct {
	Id           int       `orm:"auto;pk" json:"id"`
	SchoolId     int       `orm:"column(school_id)" json:"school_id"`
	Nama         string    `orm:"size(255)" json:"nama"`
	Kelas        string    `orm:"size(100)" json:"kelas"`
	Email        string    `orm:"size(255);unique" json:"email"`
	JenisKelamin string    `orm:"size(50);null" json:"jenis_kelamin"`
	CreatedAt    time.Time `orm:"auto_now_add;type(timestamp)" json:"created_at"`
}

func (t *SchoolTeacher) TableName() string {
	return "school_teachers"
}

func init() {
	orm.RegisterModel(new(SchoolTeacher))
}
