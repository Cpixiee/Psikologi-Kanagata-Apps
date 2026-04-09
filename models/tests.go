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
	EnableLearningStyle bool  `orm:"column(enable_learning_style);default(false)" json:"enable_learning_style"`
	EnableKraepelin bool      `orm:"column(enable_kraepelin);default(false)" json:"enable_kraepelin"`
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
	BatchId    *int      `orm:"null" json:"batch_id,omitempty"` // Bisa NULL jika batch sudah dihapus
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
