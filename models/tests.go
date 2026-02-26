package models

import (
	"time"

	"github.com/beego/beego/v2/client/orm"
)

// Purpose categories & details for pemeriksaan
const (
	PurposeCategoryEducation = "education"
	PurposeCategoryCareer    = "career"
	PurposeCategoryOther     = "other"
)

// Example enums for purpose_detail (you can extend these later)
const (
	PurposeDetailSekolah                 = "sekolah"
	PurposeDetailIdentifikasiKecerdasan  = "identifikasi_kecerdasan"
	PurposeDetailMenentukanJurusan       = "menentukan_jurusan"
	PurposeDetailPengembanganPotensi     = "pengembangan_potensi"
	PurposeDetailPenempatanKerja         = "penempatan_kerja"
	PurposeDetailLainnya                 = "lainnya"

	StatusBatchActive   = "active"
	StatusBatchArchived = "archived"
	StatusInvitationPending              = "pending"
	StatusInvitationUsed                 = "used"
	StatusInvitationExpired              = "expired"
	StatusInvitationCanceled             = "canceled"
	StatusInvitationArchived             = "archived"
)

// TestBatch represents satu sesi pemeriksaan (misal: Tes IQ IST Sekolah A)
type TestBatch struct {
	Id              int       `orm:"auto;pk" json:"id"`
	Name            string    `orm:"size(255)" json:"name"`
	Institution     string    `orm:"size(255)" json:"institution"`
	EnableIST       bool      `orm:"column(enable_ist);default(true)" json:"enable_ist"`
	EnableHolland   bool      `orm:"column(enable_holland);default(false)" json:"enable_holland"`
	PurposeCategory string    `orm:"column(purpose_category);size(50)" json:"purpose_category"`
	PurposeDetail   string    `orm:"column(purpose_detail);size(100)" json:"purpose_detail"`
	SendViaEmail    bool      `orm:"column(send_via_email);default(true)" json:"send_via_email"`
	SendViaBrowser  bool      `orm:"column(send_via_browser);default(false)" json:"send_via_browser"`
	Status          string    `orm:"column(status);size(20);default(active)" json:"status"`
	CreatedBy       int       `orm:"column(created_by)" json:"created_by"`
	CreatedAt       time.Time `orm:"column(created_at);auto_now_add;type(datetime)" json:"created_at"`
}

func (t *TestBatch) TableName() string {
	return "test_batches"
}

// TestInvitation menyimpan token undangan per peserta
type TestInvitation struct {
	Id         int       `orm:"auto;pk" json:"id"`
	BatchId    int       `json:"batch_id"`
	Email      string    `orm:"size(255)" json:"email"`
	UserId     *int      `orm:"null" json:"user_id,omitempty"`
	Token      string    `orm:"size(64);unique" json:"token"`
	ExpiresAt  time.Time `orm:"type(timestamp)" json:"expires_at"`
	UsedAt     time.Time `orm:"null;type(timestamp)" json:"used_at,omitempty"`
	Status     string    `orm:"size(20)" json:"status"`
	CreatedAt  time.Time `orm:"auto_now_add;type(datetime)" json:"created_at"`
}

func (t *TestInvitation) TableName() string {
	return "test_invitations"
}

// ISTSubtest: SE, WA, AN, ME, RA, ZA, FA, WU, GE
type ISTSubtest struct {
	Id         int    `orm:"auto;pk" json:"id"`
	Code       string `orm:"size(10);unique" json:"code"`
	Name       string `orm:"size(100)" json:"name"`
	OrderIndex int    `json:"order_index"`
}

func (s *ISTSubtest) TableName() string {
	return "ist_subtests"
}

// ISTQuestion: soal pilihan ganda (bisa teks atau didukung gambar)
type ISTQuestion struct {
	Id         int        `orm:"auto;pk" json:"id"`
	Subtest    *ISTSubtest `orm:"rel(fk)" json:"subtest"`
	Number     int        `json:"number"`
	Prompt     string     `orm:"type(text)" json:"prompt"`
	OptionA    string     `orm:"type(text)" json:"option_a"`
	OptionB    string     `orm:"type(text)" json:"option_b"`
	OptionC    string     `orm:"type(text)" json:"option_c"`
	OptionD    string     `orm:"type(text)" json:"option_d"`
	OptionE    string     `orm:"type(text)" json:"option_e"`
	Correct    string     `orm:"column(correct_option);size(1)" json:"correct_option"`
	ImageURL   string     `orm:"column(image_url);null;type(text)" json:"image_url,omitempty"`
}

func (q *ISTQuestion) TableName() string {
	return "ist_questions"
}

// ISTAnswer: jawaban per butir
type ISTAnswer struct {
	Id          int         `orm:"auto;pk" json:"id"`
	Invitation  *TestInvitation `orm:"rel(fk)" json:"invitation"`
	User        *User       `orm:"rel(fk)" json:"user"`
	Subtest     *ISTSubtest `orm:"rel(fk)" json:"subtest"`
	Question    *ISTQuestion `orm:"rel(fk)" json:"question"`
	Answer      string      `orm:"size(1)" json:"answer_option"`
	IsCorrect   bool        `json:"is_correct"`
	AnsweredAt  time.Time   `orm:"auto_now_add;type(datetime)" json:"answered_at"`
}

func (a *ISTAnswer) TableName() string {
	return "ist_answers"
}

// ISTResult: ringkasan skor per subtes + IQ
type ISTResult struct {
	Id                 int       `orm:"auto;pk" json:"id"`
	Invitation         *TestInvitation `orm:"rel(one);on_delete(cascade)" json:"invitation"`
	User               *User     `orm:"rel(fk)" json:"user"`
	RawSE              int       `json:"raw_se"`
	RawWA              int       `json:"raw_wa"`
	RawAN              int       `json:"raw_an"`
	RawME              int       `json:"raw_me"`
	RawRA              int       `json:"raw_ra"`
	RawZA              int       `json:"raw_za"`
	RawFA              int       `json:"raw_fa"`
	RawWU              int       `json:"raw_wu"`
	RawGE              int       `json:"raw_ge"`
	StdSE              int       `json:"std_se"`
	StdWA              int       `json:"std_wa"`
	StdAN              int       `json:"std_an"`
	StdME              int       `json:"std_me"`
	StdRA              int       `json:"std_ra"`
	StdZA              int       `json:"std_za"`
	StdFA              int       `json:"std_fa"`
	StdWU              int       `json:"std_wu"`
	StdGE              int       `json:"std_ge"`
	TotalStandardScore int       `json:"total_standard_score"`
	IQ                 int       `json:"iq"`
	IQCategory         string    `orm:"size(100)" json:"iq_category"`
	Strengths          string    `orm:"type(text)" json:"strengths"`
	Weaknesses         string    `orm:"type(text)" json:"weaknesses"`
	Summary            string    `orm:"type(text)" json:"summary"`
	CreatedAt          time.Time `orm:"auto_now_add;type(datetime)" json:"created_at"`
}

func (r *ISTResult) TableName() string {
	return "ist_results"
}

// ISTNorm: raw score -> standard score per usia
type ISTNorm struct {
	Id            int    `orm:"auto;pk" json:"id"`
	SubtestCode   string `orm:"size(10)" json:"subtest_code"`
	AgeMin        int    `json:"age_min"`
	AgeMax        int    `json:"age_max"`
	RawScore      int    `json:"raw_score"`
	StandardScore int    `json:"standard_score"`
}

func (n *ISTNorm) TableName() string {
	return "ist_norms"
}

// ISTIQNorm: total standard score -> IQ
type ISTIQNorm struct {
	Id                 int    `orm:"auto;pk" json:"id"`
	TotalStandardScore int    `json:"total_standard_score"`
	IQ                 int    `json:"iq"`
	Category           string `orm:"size(100)" json:"category"`
}

func (n *ISTIQNorm) TableName() string {
	return "ist_iq_norms"
}

// HollandQuestion: item untuk RIASEC
type HollandQuestion struct {
	Id         int    `orm:"auto;pk" json:"id"`
	Code       string `orm:"size(1)" json:"code"` // R, I, A, S, E, C
	Number     int    `json:"number"`
	Prompt     string `orm:"type(text)" json:"prompt"`
	AnswerType string `orm:"size(20)" json:"answer_type"` // yes_no, scale
}

func (q *HollandQuestion) TableName() string {
	return "holland_questions"
}

type HollandDescription struct {
	Id                int    `orm:"auto;pk" json:"id"`
	Code              string `orm:"size(1);unique" json:"code"`
	Title             string `orm:"size(100)" json:"title"`
	Description       string `orm:"type(text)" json:"description"`
	RecommendedMajors string `orm:"type(text)" json:"recommended_majors"`
	RecommendedJobs   string `orm:"type(text)" json:"recommended_jobs"`
}

func (d *HollandDescription) TableName() string {
	return "holland_descriptions"
}

type HollandAnswer struct {
	Id          int              `orm:"auto;pk" json:"id"`
	Invitation  *TestInvitation  `orm:"rel(fk)" json:"invitation"`
	User        *User            `orm:"rel(fk)" json:"user"`
	Question    *HollandQuestion `orm:"rel(fk)" json:"question"`
	Value       int              `json:"value"`
	AnsweredAt  time.Time        `orm:"auto_now_add;type(datetime)" json:"answered_at"`
}

func (a *HollandAnswer) TableName() string {
	return "holland_answers"
}

type HollandResult struct {
	Id        int             `orm:"auto;pk" json:"id"`
	Invitation *TestInvitation `orm:"rel(one);on_delete(cascade)" json:"invitation"`
	User      *User           `orm:"rel(fk)" json:"user"`
	ScoreR    int             `json:"score_r"`
	ScoreI    int             `json:"score_i"`
	ScoreA    int             `json:"score_a"`
	ScoreS    int             `json:"score_s"`
	ScoreE    int             `json:"score_e"`
	ScoreC    int             `json:"score_c"`
	Top1      string          `orm:"size(1)" json:"top1"`
	Top2      string          `orm:"size(1)" json:"top2"`
	Top3      string          `orm:"size(1)" json:"top3"`
	Code      string          `orm:"size(3)" json:"code"`
	Interpretation string     `orm:"type(text)" json:"interpretation"`
	CreatedAt time.Time       `orm:"auto_now_add;type(datetime)" json:"created_at"`
}

func (r *HollandResult) TableName() string {
	return "holland_results"
}

func init() {
	orm.RegisterModel(
		new(TestBatch),
		new(TestInvitation),
		new(ISTSubtest),
		new(ISTQuestion),
		new(ISTAnswer),
		new(ISTResult),
		new(ISTNorm),
		new(ISTIQNorm),
		new(HollandQuestion),
		new(HollandDescription),
		new(HollandAnswer),
		new(HollandResult),
	)
}

