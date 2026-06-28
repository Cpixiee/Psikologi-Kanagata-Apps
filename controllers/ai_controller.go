package controllers

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/beego/beego/v2/client/orm"
	beego "github.com/beego/beego/v2/server/web"
)

// AIController membungkus integrasi dengan Google Gemini untuk:
//   - Ringkasan hasil tes psikologi (per alat tes).
//   - Chat bebas pada halaman /ai.
type AIController struct {
	beego.Controller
}

// Sengaja didefinisikan ulang di sini agar tidak bentrok dengan tipe controller lain.
type aiResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

type aiSummaryRequest struct {
	TestType string                 `json:"test_type"`
	Title    string                 `json:"title"`
	Result   map[string]interface{} `json:"result"`
}

type aiChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type aiChatRequest struct {
	Messages []aiChatMessage `json:"messages"`
}

const (
	aiveneModel      = "gpt-chat-latest"
	defaultAiveneKey = "isk-ZPadRJRiEkGNIN2YqflqgFiaazWaHGxSNjyKNKbI"
)

func aiveneAPIKey() string {
	if v := strings.TrimSpace(os.Getenv("AIVENE_API_KEY")); v != "" {
		return v
	}
	if v, _ := beego.AppConfig.String("AIVENE_API_KEY"); strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	if v, _ := beego.AppConfig.String("GEMINI_API_KEY"); strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return defaultAiveneKey
}

func (c *AIController) requireAuth() bool {
	if c.GetSession("user_id") == nil {
		c.Ctx.Output.SetStatus(401)
		c.Data["json"] = aiResponse{Success: false, Message: "Silakan login terlebih dahulu"}
		c.ServeJSON()
		return false
	}
	return true
}

// callGemini dipertahankan namanya agar tidak memecah pemanggil lain,
// tetapi secara internal diubah untuk menggunakan API Aivene (OpenAI-compatible).
func callGemini(systemHint string, userPrompt string, expectJSON bool) (string, int, error) {
	apiKey := aiveneAPIKey()
	if apiKey == "" {
		return "", 500, fmt.Errorf("AIVENE_API_KEY belum dikonfigurasi")
	}

	type chatMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}

	messages := []chatMessage{}
	if strings.TrimSpace(systemHint) != "" {
		messages = append(messages, chatMessage{Role: "system", Content: systemHint})
	}
	messages = append(messages, chatMessage{Role: "user", Content: userPrompt})

	body := map[string]interface{}{
		"model":    aiveneModel,
		"messages": messages,
	}

	// We omit response_format because Aivene API model alias may not support it,
	// causing a 400 Bad Request error. We rely on prompting instructions instead.
	/*
	if expectJSON {
		body["response_format"] = map[string]string{"type": "json_object"}
	}
	*/

	payload, _ := json.Marshal(body)

	endpoint := "https://api.aivene.com/v1/chat/completions"
	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", 500, fmt.Errorf("gagal membuat request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	client := &http.Client{Timeout: 45 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", 502, fmt.Errorf("gagal menghubungi Aivene: %v", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		fmt.Printf("[AI API ERROR] Status %d: %s\n", resp.StatusCode, string(raw))
		return "", resp.StatusCode, fmt.Errorf("aivene error (%d): %s", resp.StatusCode, string(raw))
	}

	var oai struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(raw, &oai); err != nil {
		return "", 502, fmt.Errorf("respons Aivene tidak valid: %v", err)
	}

	if len(oai.Choices) == 0 {
		return "", 502, fmt.Errorf("respons Aivene kosong")
	}

	text := strings.TrimSpace(oai.Choices[0].Message.Content)
	// Strip markdown fences kalau ada.
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	return strings.TrimSpace(text), 200, nil
}

// TestSummary menerima { test_type, title, result } lalu meminta Gemini
// menghasilkan analisis terstruktur (ringkasan, kekuatan, karir, dst).
func (c *AIController) TestSummary() {
	if !c.requireAuth() {
		return
	}
	var req aiSummaryRequest
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &req); err != nil {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = aiResponse{Success: false, Message: "Payload tidak valid"}
		c.ServeJSON()
		return
	}
	req.TestType = strings.TrimSpace(req.TestType)
	if req.TestType == "" {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = aiResponse{Success: false, Message: "test_type wajib diisi"}
		c.ServeJSON()
		return
	}

	// Caching logic
	cacheKey := getCacheHash(req.Result)
	var cacheFile string
	if cacheKey != "" {
		sanitizedType := strings.ToLower(strings.ReplaceAll(req.TestType, " ", "_"))
		cacheFile = fmt.Sprintf("data/ai_cache/test_v2_%s_%s.json", sanitizedType, cacheKey)
		if fileBytes, err := os.ReadFile(cacheFile); err == nil {
			var cachedData map[string]interface{}
			if err := json.Unmarshal(fileBytes, &cachedData); err == nil {
				c.Data["json"] = aiResponse{Success: true, Data: cachedData}
				c.ServeJSON()
				return
			}
		}
	}

	resultJSON, _ := json.MarshalIndent(req.Result, "", "  ")
	systemHint := "Anda adalah seorang psikolog dan career advisor profesional yang menjawab dalam Bahasa Indonesia secara hangat, ringkas, dan praktis. Jangan pernah memberi diagnosis klinis."
	userPrompt := fmt.Sprintf(`Berdasarkan hasil tes psikologi berikut, buat analisis untuk peserta.

Jenis tes: %s
Judul: %s
Data hasil tes (JSON):
%s

Hasilkan respons HANYA dalam JSON valid (tanpa markdown, tanpa code fence) dengan struktur persis seperti ini:
{
  "summary": "ringkasan 2-4 kalimat tentang gambaran umum peserta",
  "tipe_manusia": "tipe kepribadian / minat dominan dalam 1 frasa pendek",
  "kekuatan": ["3-5 poin singkat kekuatan"],
  "area_pengembangan": ["3-5 poin singkat area pengembangan"],
  "rekomendasi_karir": [
    {"posisi": "...", "alasan": "..."},
    {"posisi": "...", "alasan": "..."},
    {"posisi": "...", "alasan": "..."}
  ],
  "rekomendasi_siswa": ["3-5 poin rekomendasi konkret untuk peserta didik/siswa"],
  "rekomendasi_ortu": ["3-5 poin rekomendasi konkret untuk orang tua"],
  "rekomendasi_bk": ["3-5 poin rekomendasi konkret untuk sekolah/guru BK/konselor"],
  "catatan_penting": "1-2 kalimat reminder/disclaimer"
}`, req.TestType, req.Title, string(resultJSON))

	text, status, err := callGemini(systemHint, userPrompt, true)
	if err != nil {
		c.Ctx.Output.SetStatus(status)
		c.Data["json"] = aiResponse{Success: false, Message: err.Error()}
		c.ServeJSON()
		return
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		// fallback: kirim sebagai summary mentah supaya UI tetap menampilkan sesuatu
		c.Data["json"] = aiResponse{Success: true, Data: map[string]interface{}{"summary": text}}
		c.ServeJSON()
		return
	}

	// Save to cache
	if cacheFile != "" {
		if fileBytes, err := json.Marshal(parsed); err == nil {
			_ = os.WriteFile(cacheFile, fileBytes, 0644)
		}
	}

	c.Data["json"] = aiResponse{Success: true, Data: parsed}
	c.ServeJSON()
}

// GetOrGenerateTestSummaryInternal is a helper called by the ZIP download endpoint to retrieve or build the AI summary on-the-fly.
func GetOrGenerateTestSummaryInternal(o orm.Ormer, testType string, resultData interface{}, studentName string) (map[string]interface{}, error) {
	sanitizedType := strings.ToLower(strings.ReplaceAll(testType, " ", "_"))
	cacheKey := getCacheHash(resultData)
	var cacheFile string
	if cacheKey != "" {
		cacheFile = fmt.Sprintf("data/ai_cache/test_v2_%s_%s.json", sanitizedType, cacheKey)
		if fileBytes, err := os.ReadFile(cacheFile); err == nil {
			var cachedData map[string]interface{}
			if err := json.Unmarshal(fileBytes, &cachedData); err == nil {
				// Verify if v2 fields exist, if so return it
				if _, ok1 := cachedData["rekomendasi_siswa"]; ok1 {
					return cachedData, nil
				}
			}
		}
	}

	resultJSON, _ := json.MarshalIndent(resultData, "", "  ")
	systemHint := "Anda adalah seorang psikolog dan career advisor profesional yang menjawab dalam Bahasa Indonesia secara hangat, ringkas, dan praktis. Jangan pernah memberi diagnosis klinis."
	userPrompt := fmt.Sprintf(`Berdasarkan hasil tes psikologi berikut, buat analisis untuk peserta.

Jenis tes: %s
Judul: %s — %s
Data hasil tes (JSON):
%s

Hasilkan respons HANYA dalam JSON valid (tanpa markdown, tanpa code fence) dengan struktur persis seperti ini:
{
  "summary": "ringkasan 2-4 kalimat tentang gambaran umum peserta",
  "tipe_manusia": "tipe kepribadian / minat dominan dalam 1 frasa pendek",
  "kekuatan": ["3-5 poin singkat kekuatan"],
  "area_pengembangan": ["3-5 poin singkat area pengembangan"],
  "rekomendasi_karir": [
    {"posisi": "...", "alasan": "..."},
    {"posisi": "...", "alasan": "..."},
    {"posisi": "...", "alasan": "..."}
  ],
  "rekomendasi_siswa": ["3-5 poin rekomendasi konkret untuk peserta didik/siswa"],
  "rekomendasi_ortu": ["3-5 poin rekomendasi konkret untuk orang tua"],
  "rekomendasi_bk": ["3-5 poin rekomendasi konkret untuk sekolah/guru BK/konselor"],
  "catatan_penting": "1-2 kalimat reminder/disclaimer"
}`, testType, studentName, testType, string(resultJSON))

	text, _, err := callGemini(systemHint, userPrompt, true)
	if err != nil {
		return nil, fmt.Errorf("gagal menghubungi AI: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		// Fallback formatting
		parsed = map[string]interface{}{
			"summary": text,
			"rekomendasi_siswa": []string{"Tetap konsisten belajar dan kembangkan minat yang dimiliki."},
			"rekomendasi_ortu": []string{"Berikan dukungan moral dan fasilitas belajar yang memadai bagi anak."},
			"rekomendasi_bk": []string{"Bimbing dan dampingi siswa dalam memilih kelanjutan studi atau karir."},
		}
	}

	// Cache the result
	if cacheFile != "" {
		if fileBytes, err := json.Marshal(parsed); err == nil {
			_ = os.WriteFile(cacheFile, fileBytes, 0644)
		}
	}

	return parsed, nil
}

// Chat untuk halaman /ai (chat bebas).
func (c *AIController) Chat() {
	if !c.requireAuth() {
		return
	}
	var req aiChatRequest
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &req); err != nil {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = aiResponse{Success: false, Message: "Payload tidak valid"}
		c.ServeJSON()
		return
	}
	if len(req.Messages) == 0 {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = aiResponse{Success: false, Message: "Pesan kosong"}
		c.ServeJSON()
		return
	}

	systemHint := "Anda adalah asisten psikologi & karir dalam aplikasi Psychee Wellness. Jawab dalam Bahasa Indonesia, hangat, ringkas, dan informatif. Jika pertanyaan keluar dari konteks psikologi/karir/pengembangan diri, tetap jawab sopan tetapi arahkan kembali ke konteks tersebut. Jangan memberi diagnosis klinis."

	var b strings.Builder
	b.WriteString("Riwayat percakapan:\n")
	for _, m := range req.Messages {
		role := strings.ToLower(strings.TrimSpace(m.Role))
		if role == "" {
			role = "user"
		}
		label := "Pengguna"
		if role == "assistant" || role == "ai" || role == "model" {
			label = "Asisten"
		}
		b.WriteString(label)
		b.WriteString(": ")
		b.WriteString(strings.TrimSpace(m.Content))
		b.WriteString("\n")
	}
	b.WriteString("\nLanjutkan sebagai Asisten dengan jawaban berikutnya saja (tanpa label \"Asisten:\").")

	text, status, err := callGemini(systemHint, b.String(), false)
	if err != nil {
		c.Ctx.Output.SetStatus(status)
		c.Data["json"] = aiResponse{Success: false, Message: err.Error()}
		c.ServeJSON()
		return
	}
	c.Data["json"] = aiResponse{Success: true, Data: map[string]interface{}{"reply": text}}
	c.ServeJSON()
}

// BatchSummary menerima aggregate results dari seluruh peserta batch,
// lalu meminta Gemini menghasilkan interpretasi level kelas (untuk guru/sekolah).
func (c *AIController) BatchSummary() {
	if !c.requireAuth() {
		return
	}

	var req struct {
		TestType   string                 `json:"test_type"`
		BatchName  string                 `json:"batch_name"`
		Kelas      string                 `json:"kelas"`
		Jurusan    string                 `json:"jurusan"`
		TotalPeserta int                  `json:"total_peserta"`
		TotalSelesai int                  `json:"total_selesai"`
		AggregateData map[string]interface{} `json:"aggregate_data"`
	}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &req); err != nil {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = aiResponse{Success: false, Message: "Payload tidak valid"}
		c.ServeJSON()
		return
	}
	if req.TestType == "" {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = aiResponse{Success: false, Message: "test_type wajib diisi"}
		c.ServeJSON()
		return
	}

	// Caching logic
	cacheKey := getCacheHash(req)
	var cacheFile string
	if cacheKey != "" {
		cacheFile = fmt.Sprintf("data/ai_cache/batch_%s.json", cacheKey)
		if fileBytes, err := os.ReadFile(cacheFile); err == nil {
			var cachedData map[string]interface{}
			if err := json.Unmarshal(fileBytes, &cachedData); err == nil {
				c.Data["json"] = aiResponse{Success: true, Data: cachedData}
				c.ServeJSON()
				return
			}
		}
	}

	aggJSON, _ := json.MarshalIndent(req.AggregateData, "", "  ")
	systemHint := "Anda adalah seorang psikolog pendidikan dan career advisor profesional yang memberikan analisis kelompok/kelas untuk guru dan konselor BK. Jawab dalam Bahasa Indonesia yang profesional, hangat, dan praktis. Jangan pernah memberi diagnosis klinis."
	
	var userPrompt string
	if strings.Contains(req.TestType, ",") {
		userPrompt = fmt.Sprintf(`Berikut adalah data agregat hasil tes psikologi untuk satu kelas/batch yang menggunakan beberapa alat tes sekaligus.

Informasi Batch:
- Nama Batch: %s
- Kelas: %s
- Jurusan: %s
- Total Peserta: %d
- Sudah Selesai: %d
- Jenis Tes Aktif: %s

Data Agregat Hasil Tes (JSON):
%s

Sebagai psikolog pendidikan, buat analisis komprehensif level kelas untuk guru/konselor BK.
Hasilkan respons HANYA dalam JSON valid (tanpa markdown, tanpa code fence) dengan struktur persis seperti ini:
{
  "ringkasan_kelas": "2-4 kalimat gambaran umum karakteristik kelas ini berdasarkan hasil tes",
  "pola_dominan": "tipe/pola yang paling banyak muncul di kelas ini dalam 1-2 kalimat",
  "kekuatan_kelas": ["3-5 kekuatan atau potensi yang menonjol dari kelompok ini"],
  "area_perhatian": ["3-5 area yang perlu mendapat perhatian atau intervensi guru/BK"],
  "rekomendasi_pembelajaran": ["3-5 rekomendasi konkret untuk strategi pembelajaran yang sesuai karakteristik kelas ini"],
  "rekomendasi_bk": ["3-5 rekomendasi untuk konseling dan bimbingan karir yang relevan"],
  "catatan_guru": "1-2 kalimat pesan penting untuk guru dalam mengelola kelas ini",
  "kesimpulan_detail": {
     "ist": "Kesimpulan/analisis khusus untuk hasil IST (jika aktif)",
     "holland": "Kesimpulan/analisis khusus untuk hasil Holland (jika aktif)",
     "learning_style": "Kesimpulan/analisis khusus untuk hasil Gaya Belajar/VAK (jika aktif)",
     "kraepelin": "Kesimpulan/analisis khusus untuk hasil Kraepelin (jika aktif)",
     "rmib": "Kesimpulan/analisis khusus untuk hasil RMIB (jika aktif)",
     "papi": "Kesimpulan/analisis khusus untuk hasil PAPI-Kostick (jika aktif)"
  },
  "kesimpulan_gabungan": "1 paragraf kesimpulan integratif menggabungkan seluruh alat tes untuk batch kelas ini"
}`,
			req.BatchName, req.Kelas, req.Jurusan,
			req.TotalPeserta, req.TotalSelesai, req.TestType,
			string(aggJSON))
	} else {
		userPrompt = fmt.Sprintf(`Berikut adalah data agregat hasil tes psikologi untuk satu kelas/batch.

Informasi Batch:
- Nama Batch: %s
- Kelas: %s
- Jurusan: %s
- Total Peserta: %d
- Sudah Selesai: %d
- Jenis Tes: %s

Data Agregat Hasil Tes (JSON):
%s

Sebagai psikolog pendidikan, buat analisis komprehensif level kelas untuk guru/konselor BK.
Hasilkan respons HANYA dalam JSON valid (tanpa markdown, tanpa code fence) dengan struktur persis seperti ini:
{
  "ringkasan_kelas": "2-4 kalimat gambaran umum karakteristik kelas ini berdasarkan hasil tes",
  "pola_dominan": "tipe/pola yang paling banyak muncul di kelas ini dalam 1-2 kalimat",
  "kekuatan_kelas": ["3-5 kekuatan atau potensi yang menonjol dari kelompok ini"],
  "area_perhatian": ["3-5 area yang perlu mendapat perhatian atau intervensi guru/BK"],
  "rekomendasi_pembelajaran": ["3-5 rekomendasi konkret untuk strategi pembelajaran yang sesuai karakteristik kelas ini"],
  "rekomendasi_bk": ["3-5 rekomendasi untuk konseling dan bimbingan karir yang relevan"],
  "catatan_guru": "1-2 kalimat pesan penting untuk guru dalam mengelola kelas ini",
  "kesimpulan_detail": {
     "%s": "Kesimpulan/analisis khusus untuk hasil alat tes ini"
  },
  "kesimpulan_gabungan": "1 paragraf kesimpulan integratif untuk hasil tes batch kelas ini"
}`,
			req.BatchName, req.Kelas, req.Jurusan,
			req.TotalPeserta, req.TotalSelesai, req.TestType,
			string(aggJSON), req.TestType)
	}

	text, status, err := callGemini(systemHint, userPrompt, true)
	if err != nil {
		c.Ctx.Output.SetStatus(status)
		c.Data["json"] = aiResponse{Success: false, Message: err.Error()}
		c.ServeJSON()
		return
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		c.Data["json"] = aiResponse{Success: true, Data: map[string]interface{}{"ringkasan_kelas": text}}
		c.ServeJSON()
		return
	}

	// Save to cache
	if cacheFile != "" {
		if fileBytes, err := json.Marshal(parsed); err == nil {
			_ = os.WriteFile(cacheFile, fileBytes, 0644)
		}
	}

	c.Data["json"] = aiResponse{Success: true, Data: parsed}
	c.ServeJSON()
}

// StudentCombinedSummary menerima hasil semua tes seorang siswa dalam satu batch,
// lalu meminta Gemini/Aivene menghasilkan interpretasi per alat tes dan kesimpulan gabungan.
func (c *AIController) StudentCombinedSummary() {
	if !c.requireAuth() {
		return
	}

	var req struct {
		StudentName string                 `json:"student_name"`
		BatchName   string                 `json:"batch_name"`
		Results     map[string]interface{} `json:"results"`
	}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &req); err != nil {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = aiResponse{Success: false, Message: "Payload tidak valid"}
		c.ServeJSON()
		return
	}

	// Caching logic
	cacheKey := getCacheHash(req)
	var cacheFile string
	if cacheKey != "" {
		cacheFile = fmt.Sprintf("data/ai_cache/student_combined_v2_%s.json", cacheKey)
		if fileBytes, err := os.ReadFile(cacheFile); err == nil {
			var cachedData map[string]interface{}
			if err := json.Unmarshal(fileBytes, &cachedData); err == nil {
				c.Data["json"] = aiResponse{Success: true, Data: cachedData}
				c.ServeJSON()
				return
			}
		}
	}

	resultsJSON, _ := json.MarshalIndent(req.Results, "", "  ")
	systemHint := "Anda adalah seorang psikolog dan career advisor profesional yang memberikan analisis kepribadian, gaya belajar, dan karir terintegrasi untuk seorang siswa berdasarkan hasil beberapa alat tes psikologi. Jawab dalam Bahasa Indonesia secara hangat, bersahabat, dan praktis. Jangan pernah memberikan diagnosis klinis."
	userPrompt := fmt.Sprintf(`Berikut adalah data hasil beberapa alat tes psikologi untuk seorang siswa.
 
Nama Siswa: %s
Nama Batch: %s
 
Hasil Alat Tes (JSON):
%s
 
Sebagai psikolog dan advisor, buatlah analisis komprehensif yang mendalam untuk siswa ini.
Hasilkan respons HANYA dalam JSON valid (tanpa markdown, tanpa code fence) dengan struktur persis seperti ini:
{
  "kesimpulan_detail": {
     "ist": "Kesimpulan singkat khusus hasil IST (jika datanya ada)...",
     "holland": "Kesimpulan singkat khusus hasil Holland RIASEC (jika datanya ada)...",
     "learning_style": "Kesimpulan singkat khusus hasil Gaya Belajar/VAK (jika datanya ada)...",
     "kraepelin": "Kesimpulan singkat khusus hasil Kraepelin (jika datanya ada)...",
     "rmib": "Kesimpulan singkat khusus hasil RMIB (jika datanya ada)...",
     "papi": "Kesimpulan singkat khusus hasil PAPI-Kostick (jika datanya ada)..."
  },
  "kesimpulan_gabungan": "Kesimpulan integratif gabungan seluruh aspek bakat, minat, kepribadian, gaya belajar, dan stabilitas emosi siswa.",
  "strengths": ["3-5 poin singkat kekuatan menonjol siswa"],
  "developments": ["3-5 poin singkat area pengembangan diri siswa"],
  "recommendations": [
    {
      "color": "violet",
      "icon": "graduation-cap",
      "title": "Academic Recommendation",
      "items": ["2-3 rekomendasi akademik konkret sesuai bakat/minat"]
    },
    {
      "color": "blue",
      "icon": "code-2",
      "title": "Skill Recommendation",
      "items": ["2-3 rekomendasi keahlian teknis/soft-skill konkret yang perlu dipelajari"]
    },
    {
      "color": "pink",
      "icon": "rocket",
      "title": "Activity Recommendation",
      "items": ["2-3 rekomendasi kegiatan/ekstrakurikuler/lomba/kursus yang disarankan"]
    }
  ],
  "potential": 85,
  "potential_desc": "1-2 kalimat deskripsi singkat potensi tinggi siswa",
  "insight": "1 paragraf insight utama untuk ditaruh di AI Insight Smart Summary",
  "emotional_analytics": {
    "selfAwareness": 75,
    "selfRegulation": 70,
    "motivation": 80,
    "empathy": 65,
    "stressManagement": 72,
    "resilience": 78
  },
  "skill_tracker": [
    {"name": "Analytical Thinking", "value": 85},
    {"name": "Problem Solving", "value": 80},
    {"name": "Communication", "value": 70},
    {"name": "Leadership", "value": 65},
    {"name": "Creativity", "value": 75},
    {"name": "Technical Skill", "value": 80}
  ],
  "career_roadmap": {
    "careers": [
      {"name": "Nama Karir 1", "match": 90, "icon": "briefcase"},
      {"name": "Nama Karir 2", "match": 85, "icon": "bar-chart-2"},
      {"name": "Nama Karir 3", "match": 80, "icon": "code-2"},
      {"name": "Nama Karir 4", "match": 75, "icon": "users"},
      {"name": "Nama Karir 5", "match": 70, "icon": "rocket"}
    ],
    "roadmap": [
      {"term": "Rekomendasi Jurusan (Major Matches)", "items": ["rekomendasi jurusan/program studi 1 yang sangat cocok", "rekomendasi jurusan 2"]},
      {"term": "Mata Pelajaran Pendukung (Subject Matches)", "items": ["mata pelajaran pendukung 1 yang relevan", "mata pelajaran 2"]}
    ]
  }
}

PENTING:
1. Di dalam objek 'kesimpulan_detail', HANYA sertakan kunci untuk alat tes yang datanya ada di input. Jika tidak ada, JANGAN sertakan kunci tersebut.
2. Semua nilai numerik harus integer 0-100.
3. 'emotional_analytics' harus mencerminkan kondisi emosi dan kepribadian siswa berdasarkan data tes yang tersedia.
4. 'skill_tracker' harus mencerminkan kemampuan spesifik siswa berdasarkan tes yang tersedia (bukan generik).
5. 'career_roadmap.careers' harus berisi karir yang BENAR-BENAR cocok dengan profil siswa, bukan template umum.`, req.StudentName, req.BatchName, string(resultsJSON))

	text, status, err := callGemini(systemHint, userPrompt, true)
	if err != nil {
		c.Ctx.Output.SetStatus(status)
		c.Data["json"] = aiResponse{Success: false, Message: err.Error()}
		c.ServeJSON()
		return
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		c.Data["json"] = aiResponse{Success: true, Data: map[string]interface{}{"kesimpulan_gabungan": text}}
		c.ServeJSON()
		return
	}

	// Filter out uncompleted tests or placeholder messages from kesimpulan_detail
	if details, ok := parsed["kesimpulan_detail"].(map[string]interface{}); ok {
		for key := range details {
			if _, exists := req.Results[key]; !exists {
				delete(details, key)
			} else {
				if valStr, ok := details[key].(string); ok {
					lowerVal := strings.ToLower(valStr)
					if strings.Contains(lowerVal, "tidak tersedia") || strings.Contains(lowerVal, "belum dapat dianalisis") {
						delete(details, key)
					}
				}
			}
		}
	}

	// Save to cache
	if cacheFile != "" {
		if fileBytes, err := json.Marshal(parsed); err == nil {
			_ = os.WriteFile(cacheFile, fileBytes, 0644)
		}
	}

	c.Data["json"] = aiResponse{Success: true, Data: parsed}
	c.ServeJSON()
}

// GetOrGenerateCombinedSummaryInternal is a helper called by the ZIP download endpoint to retrieve or build the combined AI summary on-the-fly.
func GetOrGenerateCombinedSummaryInternal(studentName string, batchName string, results map[string]interface{}) (map[string]interface{}, error) {
	req := struct {
		StudentName string                 `json:"student_name"`
		BatchName   string                 `json:"batch_name"`
		Results     map[string]interface{} `json:"results"`
	}{
		StudentName: studentName,
		BatchName:   batchName,
		Results:     results,
	}

	cacheKey := getCacheHash(req)
	var cacheFile string
	if cacheKey != "" {
		cacheFile = fmt.Sprintf("data/ai_cache/student_combined_v2_%s.json", cacheKey)
		if fileBytes, err := os.ReadFile(cacheFile); err == nil {
			var cachedData map[string]interface{}
			if err := json.Unmarshal(fileBytes, &cachedData); err == nil {
				return cachedData, nil
			}
		}
	}

	resultsJSON, _ := json.MarshalIndent(results, "", "  ")
	systemHint := "Anda adalah seorang psikolog dan career advisor profesional yang memberikan analisis kepribadian, gaya belajar, dan karir terintegrasi untuk seorang siswa berdasarkan hasil beberapa alat tes psikologi. Jawab dalam Bahasa Indonesia secara hangat, bersahabat, dan praktis. Jangan pernah memberikan diagnosis klinis."
	userPrompt := fmt.Sprintf(`Berikut adalah data hasil beberapa alat tes psikologi untuk seorang siswa.
 
Nama Siswa: %s
Nama Batch: %s
 
Hasil Alat Tes (JSON):
%s
 
Sebagai psikolog dan advisor, buatlah analisis komprehensif yang mendalam untuk siswa ini.
Hasilkan respons HANYA dalam JSON valid (tanpa markdown, tanpa code fence) dengan struktur persis seperti ini:
{
  "kesimpulan_detail": {
     "ist": "Kesimpulan singkat khusus hasil IST (jika datanya ada)...",
     "holland": "Kesimpulan singkat khusus hasil Holland RIASEC (jika datanya ada)...",
     "learning_style": "Kesimpulan singkat khusus hasil Gaya Belajar/VAK (jika datanya ada)...",
     "kraepelin": "Kesimpulan singkat khusus hasil Kraepelin (jika datanya ada)...",
     "rmib": "Kesimpulan singkat khusus hasil RMIB (jika datanya ada)...",
     "papi": "Kesimpulan singkat khusus hasil PAPI-Kostick (jika datanya ada)..."
  },
  "kesimpulan_gabungan": "Kesimpulan integratif gabungan seluruh aspek bakat, minat, kepribadian, gaya belajar, dan stabilitas emosi siswa.",
  "strengths": ["3-5 poin singkat kekuatan menonjol siswa"],
  "developments": ["3-5 poin singkat area pengembangan diri siswa"],
  "recommendations": [
    {
      "color": "violet",
      "icon": "graduation-cap",
      "title": "Academic Recommendation",
      "items": ["2-3 rekomendasi akademik konkret sesuai bakat/minat"]
    },
    {
      "color": "blue",
      "icon": "code-2",
      "title": "Skill Recommendation",
      "items": ["2-3 rekomendasi keahlian teknis/soft-skill konkret yang perlu dipelajari"]
    },
    {
      "color": "pink",
      "icon": "rocket",
      "title": "Activity Recommendation",
      "items": ["2-3 rekomendasi kegiatan/ekstrakurikuler/lomba/kursus yang disarankan"]
    }
  ],
  "potential": 85,
  "potential_desc": "1-2 kalimat deskripsi singkat potensi tinggi siswa",
  "insight": "1 paragraf insight utama untuk ditaruh di AI Insight Smart Summary",
  "emotional_analytics": {
    "selfAwareness": 75,
    "selfRegulation": 70,
    "motivation": 80,
    "empathy": 65,
    "stressManagement": 72,
    "resilience": 78
  },
  "skill_tracker": [
    {"name": "Analytical Thinking", "value": 85},
    {"name": "Problem Solving", "value": 80},
    {"name": "Communication", "value": 70},
    {"name": "Leadership", "value": 65},
    {"name": "Creativity", "value": 75},
    {"name": "Technical Skill", "value": 80}
  ],
  "career_roadmap": {
    "careers": [
      {"name": "Nama Karir 1", "match": 90, "icon": "briefcase"},
      {"name": "Nama Karir 2", "match": 85, "icon": "bar-chart-2"},
      {"name": "Nama Karir 3", "match": 80, "icon": "code-2"},
      {"name": "Nama Karir 4", "match": 75, "icon": "users"},
      {"name": "Nama Karir 5", "match": 70, "icon": "rocket"}
    ],
    "roadmap": [
      {"term": "Rekomendasi Jurusan (Major Matches)", "items": ["rekomendasi jurusan/program studi 1 yang sangat cocok", "rekomendasi jurusan 2"]},
      {"term": "Mata Pelajaran Pendukung (Subject Matches)", "items": ["mata pelajaran pendukung 1 yang relevan", "mata pelajaran 2"]}
    ]
  }
}

PENTING:
1. Di dalam objek 'kesimpulan_detail', HANYA sertakan kunci untuk alat tes yang datanya ada di input. Jika tidak ada, JANGAN sertakan kunci tersebut.
2. Semua nilai numerik harus integer 0-100.
3. 'emotional_analytics' harus mencerminkan kondisi emosi dan kepribadian siswa berdasarkan data tes yang tersedia.
4. 'skill_tracker' harus mencerminkan kemampuan spesifik siswa berdasarkan tes yang tersedia (bukan generik).
5. 'career_roadmap.careers' harus berisi karir yang BENAR-BENAR cocok dengan profil siswa, bukan template umum.`, studentName, batchName, string(resultsJSON))

	text, _, err := callGemini(systemHint, userPrompt, true)
	if err != nil {
		return nil, fmt.Errorf("gagal menghubungi AI: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		return map[string]interface{}{"kesimpulan_gabungan": text}, nil
	}

	// Filter out uncompleted tests or placeholder messages from kesimpulan_detail
	if details, ok := parsed["kesimpulan_detail"].(map[string]interface{}); ok {
		for key := range details {
			if _, exists := results[key]; !exists {
				delete(details, key)
			} else {
				if valStr, ok := details[key].(string); ok {
					lowerVal := strings.ToLower(valStr)
					if strings.Contains(lowerVal, "tidak tersedia") || strings.Contains(lowerVal, "belum dapat dianalisis") {
						delete(details, key)
					}
				}
			}
		}
	}

	// Save to cache
	if cacheFile != "" {
		if fileBytes, err := json.Marshal(parsed); err == nil {
			_ = os.WriteFile(cacheFile, fileBytes, 0644)
		}
	}

	return parsed, nil
}


func getCacheHash(data interface{}) string {
	b, err := json.Marshal(data)
	if err != nil {
		return ""
	}
	hash := sha256.Sum256(b)
	return hex.EncodeToString(hash[:])
}

func init() {
	_ = os.MkdirAll("data/ai_cache", 0755)
}
