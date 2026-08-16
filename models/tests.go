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
	TahunAjaran     string    `orm:"column(tahun_ajaran);size(50);null" json:"tahun_ajaran"`
	Sekolah         string    `orm:"column(sekolah);size(100);null" json:"sekolah"`
	JenjangSekolah  string    `orm:"column(jenjang_sekolah);size(50);null" json:"jenjang_sekolah"`
	Kelas           string    `orm:"column(kelas);size(20);null" json:"kelas"`
	Jurusan         string    `orm:"column(jurusan);size(100);null" json:"jurusan"`
	EnableIST       bool      `orm:"column(enable_ist);default(true)" json:"enable_ist"`
	EnableHolland   bool      `orm:"column(enable_holland);default(false)" json:"enable_holland"`
	EnableLearningStyle bool  `orm:"column(enable_learning_style);default(false)" json:"enable_learning_style"`
	EnableKraepelin bool      `orm:"column(enable_kraepelin);default(false)" json:"enable_kraepelin"`
	EnableRMIB      bool      `orm:"column(enable_rmib);default(false)" json:"enable_rmib"`
	EnablePAPI      bool      `orm:"column(enable_papi);default(false)" json:"enable_papi"`
	TestOrder       string    `orm:"column(test_order);type(text);null" json:"test_order"`
	PurposeCategory string    `orm:"column(purpose_category);size(50)" json:"purpose_category"`
	PurposeDetail   string    `orm:"column(purpose_detail);size(100)" json:"purpose_detail"`
	SendViaEmail    bool      `orm:"column(send_via_email);default(true)" json:"send_via_email"`
	SendViaBrowser  bool      `orm:"column(send_via_browser);default(false)" json:"send_via_browser"`
	SendViaWhatsApp bool      `orm:"column(send_via_whatsapp);default(false)" json:"send_via_whatsapp"`
	Status          string    `orm:"column(status);size(20);default(active)" json:"status"`
	CreatedBy       int       `orm:"column(created_by)" json:"created_by"`
	TeacherId       *int      `orm:"column(teacher_id);null" json:"teacher_id"`
	CreatedAt       time.Time `orm:"column(created_at);auto_now_add;type(datetime)" json:"created_at"`
}

func (t *TestBatch) TableName() string {
	return "test_batches"
}

// TestInvitation menyimpan token undangan per peserta
type TestInvitation struct {
	Id         int       `orm:"auto;pk" json:"id"`
	BatchId    *int      `orm:"null" json:"batch_id,omitempty"` // Bisa NULL jika batch sudah dihapus
	Email      string    `orm:"size(255)" json:"email"`
	Phone      string    `orm:"column(phone);size(20)" json:"phone"`
	UserId     *int      `orm:"null" json:"user_id,omitempty"`
	Token      string    `orm:"size(64);unique" json:"token"`
	ExpiresAt  time.Time `orm:"type(timestamp)" json:"expires_at"`
	UsedAt     time.Time `orm:"null;type(timestamp)" json:"used_at,omitempty"`
	Status     string    `orm:"size(20)" json:"status"`
	TeacherId  *int      `orm:"column(teacher_id);null" json:"teacher_id"`
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
	// IMPORTANT: kolom di DB (lihat migrations/000011_create_tests_tables.up.sql) bernama `answer_option`.
	// Tanpa `column(answer_option)`, Beego ORM akan memakai nama default `answer` -> insert gagal diam-diam
	// (karena beberapa controller masih mengabaikan error). Ini penyebab export kosong.
	Answer      string      `orm:"column(answer_option);size(255)" json:"answer_option"`
	// Score menyimpan skor per-butir (untuk subtest GE bisa 0/1/2).
	// Untuk subtest lain umumnya 0/1 (benar/salah).
	Score       int         `orm:"column(score);default(0)" json:"score"`
	IsCorrect   bool        `json:"is_correct"`
	AnsweredAt  time.Time   `orm:"auto_now_add;type(datetime)" json:"answered_at"`
}

func (a *ISTAnswer) TableName() string {
	return "ist_answers"
}

// ISTResult: ringkasan skor per subtes + IQ
type ISTResult struct {
	Id                 int             `orm:"auto;pk" json:"id"`
	Invitation         *TestInvitation `orm:"rel(one);on_delete(cascade)" json:"invitation"`
	User               *User           `orm:"rel(fk)" json:"user"`
	// Raw scores: kolom di DB adalah snake_case tanpa extra underscore (raw_se, raw_wa, dst.)
	RawSE              int       `orm:"column(raw_se)" json:"raw_se"`
	RawWA              int       `orm:"column(raw_wa)" json:"raw_wa"`
	RawAN              int       `orm:"column(raw_an)" json:"raw_an"`
	RawME              int       `orm:"column(raw_me)" json:"raw_me"`
	RawRA              int       `orm:"column(raw_ra)" json:"raw_ra"`
	RawZA              int       `orm:"column(raw_za)" json:"raw_za"`
	RawFA              int       `orm:"column(raw_fa)" json:"raw_fa"`
	RawWU              int       `orm:"column(raw_wu)" json:"raw_wu"`
	RawGE              int       `orm:"column(raw_ge)" json:"raw_ge"`
	// Standard scores (SW): std_se, std_wa, dst.
	StdSE              int       `orm:"column(std_se)" json:"std_se"`
	StdWA              int       `orm:"column(std_wa)" json:"std_wa"`
	StdAN              int       `orm:"column(std_an)" json:"std_an"`
	StdME              int       `orm:"column(std_me)" json:"std_me"`
	StdRA              int       `orm:"column(std_ra)" json:"std_ra"`
	StdZA              int       `orm:"column(std_za)" json:"std_za"`
	StdFA              int       `orm:"column(std_fa)" json:"std_fa"`
	StdWU              int       `orm:"column(std_wu)" json:"std_wu"`
	StdGE              int       `orm:"column(std_ge)" json:"std_ge"`
	// Total WS & IQ
	TotalStandardScore int       `orm:"column(total_standard_score)" json:"total_standard_score"`
	IQ                 int       `orm:"column(iq)" json:"iq"`
	IQCategory         string    `orm:"column(iq_category);size(100)" json:"iq_category"`
	Strengths          string    `orm:"column(strengths);type(text)" json:"strengths"`
	Weaknesses         string    `orm:"column(weaknesses);type(text)" json:"weaknesses"`
	Summary            string    `orm:"column(summary);type(text)" json:"summary"`
	CreatedAt          time.Time `orm:"column(created_at);auto_now_add;type(datetime)" json:"created_at"`
}

func (r *ISTResult) TableName() string {
	return "ist_results"
}

// ISTProgress: tracking progress peserta mengerjakan subtest IST
// Setiap kali submit subtest, akan tercatat di sini untuk tracking & export
type ISTProgress struct {
	Id          int            `orm:"auto;pk" json:"id"`
	Invitation  *TestInvitation `orm:"rel(fk)" json:"invitation"`
	SubtestCode string         `orm:"size(10)" json:"subtest_code"` // SE, WA, AN, GE, RA, ZR, FA, WU, ME
	Status      string         `orm:"size(20)" json:"status"`       // completed, in_progress
	CompletedAt time.Time      `orm:"auto_now_add;type(datetime)" json:"completed_at"`
}

func (p *ISTProgress) TableName() string {
	return "ist_progress"
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

// ISTIQNorm: total standard score -> IQ (age-dependent)
type ISTIQNorm struct {
	Id                 int    `orm:"auto;pk" json:"id"`
	TotalStandardScore int    `orm:"column(total_standard_score)" json:"total_standard_score"`
	AgeMin             int    `orm:"column(age_min)" json:"age_min"`
	AgeMax             int    `orm:"column(age_max)" json:"age_max"`
	IQ                 int    `orm:"column(iq)" json:"iq"`
	Category           string `orm:"column(category);size(100)" json:"category"`
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
	// Extra answers (page 3)
	DreamJob1          string `orm:"column(dream_job_1);type(text)" json:"dream_job_1"`
	DreamJob2          string `orm:"column(dream_job_2);type(text)" json:"dream_job_2"`
	DreamJob3          string `orm:"column(dream_job_3);type(text)" json:"dream_job_3"`
	FavoriteSubject    string `orm:"column(favorite_subject);type(text)" json:"favorite_subject"`
	DislikedSubject    string `orm:"column(disliked_subject);type(text)" json:"disliked_subject"`
	CreatedAt time.Time       `orm:"auto_now_add;type(datetime)" json:"created_at"`
}

func (r *HollandResult) TableName() string {
	return "holland_results"
}

type LearningStyleQuestion struct {
	Id        int    `orm:"auto;pk" json:"id"`
	Number    int    `orm:"unique" json:"number"`
	Statement string `orm:"type(text)" json:"statement"`
	Dimension string `orm:"size(1)" json:"dimension"` // V, A, K
}

func (q *LearningStyleQuestion) TableName() string {
	return "learning_style_questions"
}

type LearningStyleAnswer struct {
	Id         int                   `orm:"auto;pk" json:"id"`
	Invitation *TestInvitation       `orm:"rel(fk)" json:"invitation"`
	User       *User                 `orm:"rel(fk)" json:"user"`
	Question   *LearningStyleQuestion `orm:"rel(fk)" json:"question"`
	AnswerYes  int                   `orm:"column(answer_yes);default(0)" json:"answer_yes"`
	AnswerNo   int                   `orm:"column(answer_no);default(0)" json:"answer_no"`
	AnsweredAt time.Time             `orm:"auto_now_add;type(datetime)" json:"answered_at"`
}

func (a *LearningStyleAnswer) TableName() string {
	return "learning_style_answers"
}

type LearningStyleResult struct {
	Id               int             `orm:"auto;pk" json:"id"`
	Invitation       *TestInvitation `orm:"rel(one);on_delete(cascade)" json:"invitation"`
	User             *User           `orm:"rel(fk)" json:"user"`
	TestName         string          `orm:"column(test_name);size(255)" json:"test_name"`
	TestAge          int             `orm:"column(test_age)" json:"test_age"`
	TestInstitution  string          `orm:"column(test_institution);size(255)" json:"test_institution"`
	TestGender       string          `orm:"column(test_gender);size(20)" json:"test_gender"`
	TestDate         time.Time       `orm:"column(test_date);type(datetime)" json:"test_date"`
	ScoreVisual      int             `orm:"column(score_visual);default(0)" json:"score_visual"`
	ScoreAuditory    int             `orm:"column(score_auditory);default(0)" json:"score_auditory"`
	ScoreKinesthetic int             `orm:"column(score_kinesthetic);default(0)" json:"score_kinesthetic"`
	DominantType     string          `orm:"column(dominant_type);size(20)" json:"dominant_type"`
	InterpretationVisual string      `orm:"column(interpretation_visual);type(text)" json:"interpretation_visual"`
	InterpretationAuditory string    `orm:"column(interpretation_auditory);type(text)" json:"interpretation_auditory"`
	InterpretationKinesthetic string `orm:"column(interpretation_kinesthetic);type(text)" json:"interpretation_kinesthetic"`
	CreatedAt         time.Time      `orm:"auto_now_add;type(datetime)" json:"created_at"`
}

func (r *LearningStyleResult) TableName() string {
	return "learning_style_results"
}

// KraepelinAttempt menyimpan satu attempt (1 invitation) untuk tes Kraepelin.
// Soal & jawaban disimpan sebagai JSON agar fleksibel (50 kolom x 27 angka, 50 kolom x 26 jawaban).
type KraepelinAttempt struct {
	Id         int             `orm:"auto;pk" json:"id"`
	Invitation *TestInvitation `orm:"rel(one);on_delete(cascade)" json:"invitation"`
	User       *User           `orm:"rel(fk)" json:"user"`

	// Biodata sesuai kebutuhan tes
	TestName        string    `orm:"column(test_name);size(255)" json:"test_name"`
	TestGender      string    `orm:"column(test_gender);size(20)" json:"test_gender"` // laki-laki / perempuan
	TestBirthPlace  string    `orm:"column(test_birth_place);size(255)" json:"test_birth_place"`
	// Simpan sebagai teks YYYY-MM-DD: Beego + pq sering mengirim time.Time sebagai literal yang
	// ditolak oleh kolom DATE/TIMESTAMPTZ ("2005-06-07 00:00:00Z").
	TestBirthDate *string `orm:"column(test_birth_date);null;size(10)" json:"test_birth_date,omitempty"`
	TestAge         int       `orm:"column(test_age);default(0)" json:"test_age"`
	TestAddress     string    `orm:"column(test_address);type(text)" json:"test_address"`
	TestEducation   string    `orm:"column(test_education);size(255)" json:"test_education"`
	TestMajor       string    `orm:"column(test_major);size(255)" json:"test_major"`
	TestJob         string    `orm:"column(test_job);size(255);null" json:"test_job,omitempty"`
	Tester          string    `orm:"column(tester);size(255)" json:"tester"`
	TestDate        time.Time `orm:"column(test_date);type(datetime)" json:"test_date"`

	// Konfigurasi timing
	ColumnCount           int `orm:"column(column_count);default(40)" json:"column_count"`
	DigitsPerColumn       int `orm:"column(digits_per_column);default(27)" json:"digits_per_column"`
	SecondsPerColumn      int `orm:"column(seconds_per_column);default(30)" json:"seconds_per_column"`
	GraceSecondsOnSwitch  int `orm:"column(grace_seconds_on_switch);default(0)" json:"grace_seconds_on_switch"`

	// Payload JSON
	DigitsJSON       string `orm:"column(digits_json);type(text)" json:"digits_json"`         // [][]int
	AnswersJSON      string `orm:"column(answers_json);type(text);null" json:"answers_json"`  // [][]*int (nil=skip)
	CorrectCountsJSON string `orm:"column(correct_counts_json);type(text);null" json:"correct_counts_json"` // []int len 40

	// Summary
	TotalCorrect int `orm:"column(total_correct);default(0)" json:"total_correct"`
	TotalErrors  int `orm:"column(total_errors);default(0)" json:"total_errors"`
	TotalSkipped int `orm:"column(total_skipped);default(0)" json:"total_skipped"`

	Status    string    `orm:"column(status);size(20);default(in_progress)" json:"status"` // in_progress, finished
	StartedAt time.Time `orm:"column(started_at);auto_now_add;type(datetime)" json:"started_at"`
	FinishedAt time.Time `orm:"column(finished_at);null;type(datetime)" json:"finished_at,omitempty"`
	CreatedAt time.Time `orm:"column(created_at);auto_now_add;type(datetime)" json:"created_at"`
}

func (a *KraepelinAttempt) TableName() string {
	return "kraepelin_attempts"
}

// RMIBQuestion: master 96 item RMIB per gender_version (pria/wanita), 8 kelompok x 12 aktivitas.
type RMIBQuestion struct {
	Id               int    `orm:"auto;pk" json:"id"`
	GenderVersion    string `orm:"column(gender_version);size(10)" json:"gender_version"`
	GroupNumber      int    `orm:"column(group_number)" json:"group_number"`
	GroupTitle       string `orm:"column(group_title);size(255)" json:"group_title"`
	GroupDescription string `orm:"column(group_description);type(text);null" json:"group_description"`
	ItemOrder        int    `orm:"column(item_order)" json:"item_order"`
	QuestionText     string `orm:"column(question_text);type(text)" json:"question_text"`
	CategoryCode     string `orm:"column(category_code);size(8)" json:"category_code"`
}

func (q *RMIBQuestion) TableName() string {
	return "rmib_questions"
}

// RMIBSession: satu attempt RMIB per invitation.
type RMIBSession struct {
	Id            int             `orm:"auto;pk" json:"id"`
	Invitation    *TestInvitation `orm:"rel(one);on_delete(cascade)" json:"invitation"`
	User          *User           `orm:"rel(fk)" json:"user"`
	BatchId       *int            `orm:"column(batch_id);null" json:"batch_id,omitempty"`
	GenderVersion string          `orm:"column(gender_version);size(10)" json:"gender_version"`
	Status        string          `orm:"column(status);size(20);default(in_progress)" json:"status"`
	StartedAt     time.Time       `orm:"column(started_at);auto_now_add;type(datetime)" json:"started_at"`
	CompletedAt   time.Time       `orm:"column(completed_at);null;type(datetime)" json:"completed_at,omitempty"`
}

func (s *RMIBSession) TableName() string {
	return "rmib_sessions"
}

// RMIBAnswer: ranking 1-12 per item; UPSERT by (session_id, question_id).
type RMIBAnswer struct {
	Id           int           `orm:"auto;pk" json:"id"`
	Session      *RMIBSession  `orm:"rel(fk);on_delete(cascade)" json:"session"`
	GroupNumber  int           `orm:"column(group_number)" json:"group_number"`
	Question     *RMIBQuestion `orm:"rel(fk);on_delete(cascade)" json:"question"`
	SelectedRank int           `orm:"column(selected_rank)" json:"selected_rank"`
	UpdatedAt    time.Time     `orm:"column(updated_at);auto_now;type(datetime)" json:"updated_at"`
}

func (a *RMIBAnswer) TableName() string {
	return "rmib_answers"
}

// RMIBResult: ringkasan skor + top-3 per invitation.
type RMIBResult struct {
	Id               int             `orm:"auto;pk" json:"id"`
	Invitation       *TestInvitation `orm:"rel(one);on_delete(cascade)" json:"invitation"`
	User             *User           `orm:"rel(fk)" json:"user"`
	GenderVersion    string          `orm:"column(gender_version);size(10)" json:"gender_version"`
	ResultJSON       string          `orm:"column(result_json);type(text)" json:"result_json"`
	DominantCategory string          `orm:"column(dominant_category);size(8)" json:"dominant_category"`
	Top1             string          `orm:"column(top1);size(8)" json:"top1"`
	Top2             string          `orm:"column(top2);size(8)" json:"top2"`
	Top3             string          `orm:"column(top3);size(8)" json:"top3"`
	Interpretation   string          `orm:"column(interpretation);type(text);null" json:"interpretation"`
	CompletedAt      time.Time       `orm:"column(completed_at);auto_now_add;type(datetime)" json:"completed_at"`
}

func (r *RMIBResult) TableName() string {
	return "rmib_results"
}

// PAPIQuestion: master 90 item PAPI dengan format paired comparison.
// Setiap soal berisi 2 pernyataan (OptionA = atas, OptionB = bawah), peserta memilih SATU.
type PAPIQuestion struct {
	Id         int    `orm:"auto;pk" json:"id"`
	ItemNumber int    `orm:"column(item_number)" json:"item_number"`
	OptionA    string `orm:"column(option_a);type(text)" json:"option_a"`
	OptionB    string `orm:"column(option_b);type(text)" json:"option_b"`
	CategoryA  string `orm:"column(category_a);size(8)" json:"category_a"`
	CategoryB  string `orm:"column(category_b);size(8)" json:"category_b"`
}

func (q *PAPIQuestion) TableName() string {
	return "papi_questions"
}

// PAPISession: satu attempt PAPI per invitation
type PAPISession struct {
	Id                   int             `orm:"auto;pk" json:"id"`
	Invitation           *TestInvitation `orm:"rel(one);on_delete(cascade)" json:"invitation"`
	User                 *User           `orm:"rel(fk)" json:"user"`
	BatchId              *int            `orm:"column(batch_id);null" json:"batch_id,omitempty"`
	Status               string          `orm:"column(status);size(20);default(in_progress)" json:"status"`
	StartedAt            time.Time       `orm:"column(started_at);auto_now_add;type(datetime)" json:"started_at"`
	CompletedAt          time.Time       `orm:"column(completed_at);null;type(datetime)" json:"completed_at,omitempty"`
	TimeLimitMinutes     int             `orm:"column(time_limit_minutes);default(60)" json:"time_limit_minutes"`
	TimeRemainingSeconds int             `orm:"column(time_remaining_seconds);default(3600)" json:"time_remaining_seconds"`
}

func (s *PAPISession) TableName() string {
	return "papi_sessions"
}

// PAPIAnswer: jawaban A/B/C/D per item; UPSERT by (session_id, question_id)
type PAPIAnswer struct {
	Id             int           `orm:"auto;pk" json:"id"`
	Session        *PAPISession  `orm:"rel(fk);on_delete(cascade)" json:"session"`
	Question       *PAPIQuestion `orm:"rel(fk);on_delete(cascade)" json:"question"`
	SelectedOption string        `orm:"column(selected_option);size(1)" json:"selected_option"`
	UpdatedAt      time.Time     `orm:"column(updated_at);auto_now;type(datetime)" json:"updated_at"`
}

func (a *PAPIAnswer) TableName() string {
	return "papi_answers"
}

// PAPIResult: ringkasan skor per invitation
type PAPIResult struct {
	Id               int             `orm:"auto;pk" json:"id"`
	Invitation       *TestInvitation `orm:"rel(one);on_delete(cascade)" json:"invitation"`
	User             *User           `orm:"rel(fk)" json:"user"`
	ResultJSON       string          `orm:"column(result_json);type(text)" json:"result_json"`
	DominantCategory string          `orm:"column(dominant_category);size(8)" json:"dominant_category"`
	TopCategories    string          `orm:"column(top_categories);type(text)" json:"top_categories"`
	Interpretation   string          `orm:"column(interpretation);type(text);null" json:"interpretation"`
	CompletedAt      time.Time       `orm:"column(completed_at);auto_now_add;type(datetime)" json:"completed_at"`
	TimeTakenMinutes int             `orm:"column(time_taken_minutes)" json:"time_taken_minutes"`
}

func (r *PAPIResult) TableName() string {
	return "papi_results"
}

func init() {
	orm.RegisterModel(
		new(TestBatch),
		new(TestInvitation),
		new(ISTSubtest),
		new(ISTQuestion),
		new(ISTAnswer),
		new(ISTResult),
		new(ISTProgress),
		new(ISTNorm),
		new(ISTIQNorm),
		new(HollandQuestion),
		new(HollandDescription),
		new(HollandAnswer),
		new(HollandResult),
		new(LearningStyleQuestion),
		new(LearningStyleAnswer),
		new(LearningStyleResult),
		new(KraepelinAttempt),
		new(RMIBQuestion),
		new(RMIBSession),
		new(RMIBAnswer),
		new(RMIBResult),
		new(PAPIQuestion),
		new(PAPISession),
		new(PAPIAnswer),
		new(PAPIResult),
	)
}

// EnsureISTProgressTable creates ist_progress table if not exists
// Dipanggil dari main.go setelah database ready
func EnsureISTProgressTable() error {
	o := orm.NewOrm()
	_, err := o.Raw(`
		CREATE TABLE IF NOT EXISTS ist_progress (
			id SERIAL PRIMARY KEY,
			invitation_id INT NOT NULL REFERENCES test_invitations(id) ON DELETE CASCADE,
			subtest_code VARCHAR(10) NOT NULL,
			status VARCHAR(20) NOT NULL DEFAULT 'completed',
			completed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(invitation_id, subtest_code)
		);
	`).Exec()
	if err != nil {
		return err
	}
	
	// Create indexes if not exists
	o.Raw(`CREATE INDEX IF NOT EXISTS idx_ist_progress_invitation_id ON ist_progress(invitation_id);`).Exec()
	o.Raw(`CREATE INDEX IF NOT EXISTS idx_ist_progress_subtest_code ON ist_progress(subtest_code);`).Exec()
	
	return nil
}

// EnsureHollandExtraFields ensures columns exist on holland_results.
// This makes the app safer even if migrations haven't been run yet.
func EnsureHollandExtraFields() error {
	o := orm.NewOrm()
	_, err := o.Raw(`
		ALTER TABLE IF EXISTS holland_results
		ADD COLUMN IF NOT EXISTS dream_job_1 TEXT,
		ADD COLUMN IF NOT EXISTS dream_job_2 TEXT,
		ADD COLUMN IF NOT EXISTS dream_job_3 TEXT,
		ADD COLUMN IF NOT EXISTS favorite_subject TEXT,
		ADD COLUMN IF NOT EXISTS disliked_subject TEXT;
	`).Exec()
	return err
}

// EnsureLearningStyleTables ensures VAK schema exists even if migrations weren't run.
func EnsureLearningStyleTables() error {
	o := orm.NewOrm()
	// Add toggle column on batch table.
	if _, err := o.Raw(`
		ALTER TABLE IF EXISTS test_batches
		ADD COLUMN IF NOT EXISTS enable_learning_style BOOLEAN NOT NULL DEFAULT FALSE;
	`).Exec(); err != nil {
		return err
	}

	// Add Kraepelin toggle column on batch table.
	if _, err := o.Raw(`
		ALTER TABLE IF EXISTS test_batches
		ADD COLUMN IF NOT EXISTS enable_kraepelin BOOLEAN NOT NULL DEFAULT FALSE;
	`).Exec(); err != nil {
		return err
	}

	// Create master questions table.
	if _, err := o.Raw(`
		CREATE TABLE IF NOT EXISTS learning_style_questions (
			id SERIAL PRIMARY KEY,
			number INT NOT NULL UNIQUE,
			statement TEXT NOT NULL,
			dimension CHAR(1) NOT NULL
		);
	`).Exec(); err != nil {
		return err
	}

	// Create answers table.
	if _, err := o.Raw(`
		CREATE TABLE IF NOT EXISTS learning_style_answers (
			id SERIAL PRIMARY KEY,
			invitation_id INT NOT NULL REFERENCES test_invitations(id) ON DELETE CASCADE,
			user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			question_id INT NOT NULL REFERENCES learning_style_questions(id) ON DELETE CASCADE,
			answer_yes INT NOT NULL DEFAULT 0,
			answer_no INT NOT NULL DEFAULT 0,
			answered_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(invitation_id, question_id),
			CONSTRAINT learning_style_answer_binary_check CHECK (
				(answer_yes = 1 AND answer_no = 0) OR
				(answer_yes = 0 AND answer_no = 1)
			)
		);
	`).Exec(); err != nil {
		return err
	}

	// Create result table.
	if _, err := o.Raw(`
		CREATE TABLE IF NOT EXISTS learning_style_results (
			id SERIAL PRIMARY KEY,
			invitation_id INT NOT NULL UNIQUE REFERENCES test_invitations(id) ON DELETE CASCADE,
			user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			test_name VARCHAR(255) NOT NULL DEFAULT '',
			test_age INT NOT NULL DEFAULT 0,
			test_institution VARCHAR(255) NOT NULL DEFAULT '',
			test_gender VARCHAR(20) NOT NULL DEFAULT '',
			test_date TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			score_visual INT NOT NULL DEFAULT 0,
			score_auditory INT NOT NULL DEFAULT 0,
			score_kinesthetic INT NOT NULL DEFAULT 0,
			dominant_type VARCHAR(20) NOT NULL DEFAULT '',
			interpretation_visual TEXT,
			interpretation_auditory TEXT,
			interpretation_kinesthetic TEXT,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
	`).Exec(); err != nil {
		return err
	}

	// Indexes (safe if already exist)
	_, _ = o.Raw(`CREATE INDEX IF NOT EXISTS idx_learning_style_answers_invitation ON learning_style_answers(invitation_id);`).Exec()
	_, _ = o.Raw(`CREATE INDEX IF NOT EXISTS idx_learning_style_answers_user ON learning_style_answers(user_id);`).Exec()
	_, _ = o.Raw(`CREATE INDEX IF NOT EXISTS idx_learning_style_results_user ON learning_style_results(user_id);`).Exec()

	return nil
}

// EnsureKraepelinTables ensures Kraepelin schema exists even if migrations weren't run.
func EnsureKraepelinTables() error {
	o := orm.NewOrm()

	// Add toggle column on batch table (safe untuk MySQL & PostgreSQL baru).
	_, _ = o.Raw(`
		ALTER TABLE test_batches
		ADD COLUMN IF NOT EXISTS enable_kraepelin BOOLEAN NOT NULL DEFAULT FALSE;
	`).Exec()

	// Attempt table.
	// Catatan:
	// - Sintaks disesuaikan agar kompatibel dengan MySQL/MariaDB yang umum dipakai di Laragon.
	// - Jika kamu sudah punya migration SQL sendiri, fungsi ini hanya sebagai fallback
	//   dan tidak akan error kalau tabel sudah ada.
	_, _ = o.Raw(`
		CREATE TABLE IF NOT EXISTS kraepelin_attempts (
			id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			invitation_id INT NOT NULL UNIQUE,
			user_id INT NOT NULL,
			test_name VARCHAR(255) NOT NULL DEFAULT '',
			test_gender VARCHAR(20) NOT NULL DEFAULT '',
			test_birth_place VARCHAR(255) NOT NULL DEFAULT '',
			test_birth_date VARCHAR(10) NULL,
			test_age INT NOT NULL DEFAULT 0,
			test_address TEXT NOT NULL,
			test_education VARCHAR(255) NOT NULL DEFAULT '',
			test_major VARCHAR(255) NOT NULL DEFAULT '',
			test_job VARCHAR(255) NULL,
			tester VARCHAR(255) NOT NULL DEFAULT '',
			test_date DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			column_count INT NOT NULL DEFAULT 40,
			digits_per_column INT NOT NULL DEFAULT 27,
			seconds_per_column INT NOT NULL DEFAULT 30,
			grace_seconds_on_switch INT NOT NULL DEFAULT 0,
			digits_json LONGTEXT NOT NULL,
			answers_json LONGTEXT NULL,
			correct_counts_json LONGTEXT NULL,
			total_correct INT NOT NULL DEFAULT 0,
			total_errors INT NOT NULL DEFAULT 0,
			total_skipped INT NOT NULL DEFAULT 0,
			status VARCHAR(20) NOT NULL DEFAULT 'in_progress',
			started_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			finished_at DATETIME NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
	`).Exec()

	_, _ = o.Raw(`CREATE INDEX IF NOT EXISTS idx_kraepelin_attempts_user_id ON kraepelin_attempts(user_id);`).Exec()
	_, _ = o.Raw(`CREATE INDEX IF NOT EXISTS idx_kraepelin_attempts_invitation_id ON kraepelin_attempts(invitation_id);`).Exec()
	return nil
}

// EnsureRMIBTables membuat schema RMIB jika belum ada (fallback selain migration).
func EnsureRMIBTables() error {
	o := orm.NewOrm()

	// Toggle RMIB di test_batches.
	if _, err := o.Raw(`
		ALTER TABLE IF EXISTS test_batches
		ADD COLUMN IF NOT EXISTS enable_rmib BOOLEAN NOT NULL DEFAULT FALSE;
	`).Exec(); err != nil {
		return err
	}

	if _, err := o.Raw(`
		CREATE TABLE IF NOT EXISTS rmib_questions (
			id SERIAL PRIMARY KEY,
			gender_version VARCHAR(10) NOT NULL,
			group_number INT NOT NULL,
			group_title VARCHAR(255) NOT NULL,
			group_description TEXT,
			item_order INT NOT NULL,
			question_text TEXT NOT NULL,
			category_code VARCHAR(8) NOT NULL,
			UNIQUE (gender_version, group_number, item_order)
		);
	`).Exec(); err != nil {
		return err
	}
	_, _ = o.Raw(`CREATE INDEX IF NOT EXISTS idx_rmib_questions_gv_group ON rmib_questions(gender_version, group_number);`).Exec()

	if _, err := o.Raw(`
		CREATE TABLE IF NOT EXISTS rmib_sessions (
			id SERIAL PRIMARY KEY,
			invitation_id INT NOT NULL UNIQUE REFERENCES test_invitations(id) ON DELETE CASCADE,
			user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			batch_id INT REFERENCES test_batches(id) ON DELETE SET NULL,
			gender_version VARCHAR(10) NOT NULL,
			status VARCHAR(20) NOT NULL DEFAULT 'in_progress',
			started_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			completed_at TIMESTAMP NULL
		);
	`).Exec(); err != nil {
		return err
	}
	_, _ = o.Raw(`CREATE INDEX IF NOT EXISTS idx_rmib_sessions_user ON rmib_sessions(user_id);`).Exec()

	if _, err := o.Raw(`
		CREATE TABLE IF NOT EXISTS rmib_answers (
			id SERIAL PRIMARY KEY,
			session_id INT NOT NULL REFERENCES rmib_sessions(id) ON DELETE CASCADE,
			group_number INT NOT NULL,
			question_id INT NOT NULL REFERENCES rmib_questions(id) ON DELETE CASCADE,
			selected_rank INT NOT NULL CHECK (selected_rank BETWEEN 1 AND 12),
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE (session_id, question_id)
		);
	`).Exec(); err != nil {
		return err
	}
	_, _ = o.Raw(`CREATE INDEX IF NOT EXISTS idx_rmib_answers_session_group ON rmib_answers(session_id, group_number);`).Exec()

	if _, err := o.Raw(`
		CREATE TABLE IF NOT EXISTS rmib_results (
			id SERIAL PRIMARY KEY,
			invitation_id INT NOT NULL UNIQUE REFERENCES test_invitations(id) ON DELETE CASCADE,
			user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			gender_version VARCHAR(10) NOT NULL,
			result_json TEXT NOT NULL,
			dominant_category VARCHAR(8) NOT NULL,
			top1 VARCHAR(8) NOT NULL,
			top2 VARCHAR(8) NOT NULL,
			top3 VARCHAR(8) NOT NULL,
			interpretation TEXT,
			completed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
	`).Exec(); err != nil {
		return err
	}
	_, _ = o.Raw(`CREATE INDEX IF NOT EXISTS idx_rmib_results_user ON rmib_results(user_id);`).Exec()

	return nil
}

// EnsureBatchExtendedFields memastikan kolom baru tahun_ajaran, sekolah, kelas, jurusan, dan teacher_id ada di tabel yang tepat.
func EnsureBatchExtendedFields() error {
	o := orm.NewOrm()
	_, err := o.Raw(`
		ALTER TABLE test_batches
		ADD COLUMN IF NOT EXISTS tahun_ajaran VARCHAR(50),
		ADD COLUMN IF NOT EXISTS sekolah VARCHAR(100),
		ADD COLUMN IF NOT EXISTS teacher_id INTEGER,
		ADD COLUMN IF NOT EXISTS test_order TEXT;
	`).Exec()
	if err != nil {
		return err
	}
	// Tambah kolom kelas & jurusan jika belum ada.
	_, _ = o.Raw(`ALTER TABLE test_batches ADD COLUMN IF NOT EXISTS kelas VARCHAR(20);`).Exec()
	_, _ = o.Raw(`ALTER TABLE test_batches ADD COLUMN IF NOT EXISTS jurusan VARCHAR(100);`).Exec()
	_, err = o.Raw(`
		ALTER TABLE test_invitations
		ADD COLUMN IF NOT EXISTS teacher_id INTEGER;
	`).Exec()
	return err
}

// EnsurePAPITables memastikan schema PAPI ada meskipun migration belum dijalankan.
func EnsurePAPITables() error {
	o := orm.NewOrm()

	// Tambahkan toggle PAPI ke test_batches
	if _, err := o.Raw(`
		ALTER TABLE test_batches
		ADD COLUMN IF NOT EXISTS enable_papi BOOLEAN NOT NULL DEFAULT FALSE;
	`).Exec(); err != nil {
		return err
	}

	// Master soal PAPI
	if _, err := o.Raw(`
		CREATE TABLE IF NOT EXISTS papi_questions (
			id SERIAL PRIMARY KEY,
			item_number INT NOT NULL UNIQUE,
			option_a TEXT NOT NULL,
			option_b TEXT NOT NULL,
			category_a VARCHAR(8) NOT NULL,
			category_b VARCHAR(8) NOT NULL
		);
	`).Exec(); err != nil {
		return err
	}
	_, _ = o.Raw(`CREATE INDEX IF NOT EXISTS idx_papi_questions_item ON papi_questions(item_number);`).Exec()

	// Sesi pengerjaan PAPI
	if _, err := o.Raw(`
		CREATE TABLE IF NOT EXISTS papi_sessions (
			id SERIAL PRIMARY KEY,
			invitation_id INT NOT NULL UNIQUE REFERENCES test_invitations(id) ON DELETE CASCADE,
			user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			batch_id INT REFERENCES test_batches(id) ON DELETE SET NULL,
			status VARCHAR(20) NOT NULL DEFAULT 'in_progress',
			started_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			completed_at TIMESTAMP NULL,
			time_limit_minutes INT NOT NULL DEFAULT 60,
			time_remaining_seconds INT NOT NULL DEFAULT 3600
		);
	`).Exec(); err != nil {
		return err
	}
	_, _ = o.Raw(`CREATE INDEX IF NOT EXISTS idx_papi_sessions_user ON papi_sessions(user_id);`).Exec()

	// Jawaban PAPI
	if _, err := o.Raw(`
		CREATE TABLE IF NOT EXISTS papi_answers (
			id SERIAL PRIMARY KEY,
			session_id INT NOT NULL REFERENCES papi_sessions(id) ON DELETE CASCADE,
			question_id INT NOT NULL REFERENCES papi_questions(id) ON DELETE CASCADE,
			selected_option CHAR(1) NOT NULL CHECK (selected_option IN ('A', 'B')),
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE (session_id, question_id)
		);
	`).Exec(); err != nil {
		return err
	}
	_, _ = o.Raw(`CREATE INDEX IF NOT EXISTS idx_papi_answers_session ON papi_answers(session_id);`).Exec()

	// Hasil akhir PAPI
	if _, err := o.Raw(`
		CREATE TABLE IF NOT EXISTS papi_results (
			id SERIAL PRIMARY KEY,
			invitation_id INT NOT NULL UNIQUE REFERENCES test_invitations(id) ON DELETE CASCADE,
			user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			result_json TEXT NOT NULL,
			dominant_category VARCHAR(8) NOT NULL,
			top_categories TEXT,
			interpretation TEXT,
			completed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			time_taken_minutes INT NOT NULL
		);
	`).Exec(); err != nil {
		return err
	}
	_, _ = o.Raw(`CREATE INDEX IF NOT EXISTS idx_papi_results_user ON papi_results(user_id);`).Exec()

	// Seed soal PAPI jika kosong
	var count int
	_ = o.Raw("SELECT COUNT(*) FROM papi_questions").QueryRow(&count)
	if count == 0 {
		_, err := o.Raw(`
			INSERT INTO papi_questions (item_number, option_a, option_b, category_a, category_b) VALUES
			(1,  'Saya seorang pekerja keras', 'Saya bukan seorang pemurung', 'G', 'E'),
			(2,  'Saya suka bekerja lebih baik dari yang lain', 'Saya senang menekuni pekerjaan yang saya lakukan sampai selesai', 'A', 'N'),
			(3,  'Saya suka memberi petunjuk kepada orang bagaimana melakukan sesuatu', 'Saya ingin melakukan sesuatu sebaik mungkin', 'P', 'A'),
			(4,  'Saya suka melakukan hal-hal yang lucu', 'Saya senang memberi tahu orang apa yang harus dikerjakannya', 'X', 'P'),
			(5,  'Saya suka bergabung dengan kelompok', 'Saya senang diperhatikan oleh kelompok', 'B', 'X'),
			(6,  'Saya suka membina suatu hubungan persahabatan antar pribadi', 'Saya suka berteman dengan suatu kelompok', 'O', 'B'),
			(7,  'Cepat berubah jika saya rasa hal itu diperlukan', 'Saya berusaha membina hubungan yang akrab dengan teman saya', 'Z', 'O'),
			(8,  'Saya suka membalas jika saya disakiti', 'Saya suka melakukan hal-hal yang baru dan berbeda', 'K', 'Z'),
			(9,  'Saya ingin atasan saya menyukai saya', 'Saya suka memberi tahu orang jika mereka salah', 'F', 'K'),
			(10, 'Saya suka mengikuti petunjuk-petunjuk yang diberikan kepada saya', 'Saya suka mendukung pendapat atasan saya', 'W', 'F'),
			(11, 'Saya berusaha sangat keras', 'Saya seorang teratur, saya menaruh semua barang pada tempatnya', 'G', 'C'),
			(12, 'Saya dapat membuat orang mau bekerja keras', 'Saya tidak mudah marah', 'L', 'C'),
			(13, 'Saya suka memberi tahu kelompok apa yang harus dikerjakan', 'Saya selalu menekuni suatu pekerjaan sampai selesai', 'P', 'N'),
			(14, 'Saya ingin tampil menarik dan menakjubkan', 'Saya ingin menjadi orang yang berhasil', 'X', 'A'),
			(15, 'Saya ingin sesuai dan diterima dalam kelompok', 'Saya suka membantu orang lain mengambil sikap', 'B', 'P'),
			(16, 'Saya cemas jika seseorang tidak menyukai saya', 'Saya suka orang memperhatikan saya', 'O', 'X'),
			(17, 'Saya suka mencoba hal-hal baru', 'Saya lebih suka bekerja bersama orang lain daripada sendiri', 'Z', 'B'),
			(18, 'Saya kadang-kadang menyalahkan orang lain jika ada terjadi kesalahan', 'Saya merasa terganggu jika ada yang tidak menyukai saya', 'K', 'O'),
			(19, 'Saya suka mendukung pendapat atasan saya', 'Saya suka mencoba pekerjaan-pekerjaan yang baru dan berbeda', 'F', 'Z'),
			(20, 'Saya menyukai petunjuk terperinci dalam menyelesaikan permasalahan', 'Saya suka memberi tahu bila orang membuat saya kesal', 'W', 'K'),
			(21, 'Saya selalu berusaha keras', 'Saya suka melaksanakan setiap langkah dengan hati-hati', 'G', 'D'),
			(22, 'Saya seorang pemimpin yang baik', 'Saya dapat mengorganisir suatu pekerjaan dengan baik', 'L', 'C'),
			(23, 'Saya mudah tersinggung', 'Saya lambat dalam membuat keputusan', 'I', 'E'),
			(24, 'Dalam suatu kelompok saya lebih suka diam', 'Saya suka mengerjakan beberapa pekerjaan sekaligus', 'X', 'N'),
			(25, 'Saya sangat suka bila saya diundang', 'Saya ingin lebih baik dari yang lain dalam mengerjakan sesuatu', 'B', 'A'),
			(26, 'Saya suka membina hubungan yang akrab dengan teman-teman saya', 'Saya suka menasehati orang lain', 'O', 'P'),
			(27, 'Saya suka melakukan hal-hal yang baru dan berbeda', 'Saya menceritakan bagaimana saya berhasil melakukan sesuatu', 'Z', 'X'),
			(28, 'Bila saya betul, saya suka mempertahankannya', 'Saya ingin diterima dan diakui dalam suatu kelompok', 'K', 'B'),
			(29, 'Saya berusaha untuk tidak menjadi seorang yang berbeda', 'Saya berusaha untuk sekali-sekali bersama orang lain', 'F', 'O'),
			(30, 'Saya senang diberi tahu bagaimana melakukan suatu pekerjaan', 'Saya mudah bosan', 'W', 'Z'),
			(31, 'Saya bekerja keras', 'Saya banyak berfikir dan membuat rencana', 'G', 'R'),
			(32, 'Saya memimpin kelompok', 'Detail (hal-hal kecil) menarik bagi saya', 'L', 'D'),
			(33, 'Saya mengambil keputusan secara mudah dan cepat', 'Saya menyimpan barang-barang saya secara rapih dan teratur', 'I', 'C'),
			(34, 'Biasanya saya bekerja dengan tergesa-gesa', 'Saya jarang marah atau bersedih', 'T', 'E'),
			(35, 'Saya ingin menjadi bagian dari kelompok', 'Saya ingin melakukan satu pekerjaan pada satu saat', 'B', 'N'),
			(36, 'Saya berusaha berteman secara akrab', 'Saya berusaha sangat keras untuk menjadi yang terbaik', 'O', 'A'),
			(37, 'Saya ingin menjadi bagian dari suatu kelompok', 'Saya berusaha menjadi yang terbaik', 'Z', 'P'),
			(38, 'Saya menyukai perdebatan', 'Saya suka mendapat perhatian', 'K', 'X'),
			(39, 'Saya suka mendukung orang-orang yang menjadi atasan saya', 'Saya tertarik menjadi bagian dari kelompok', 'F', 'B'),
			(40, 'Saya suka mengikuti peraturan dengan hati-hati', 'Saya suka orang mengenal saya dengan baik', 'W', 'O'),
			(41, 'Saya berusaha keras sekali', 'Saya sangat ramah', 'G', 'S'),
			(42, 'Orang menilai saya seorang pemimpin yang baik', 'Saya berfikir panjang dan hati-hati', 'L', 'R'),
			(43, 'Saya sering mengambil resiko/coba-coba', 'Saya sering mengurus hal-hal kecil/detail', 'I', 'D'),
			(44, 'Orang berpendapat bahwa saya bekerja cepat', 'Saya sering mengurus hal-hal kecil/detail', 'T', 'C'),
			(45, 'Saya senang mengikuti pertandingan dan olah raga', 'Saya mempunyai pribadi yang menyenangkan', 'V', 'E'),
			(46, 'Saya senang jika orang dekat', 'Saya mempunyai pribadi yang menyenangkan', 'O', 'N'),
			(47, 'Saya senang bereksperimen dan mencoba hal-hal baru', 'Saya suka melaksanakan suatu pekerjaan sulit dengan baik', 'O', 'A'),
			(48, 'Saya suka diperlakukan secara adil', 'Saya suka memberi tahu orang lain bagaimana melaksanakan suatu pekerjaan', 'Z', 'P'),
			(49, 'Saya suka melakukan apa yang diharapkan oleh saya', 'Saya suka memperoleh perhatian', 'K', 'X'),
			(50, 'Saya suka petunjuk-petunjuk terperinci untuk melaksanakan suatu tugas', 'Saya senang berada bersama orang lain', 'W', 'B'),
			(51, 'Saya selalu berusaha menyelesaikan pekerjaan secara sempurna', 'Orang mengatakan bahwa saya tidak mengenal lelah', 'G', 'V'),
			(52, 'Saya tipe pemimpin', 'Saya mudah berteman', 'L', 'S'),
			(53, 'Saya selalu berspekulasi', 'Saya banyak sekali berfikir', 'I', 'R'),
			(54, 'Saya bekerja dengan kecepatan teratur dan tetap', 'Saya senang bekerja dengan hal-hal kecil/terperinci', 'T', 'D'),
			(55, 'Saya bersemangat untuk mengikuti berbagai pertandingan dalam olah raga', 'Saya mengatur dan menyimpan barang-barang saya secara rapi dan teratur', 'V', 'C'),
			(56, 'Saya dapat bergaul secara baik dengan semua orang', 'Saya adalah seorang yang mempunyai pembawaan tenang', 'S', 'E'),
			(57, 'Saya ingin bertemu dengan orang-orang baru dan melakukan hal-hal baru', 'Saya selalu ingin menyelesaikan pekerjaan yang telah saya mulai', 'Z', 'N'),
			(58, 'Saya biasanya mempertahankan pendapat yang saya yakini', 'Saya biasanya suka bekerja keras', 'K', 'A'),
			(59, 'Saya suka saran-saran dari orang yang saya kagumi', 'Saya senang diserahi tanggung jawab atas kelompok orang', 'F', 'P'),
			(60, 'Saya biarkan diri saya banyak dipengaruhi orang lain', 'Saya suka jika mendapat banyak perhatian', 'W', 'X'),
			(61, 'Saya berusaha bekerja keras', 'Saya mengerjakan sesuatu secara cepat', 'G', 'C'),
			(62, 'Apabila saya bicara, kelompok mendengarkan', 'Saya terampil menggunakan perkakas atau alat-alat', 'L', 'V'),
			(63, 'Saya lambat dalam membina hubungan', 'Saya lambat dalam mengambil keputusan', 'I', 'S'),
			(64, 'Saya biasanya makan secara cepat', 'Saya suka membaca', 'T', 'R'),
			(65, 'Saya suka pekerjaan dimana saya banyak bergerak', 'Saya suka pekerjaan yang harus dilakukan secara hati-hati', 'V', 'D'),
			(66, 'Saya mencari teman sebanyak mungkin', 'Apa yang sudah saya simpan, akan mudah saya temukan kembali', 'S', 'C'),
			(67, 'Saya merencanakan jauh-jauh hari berikutnya', 'Saya selalu menyenangkan', 'R', 'E'),
			(68, 'Saya mempertahankan nama baik saya dengan bangga', 'Saya terus menekuni suatu masalah sampai selesai', 'K', 'N'),
			(69, 'Saya suka mendukung orang-orang yang saya kagumi', 'Saya ingin sukses', 'F', 'A'),
			(70, 'Saya suka orang lain yang memutuskan untuk kelompok', 'Saya suka membuat keputusan untuk kelompok', 'W', 'P'),
			(71, 'Saya selalu berusaha bekerja keras', 'Saya mengambil keputusan secara cepat dan mudah', 'G', 'I'),
			(72, 'Kelompok biasanya melakukan apa yang saya inginkan', 'Saya biasa terburu-buru', 'L', 'T'),
			(73, 'Saya sering merasa lelah', 'Saya lamban dalam menentukan sikap', 'I', 'V'),
			(74, 'Saya bekerja cepat', 'Saya mudah berteman', 'T', 'S'),
			(75, 'Saya biasanya mempunyai semangat dan tenaga', 'Saya banyak menghabiskan waktu dengan berfikir', 'V', 'R'),
			(76, 'Saya sangat ramah terhadap orang', 'Saya suka pekerjaan yang memerlukan ketelitian', 'S', 'D'),
			(77, 'Saya banyak berfikir dan merencanakan', 'Saya menyimpan segala sesuatu pada tempatnya', 'R', 'C'),
			(78, 'Saya suka pekerjaan yang harus memperhatikan hal-hal kecil (detail)', 'Saya tidak mudah marah', 'D', 'E'),
			(79, 'Saya suka mengikuti orang yang saya kagumi', 'Saya selalu menyelesaikan pekerjaan yang telah saya mulai', 'V', 'N'),
			(80, 'Saya suka petunjuk-petunjuk yang jelas', 'Saya suka bekerja keras', 'W', 'A'),
			(81, 'Saya mengejar apa yang saya inginkan', 'Saya seorang pemimpin yang baik', 'G', 'L'),
			(82, 'Saya dapat membuat orang lain bekerja sesuai dengan yang saya inginkan', 'Saya seorang yang tergolong santai dan beruntung', 'L', 'I'),
			(83, 'Saya mengambil keputusan secara mudah dan amat cepat', 'Saya bicara secara cepat', 'I', 'T'),
			(84, 'Saya biasanya bekerja cepat', 'Saya pemimpin yang baik', 'T', 'V'),
			(85, 'Saya tidak suka bertemu dengan orang', 'Saya cepat merasa lelah', 'V', 'S'),
			(86, 'Saya mempunyai banyak sekali teman', 'Saya banyak menghabiskan waktu dengan berfikir', 'S', 'R'),
			(87, 'Saya suka berjalan dengan teori', 'Saya suka bekerja dengan hal-hal yang terperinci', 'R', 'D'),
			(88, 'Saya menikmati pekerjaan yang melibatkan hal-hal kecil (detail)', 'Saya suka mengorganisir pekerjaan saya', 'D', 'C'),
			(89, 'Saya menaruh barang pada tempatnya', 'Saya selalu menyenangkan', 'C', 'E'),
			(90, 'Saya suka diberitahu tentang apa yang perlu dilakukan', 'Saya harus menyelesaikan apa yang saya mulai', 'W', 'N');
		`).Exec()
		if err != nil {
			return err
		}
	}
	return nil
}

