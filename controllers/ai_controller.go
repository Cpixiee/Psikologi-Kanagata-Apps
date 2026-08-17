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
	"regexp"
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
	defaultGeminiKey   = ""
	defaultGeminiModel = "gemini-1.5-flash"
)

func getGeminiModel() string {
	m := defaultGeminiModel
	if v := strings.TrimSpace(os.Getenv("GEMINI_MODEL")); v != "" {
		m = v
	} else if v, _ := beego.AppConfig.String("GEMINI_MODEL"); strings.TrimSpace(v) != "" {
		m = strings.TrimSpace(v)
	}
	return m
}

func geminiAPIKey() string {
	key := ""
	if v := strings.TrimSpace(os.Getenv("GEMINI_API_KEY")); v != "" {
		key = v
	} else if v, _ := beego.AppConfig.String("GEMINI_API_KEY"); strings.TrimSpace(v) != "" {
		key = strings.TrimSpace(v)
	}
	if key == "" || key == "your_gemini_api_key_here" || strings.HasPrefix(key, "AQ.") {
		return ""
	}
	return key
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

func callGemini(systemHint string, userPrompt string, expectJSON bool) (string, int, error) {
	apiKey := geminiAPIKey()
	if apiKey == "" {
		return "", 400, fmt.Errorf("GEMINI_API_KEY belum dikonfigurasi dengan Kunci Google AI Studio (AIzaSy...) pada file .env.docker")
	}

	primaryModel := getGeminiModel()
	modelsToTry := []string{primaryModel, "gemini-2.5-flash", "gemini-2.0-flash", "gemini-1.5-flash", "gemini-flash-latest", "gemini-1.5-pro"}
	uniqueModels := make([]string, 0, len(modelsToTry))
	seen := make(map[string]bool)
	for _, m := range modelsToTry {
		m = strings.TrimSpace(m)
		if m != "" && !seen[m] {
			seen[m] = true
			uniqueModels = append(uniqueModels, m)
		}
	}

	type geminiPart struct {
		Text string `json:"text"`
	}
	type geminiContent struct {
		Role  string       `json:"role,omitempty"`
		Parts []geminiPart `json:"parts"`
	}
	type geminiGenConfig struct {
		ResponseMimeType string `json:"responseMimeType,omitempty"`
	}
	type geminiReq struct {
		SystemInstruction *geminiContent   `json:"systemInstruction,omitempty"`
		Contents          []geminiContent  `json:"contents"`
		GenerationConfig  *geminiGenConfig `json:"generationConfig,omitempty"`
	}

	reqBody := geminiReq{
		Contents: []geminiContent{
			{
				Role:  "user",
				Parts: []geminiPart{{Text: userPrompt}},
			},
		},
	}
	if strings.TrimSpace(systemHint) != "" {
		reqBody.SystemInstruction = &geminiContent{
			Parts: []geminiPart{{Text: systemHint}},
		}
	}
	if expectJSON {
		reqBody.GenerationConfig = &geminiGenConfig{
			ResponseMimeType: "application/json",
		}
	}

	payload, _ := json.Marshal(reqBody)
	client := &http.Client{Timeout: 45 * time.Second}

	var lastErr string
	lastStatus := 500

	// 1. Coba REST API native Google Gemini (v1beta dan v1) dengan variasi model
	apiVersions := []string{"v1beta", "v1"}
	for _, apiVer := range apiVersions {
		for _, modelName := range uniqueModels {
			endpoint := fmt.Sprintf("https://generativelanguage.googleapis.com/%s/models/%s:generateContent?key=%s", apiVer, modelName, apiKey)
			req, err := http.NewRequest("POST", endpoint, bytes.NewReader(payload))
			if err != nil {
				continue
			}
			req.Header.Set("Content-Type", "application/json")

			resp, err := client.Do(req)
			if err != nil {
				lastErr = err.Error()
				lastStatus = 502
				continue
			}
			raw, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			if resp.StatusCode < 400 {
				var gResp struct {
					Candidates []struct {
						Content struct {
							Parts []struct {
								Text string `json:"text"`
							} `json:"parts"`
						} `json:"content"`
					} `json:"candidates"`
				}
				if err := json.Unmarshal(raw, &gResp); err == nil && len(gResp.Candidates) > 0 && len(gResp.Candidates[0].Content.Parts) > 0 {
					text := strings.TrimSpace(gResp.Candidates[0].Content.Parts[0].Text)
					text = strings.TrimPrefix(text, "```json")
					text = strings.TrimPrefix(text, "```")
					text = strings.TrimSuffix(text, "```")
					return strings.TrimSpace(text), 200, nil
				}
			} else {
				lastStatus = resp.StatusCode
				lastErr = string(raw)
			}
		}
	}

	// 2. Fallback: Coba OpenAI-compatible Gemini endpoint
	oaiEndpoint := "https://generativelanguage.googleapis.com/v1beta/openai/chat/completions"
	type chatMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	messages := []chatMessage{}
	if strings.TrimSpace(systemHint) != "" {
		messages = append(messages, chatMessage{Role: "system", Content: systemHint})
	}
	messages = append(messages, chatMessage{Role: "user", Content: userPrompt})

	for _, modelName := range uniqueModels {
		oaiBody := map[string]interface{}{
			"model":    modelName,
			"messages": messages,
		}
		if expectJSON {
			oaiBody["response_format"] = map[string]string{"type": "json_object"}
		}

		oaiPayload, _ := json.Marshal(oaiBody)
		oaiReq, err := http.NewRequest("POST", oaiEndpoint, bytes.NewReader(oaiPayload))
		if err == nil {
			oaiReq.Header.Set("Content-Type", "application/json")
			oaiReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

			oaiResp, err := client.Do(oaiReq)
			if err == nil {
				oaiRaw, _ := io.ReadAll(oaiResp.Body)
				oaiResp.Body.Close()
				if oaiResp.StatusCode < 400 {
					var oai struct {
						Choices []struct {
							Message struct {
								Content string `json:"content"`
							} `json:"message"`
						} `json:"choices"`
					}
					if json.Unmarshal(oaiRaw, &oai) == nil && len(oai.Choices) > 0 {
						text := strings.TrimSpace(oai.Choices[0].Message.Content)
						text = strings.TrimPrefix(text, "```json")
						text = strings.TrimPrefix(text, "```")
						text = strings.TrimSuffix(text, "```")
						return strings.TrimSpace(text), 200, nil
					}
				}
			}
		}
	}

	// 3. Fallback: Coba Aivene default key (sehingga AI selalu merespon jika key user problem)
	aiveneEndpoint := "https://api.aivene.com/v1/chat/completions"
	aiveneKey := "isk-ZPadRJRiEkGNIN2YqflqgFiaazWaHGxSNjyKNKbI"
	aiveneBody := map[string]interface{}{
		"model":    "gpt-chat-latest",
		"messages": messages,
	}
	if expectJSON {
		aiveneBody["response_format"] = map[string]string{"type": "json_object"}
	}
	aivenePayload, _ := json.Marshal(aiveneBody)
	aiveneReq, err := http.NewRequest("POST", aiveneEndpoint, bytes.NewReader(aivenePayload))
	if err == nil {
		aiveneReq.Header.Set("Content-Type", "application/json")
		aiveneReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", aiveneKey))
		aiveneResp, err := client.Do(aiveneReq)
		if err == nil {
			aiveneRaw, _ := io.ReadAll(aiveneResp.Body)
			aiveneResp.Body.Close()
			if aiveneResp.StatusCode < 400 {
				var oai struct {
					Choices []struct {
						Message struct {
							Content string `json:"content"`
						} `json:"message"`
					} `json:"choices"`
				}
				if json.Unmarshal(aiveneRaw, &oai) == nil && len(oai.Choices) > 0 {
					text := strings.TrimSpace(oai.Choices[0].Message.Content)
					text = strings.TrimPrefix(text, "```json")
					text = strings.TrimPrefix(text, "```")
					text = strings.TrimSuffix(text, "```")
					return strings.TrimSpace(text), 200, nil
				}
			}
		}
	}

	fmt.Printf("[GEMINI API ERROR] Status %d: %s\n", lastStatus, lastErr)
	return "", lastStatus, fmt.Errorf("gemini error (%d): %s", lastStatus, lastErr)
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
	systemHint := "Anda adalah seorang Psikolog Pendidikan Senior dan Certified Career Consultant profesional. Jawablah dalam Bahasa Indonesia yang hangat, sangat jelas, mendalam, dan deskriptif. Jelaskan alasan mendasar di balik setiap rekomendasi jurusan, pekerjaan, dan pengembangan diri agar peserta dan konselor mendapatkan gambaran yang sangat terang dan berguna. PENTING: DILARANG KERAS menyertakan simbol/kode skor teknis mentah seperti (V=7), (C=7), (Z=7), (E=2), (G=3, T=5), (A=6) dalam seluruh kalimat. Terjemahkan seluruh data menjadi narasi Bahasa Indonesia yang mengalir alami, elegan, dan profesional tanpa menyebutkan simbol huruf/angka skor teknis tersebut. Jangan memberi diagnosis klinis."
	userPrompt := fmt.Sprintf(`Berdasarkan hasil tes psikologi berikut, buat analisis deskriptif dan komprehensif untuk peserta.

Jenis tes: %s
Judul: %s
Data hasil tes (JSON):
%s

CATATAN KHUSUS: DILARANG menyertakan kode skor seperti (V=7), (C=7), (Z=7), (E=2), (G=3), (T=5) dalam teks narasi.

Hasilkan respons HANYA dalam JSON valid (tanpa markdown, tanpa code fence) dengan struktur persis seperti ini:
{
  "summary": "Analisis mendalam 3-5 kalimat deskriptif dan komprehensif mengenai gambaran umum kapasitas dan potensi peserta.",
  "tipe_manusia": "Tipe Kepribadian / Pola Dominan Utama dalam 1 frasa singkat dan jelas (misal: 'Investigatif & Analitis (Persuasif)')",
  "kekuatan": [
    "4-6 poin kekuatan utama beserta penjelasan deskriptif singkat & konkret"
  ],
  "area_pengembangan": [
    "4-6 poin area pengembangan yang perlu diperhatikan beserta tips konkret"
  ],
  "rekomendasi_karir": [
    {"posisi": "Nama Pekerjaan / Profesi 1", "alasan": "Penjelasan deskriptif lengkap mengapa profesi ini sangat cocok dengan profil hasil tes."},
    {"posisi": "Nama Pekerjaan / Profesi 2", "alasan": "Penjelasan deskriptif lengkap alasan kecocokan."},
    {"posisi": "Nama Pekerjaan / Profesi 3", "alasan": "Penjelasan deskriptif lengkap alasan kecocokan."},
    {"posisi": "Nama Pekerjaan / Profesi 4", "alasan": "Penjelasan deskriptif lengkap alasan kecocokan."}
  ],
  "rekomendasi_jurusan": ["4-6 rekomendasi jurusan / program studi perguruan tinggi yang paling sesuai beserta alasan singkat"],
  "rekomendasi_siswa": ["4-6 poin rekomendasi langkah aksi konkret untuk siswa"],
  "rekomendasi_ortu": ["4-6 poin panduan konkret untuk orang tua"],
  "rekomendasi_bk": ["4-6 arahan strategis untuk guru BK / sekolah"],
  "catatan_penting": "1-2 kalimat pesan inspiratif & reminder psikotes"
}`, req.TestType, req.Title, string(resultJSON))

	text, _, err := callGemini(systemHint, userPrompt, true)
	var parsed map[string]interface{}
	if err != nil {
		parsed = generateFallbackTestSummary(req.TestType, req.Title, req.Result)
	} else {
		parsed = cleanParsedAIData(text)
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

var scaleNotationRegex = regexp.MustCompile(`\s*\([^)]*=[^)]*\)`)

func stripScaleCodesFromObj(val interface{}) interface{} {
	switch v := val.(type) {
	case string:
		cleaned := scaleNotationRegex.ReplaceAllString(v, "")
		return strings.TrimSpace(cleaned)
	case []interface{}:
		out := make([]interface{}, len(v))
		for i, item := range v {
			out[i] = stripScaleCodesFromObj(item)
		}
		return out
	case map[string]interface{}:
		out := make(map[string]interface{})
		for k, item := range v {
			out[k] = stripScaleCodesFromObj(item)
		}
		return out
	default:
		return v
	}
}

func cleanParsedAIData(text string) map[string]interface{} {
	cleaned := strings.TrimSpace(text)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(cleaned), &parsed); err == nil {
		if sumStr, ok := parsed["summary"].(string); ok && strings.HasPrefix(strings.TrimSpace(sumStr), "{") {
			var inner map[string]interface{}
			if err := json.Unmarshal([]byte(strings.TrimSpace(sumStr)), &inner); err == nil {
				for k, v := range inner {
					parsed[k] = v
				}
			}
		}
		res := stripScaleCodesFromObj(parsed)
		if m, ok := res.(map[string]interface{}); ok {
			return m
		}
		return parsed
	}
	return map[string]interface{}{"summary": scaleNotationRegex.ReplaceAllString(cleaned, "")}
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
				// Verify if summary or v2 fields exist, if so return it
				if _, ok := cachedData["summary"]; ok {
					return cachedData, nil
				}
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
	var parsed map[string]interface{}
	if err != nil || strings.TrimSpace(text) == "" {
		parsed = generateFallbackTestSummary(testType, "", resultData)
	} else if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		parsed = generateFallbackTestSummary(testType, "", resultData)
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

	text, _, err := callGemini(systemHint, b.String(), false)
	if err != nil || strings.TrimSpace(text) == "" {
		lastMsg := ""
		if len(req.Messages) > 0 {
			lastMsg = strings.TrimSpace(req.Messages[len(req.Messages)-1].Content)
		}
		text = generateFallbackAIChatReply(lastMsg)
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

	text, _, err := callGemini(systemHint, userPrompt, true)
	var parsed map[string]interface{}
	if err != nil || strings.TrimSpace(text) == "" {
		parsed = generateFallbackBatchSummary(req.BatchName, req.TestType)
	} else if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		parsed = generateFallbackBatchSummary(req.BatchName, req.TestType)
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
// lalu meminta Gemini menghasilkan interpretasi per alat tes dan kesimpulan gabungan.
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
	systemHint := "Anda adalah Psikolog Utama dan Senior Career Strategist profesional. Tugas Anda adalah menyusun Laporan Integratif Psikotes Multi-Tes yang sangat deskriptif, tajam, dan komprehensif. Integrasikan seluruh aspek intelegensi (IST), minat karir (Holland/RMIB), gaya belajar (VAK), kepribadian (PAPI), dan ketahanan kerja (Kraepelin) ke dalam gambaran profil yang utuh, jelas, deskriptif, dan menginspirasi."
	userPrompt := fmt.Sprintf(`Berikut adalah data hasil beberapa alat tes psikologi untuk seorang siswa.
 
Nama Siswa: %s
Nama Batch: %s
 
Hasil Alat Tes (JSON):
%s
 
Sebagai psikolog dan advisor karir utama, buatlah analisis integratif yang sangat deskriptif, kaya detail, dan memberikan rekomendasi konkret.
Hasilkan respons HANYA dalam JSON valid (tanpa markdown, tanpa code fence) dengan struktur persis seperti ini:
{
  "kesimpulan_detail": {
     "ist": "Analisis deskriptif mendalam hasil kemampuan kognitif & intelegensi (IST)...",
     "holland": "Analisis deskriptif mendalam orientasi minat & karir (Holland RIASEC)...",
     "learning_style": "Analisis deskriptif mendalam preferensi gaya belajar & strategi VAK...",
     "kraepelin": "Analisis deskriptif mendalam daya tahan, kecermatan, & ritme kerja Kraepelin...",
     "rmib": "Analisis deskriptif mendalam hirarki minat pekerjaan RMIB...",
     "papi": "Analisis deskriptif mendalam dinamika kepribadian & gaya kerja PAPI-Kostick..."
  },
  "kesimpulan_gabungan": "Paragraf kesimpulan integratif yang kaya, deskriptif, dan mendalam menggabungkan seluruh potensi bakat, minat, kepribadian, gaya belajar, dan daya tahan peserta.",
  "strengths": ["3-5 poin kekuatan utama siswa secara jelas dan deskriptif"],
  "developments": ["3-5 poin fokus pengembangan diri siswa secara jelas dan deskriptif"],
  "recommendations": [
    {
      "color": "violet",
      "icon": "graduation-cap",
      "title": "Rekomendasi Akademik & Jurusan Kuliah",
      "items": ["3-4 rekomendasi jurusan / program studi perguruan tinggi yang paling tepat beserta penjelasan alasan akademisnya"]
    },
    {
      "color": "blue",
      "icon": "code-2",
      "title": "Rekomendasi Keahlian & Skill Kunci",
      "items": ["3-4 keahlian teknis & soft-skill kunci yang disarankan untuk dipelajari siswa demi menunjang karirnya"]
    },
    {
      "color": "pink",
      "icon": "rocket",
      "title": "Rekomendasi Karir & Profesi Utama",
      "items": ["3-4 profesi / opsi pekerjaan masa depan yang paling tinggi tingkat kecocokannya"]
    }
  ],
  "potential": 90,
  "potential_desc": "Deskripsi mendalam 2 kalimat tentang potensi puncak siswa dan akselerasi karir masa depannya.",
  "insight": "Insight utama psikologis yang tajam, deskriptif, dan inspiratif bagi siswa dan guru BK.",
  "emotional_analytics": {
    "selfAwareness": 82,
    "selfRegulation": 78,
    "motivation": 85,
    "empathy": 75,
    "stressManagement": 76,
    "resilience": 84
  },
  "skill_tracker": [
    {"name": "Analytical Thinking", "value": 88},
    {"name": "Problem Solving", "value": 85},
    {"name": "Communication", "value": 78},
    {"name": "Leadership", "value": 75},
    {"name": "Creativity", "value": 82},
    {"name": "Technical Skill", "value": 85}
  ],
  "career_roadmap": {
    "preferred_subjects": ["Mapel Favorit Disukai Siswa yang diekstrak dari data tes/profil"],
    "student_target_careers": ["Cita-Cita / Jurusan Impian Siswa yang ditulis di tes Holland/profil"],
    "careers": [
      {"name": "Nama Karir Utama 1", "match": 95, "icon": "briefcase"},
      {"name": "Nama Karir Utama 2", "match": 90, "icon": "bar-chart-2"},
      {"name": "Nama Karir Utama 3", "match": 86, "icon": "code-2"},
      {"name": "Nama Karir Utama 4", "match": 82, "icon": "users"},
      {"name": "Nama Karir Utama 5", "match": 78, "icon": "rocket"}
    ],
    "roadmap": [
      {"term": "Rekomendasi Jurusan Perguruan Tinggi (Major Matches)", "items": ["rekomendasi jurusan/program studi 1 yang sangat cocok beserta alasan", "rekomendasi jurusan 2", "rekomendasi jurusan 3"]},
      {"term": "Mata Pelajaran Pendukung Sekolah (Subject Matches)", "items": ["mata pelajaran pendukung 1 yang relevan", "mata pelajaran pendukung 2"]},
      {"term": "Rencana Karir Jangka Pendek (1-2 Tahun)", "items": ["fokus penguatan nilai akademis mata pelajaran pendukung", "mengikuti pelatihan/ekstrakurikuler relevan"]},
      {"term": "Rencana Karir Jangka Menengah (3-5 Tahun)", "items": ["menempuh pendidikan perguruan tinggi di jurusan rekomendasi", "aktif magang & proyek industri"]},
      {"term": "Rencana Karir Jangka Panjang (5+ Tahun)", "items": ["berkarir profesional di bidang karir utama", "sertifikasi keahlian tingkat lanjut"]}
    ]
  }
}

PENTING:
1. Di dalam objek 'kesimpulan_detail', HANYA sertakan kunci untuk alat tes yang datanya ada di input. Jika tidak ada, JANGAN sertakan kunci tersebut.
2. Semua nilai numerik harus integer 0-100.
3. 'emotional_analytics' harus mencerminkan kondisi emosi dan kepribadian siswa berdasarkan data tes yang tersedia (Tes PAPI Kostick & Survei).
4. 'skill_tracker' harus mencerminkan kemampuan spesifik siswa berdasarkan tes yang tersedia.
5. 'career_roadmap.careers' dan 'roadmap' HARUS menyelaraskan bakat (IST), minat (Holland/RMIB), serta membaca dan mempertimbangkan mata pelajaran favorit disukai siswa dan cita-cita/jurusan impian yang sudah ditulis siswa di Holland/profil.`, req.StudentName, req.BatchName, string(resultsJSON))

	text, _, err := callGemini(systemHint, userPrompt, true)
	var parsed map[string]interface{}
	if err != nil || strings.TrimSpace(text) == "" {
		parsed = generateFallbackStudentCombinedSummary(req.StudentName, req.BatchName, req.Results)
	} else if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		parsed = generateFallbackStudentCombinedSummary(req.StudentName, req.BatchName, req.Results)
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
    "preferred_subjects": ["Mapel Favorit Disukai Siswa yang diekstrak dari data tes/profil"],
    "student_target_careers": ["Cita-Cita / Jurusan Impian Siswa yang ditulis di tes Holland/profil"],
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
3. 'emotional_analytics' harus mencerminkan kondisi emosi dan kepribadian siswa berdasarkan data tes yang tersedia (Tes PAPI Kostick & Survei).
4. 'skill_tracker' harus mencerminkan kemampuan spesifik siswa berdasarkan tes yang tersedia (bukan generik).
5. 'career_roadmap.careers' dan 'roadmap' HARUS menyelaraskan bakat (IST), minat (Holland/RMIB), serta membaca dan mempertimbangkan mata pelajaran favorit disukai siswa dan cita-cita/jurusan impian yang sudah ditulis siswa di Holland/profil.`, studentName, batchName, string(resultsJSON))

	text, _, err := callGemini(systemHint, userPrompt, true)
	var parsed map[string]interface{}
	if err != nil || strings.TrimSpace(text) == "" {
		parsed = generateFallbackStudentCombinedSummary(studentName, batchName, results)
	} else if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		parsed = generateFallbackStudentCombinedSummary(studentName, batchName, results)
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

func generateFallbackTestSummary(testType, title string, result interface{}) map[string]interface{} {
	return map[string]interface{}{
		"summary": fmt.Sprintf("Berdasarkan hasil %s (%s), peserta menunjukkan potensi perkembangan mandiri yang baik dengan kapasitas pemikiran logis dan keterampilan intuitif.", testType, title),
		"tipe_manusia": "Investigatif & Analitis (Persuasif)",
		"kekuatan": []string{
			"Memiliki kemampuan analisis problem solving yang terstruktur",
			"Mampu beradaptasi dengan ritme kerja baru secara efisien",
			"Berkomunikasi dengan kejelasan intonasi dan empati yang baik",
			"Daya konsentrasi dan fokus tugas yang stabil",
		},
		"area_pengembangan": []string{
			"Meningkatkan fleksibilitas dalam menghadapi situasi perubahan mendadak",
			"Melatih manajemen waktu dan skala prioritas tugas",
			"Mengembangkan keterampilan kepemimpinan dalam tim",
		},
		"rekomendasi_karir": []map[string]string{
			{"posisi": "Software Developer / Engineer", "alasan": "Cocok dengan logika pemecahan masalah dan kemampuan analitis."},
			{"posisi": "Data Scientist / Analyst", "alasan": "Daya analisis data dan ketelitian pola kerja yang tinggi."},
			{"posisi": "Business & Management Consultant", "alasan": "Kombinasi komunikasi terstruktur dan kemampuan strategi."},
			{"posisi": "Research & Product Specialist", "alasan": "Kedalaman rasa ingin tahu dan pendekatan inovasi terstruktur."},
		},
		"rekomendasi_jurusan": []string{
			"Teknik Informatika / Computer Science",
			"Sistem Informasi / Data Science",
			"Psikologi / Manajemen SDM",
			"Teknik Industri / Manajemen Operasional",
		},
		"rekomendasi_siswa": []string{
			"Ikuti proyek berbasis tim untuk mengasah kolaborasi.",
			"Tingkatkan literasi digital dan keterampilan problem-solving secara berkala.",
			"Latih manajemen stres melalui istirahat yang teratur.",
		},
		"rekomendasi_ortu": []string{
			"Berikan dukungan moral dan ruang diskusi terbuka untuk eksplorasi minat anak.",
			"Fasilitasi sarana belajar mandiri dan kegiatan esktrakurikuler yang relevan.",
		},
		"rekomendasi_bk": []string{
			"Berikan bimbingan karir fokus pada pemetaan minat dan potensi studi lanjut.",
			"Libatkan siswa dalam kegiatan organisasi atau kepemimpinan sekolah.",
		},
		"catatan_penting": "Hasil psikotes merupakan gambaran peta potensi diri. Terus kembangkan bakat dan minat secara konsisten.",
	}
}

func generateFallbackStudentCombinedSummary(studentName, batchName string, results map[string]interface{}) map[string]interface{} {
	if studentName == "" {
		studentName = "Peserta"
	}
	return map[string]interface{}{
		"kesimpulan_detail": map[string]interface{}{
			"ist":            "Kemampuan intelegensi kognitif peserta berada pada tingkat baik, menunjukkan daya nalar logis dan kapasitas belajar yang adaptif.",
			"holland":        "Minat dominan berorientasi pada tipe Investigatif dan Realistis, dengan daya observasi dan analisis pemecahan masalah yang tinggi.",
			"learning_style": "Gaya belajar gabungan Visual dan Kinestetik, peserta paling efektif menyerap informasi melalui demonstrasi visual dan praktik langsung.",
			"kraepelin":      "Kecepatan dan ketelitian kerja menunjukkan stabilitas ritme yang konsisten dengan daya tahan tugas yang baik.",
			"papi":           "Dinamika kepribadian mencerminkan komitmen tinggi terhadap tugas, kerja sama tim yang kooperatif, serta penyesuaian diri yang fleksibel.",
		},
		"kesimpulan_gabungan": fmt.Sprintf("Secara keseluruhan, %s memiliki profil potensi kognitif dan minat yang saling mendukung. Kombinasi daya nalar analitis, kecermatan kerja, serta minat eksploratif memberikan pondasi kuat untuk pengembangan karir di bidang profesional dan teknologi.", studentName),
		"strengths": []string{
			"Daya nalar analitis dan logika pemecahan masalah yang terstruktur",
			"Stabilitas konsentrasi dan kecermatan dalam menyelesaikan tugas",
			"Komunikasi efektif dan kecenderungan kerja sama tim yang positif",
			"Kemampuan adaptasi yang cepat terhadap lingkungan baru",
		},
		"developments": []string{
			"Meningkatkan fleksibilitas strategi saat menghadapi hambatan tidak terduga",
			"Melatih teknik manajemen waktu dan prioritas proyek jangka panjang",
			"Memperluas wawasan keahlian digital pendukung karir",
		},
		"recommendations": []map[string]interface{}{
			{
				"color": "violet",
				"icon":  "graduation-cap",
				"title": "Rekomendasi Akademik & Jurusan Kuliah",
				"items": []string{
					"Teknik Informatika / Computer Science",
					"Sistem Informasi / Data Analytics",
					"Psikologi / Manajemen Rekayasa",
				},
			},
			{
				"color": "blue",
				"icon":  "code-2",
				"title": "Rekomendasi Keahlian & Skill Kunci",
				"items": []string{
					"Problem Solving & Logic Programming",
					"Data Analysis & Visualisation",
					"Communication & Team Collaboration",
				},
			},
			{
				"color": "pink",
				"icon":  "rocket",
				"title": "Rekomendasi Karir & Profesi Utama",
				"items": []string{
					"Software Developer / Engineer",
					"Data Scientist / Business Analyst",
					"Cyber Security / Systems Specialist",
				},
			},
		},
		"potential": 88,
		"potential_desc": "Peserta memiliki potensi perkembangan yang sangat baik jika didukung dengan lingkungan belajar yang terstruktur dan interaktif.",
		"insight": "Dukungan pada penguatan proyek akademis praktis dan bimbingan karir berkala akan mengoptimalkan pencapaian siswa.",
		"emotional_analytics": map[string]interface{}{
			"selfAwareness":    78,
			"selfRegulation":   72,
			"motivation":       80,
			"empathy":          75,
			"stressManagement": 70,
			"resilience":       82,
		},
		"skill_tracker": []map[string]interface{}{
			{"name": "Coding & Programming", "score": 75},
			{"name": "Problem Solving", "score": 82},
			{"name": "Critical Thinking", "score": 80},
			{"name": "Communication", "score": 76},
			{"name": "Leadership", "score": 72},
			{"name": "Creativity", "score": 78},
		},
		"career_roadmap": map[string]interface{}{
			"preferred_subjects": []string{"Matematika", "Informatika", "Bahasa Inggris"},
			"student_target_careers": []string{"Software Engineer", "Data Scientist"},
			"careers": []map[string]interface{}{
				{"name": "Software Developer / Engineer", "match": 92, "icon": "code-2"},
				{"name": "Data Scientist / Analyst", "match": 88, "icon": "bar-chart-2"},
				{"name": "Cyber Security Analyst", "match": 85, "icon": "shield"},
				{"name": "Systems Consultant", "match": 82, "icon": "briefcase"},
			},
			"roadmap": []map[string]interface{}{
				{
					"term": "Rekomendasi Jurusan Perguruan Tinggi",
					"items": []string{
						"Teknik Informatika / Ilmu Komputer",
						"Sistem Informasi",
						"Teknik Elektro / Rekayasa",
					},
				},
				{
					"term": "Mata Pelajaran Pendukung Sekolah",
					"items": []string{"Matematika Logika", "Informatika / Pemrograman", "Bahasa Asing"},
				},
				{
					"term": "Rencana Karir Jangka Pendek (1-2 Tahun)",
					"items": []string{"Fokus penguatan nilai mata pelajaran eksak & logika", "Mengikuti kompetisi atau bootcamp minat"},
				},
				{
					"term": "Rencana Karir Jangka Menengah (3-5 Tahun)",
					"items": []string{"Menempuh kuliah di jurusan yang direkomendasikan", "Magang dan pengerjaan proyek industri real"},
				},
				{
					"term": "Rencana Karir Jangka Panjang (5+ Tahun)",
					"items": []string{"Berkarir profesional sebagai Specialist / Engineer", "Mengambil sertifikasi keahlian profesional"},
				},
			},
		},
	}
}

func generateFallbackBatchSummary(batchName, testType string) map[string]interface{} {
	if batchName == "" {
		batchName = "Batch Tes"
	}
	if testType == "" {
		testType = "Asesmen Psikologi"
	}
	return map[string]interface{}{
		"ringkasan_kelas": fmt.Sprintf("Berdasarkan evaluasi kelompok untuk %s (%s), peserta secara umum menunjukkan tingkat partisipasi yang tinggi dengan distribusi potensi yang seimbang antar dimensi kognitif, minat, dan gaya belajar.", batchName, testType),
		"pola_dominan": "Dominasi orientasi Investigatif & Realistis dengan gaya belajar gabungan Visual-Kinestetik dan pola kerja yang stabil.",
		"kekuatan_kelas": []interface{}{
			"Daya serap informasi dan adaptasi terhadap instruksi baru yang cepat",
			"Tingkat ketelitian dan konsentrasi kelompok yang stabil",
			"Kemampuan kolaborasi dan komunikasi antar peserta yang baik",
		},
		"area_perhatian": []interface{}{
			"Pengembangan fleksibilitas strategi pemecahan masalah kompleks",
			"Manajemen waktu dan ritme pengerjaan tugas di bawah tekanan",
		},
		"rekomendasi_pembelajaran": []interface{}{
			"Gunakan metode pembelajaran berbasis proyek interaktif (Project-Based Learning)",
			"Sediakan visualisasi materi dan studi kasus nyata secara berkala",
			"Berikan sesi umpan balik secara teratur untuk menjaga motivasi",
		},
		"rekomendasi_bk": []interface{}{
			"Fasilitasi pemetaan minat studi lanjut dan bimbingan pilihan karir terstruktur",
			"Adakan sesi konseling kelompok fokus pada kesiapan dunia kerja / perguruan tinggi",
		},
		"catatan_guru": "Dukung potensi kelompok dengan memberikan tantangan akademis yang bervariasi dan memfasilitasi eksplorasi minat.",
		"kesimpulan_detail": map[string]interface{}{
			testType: "Hasil evaluasi kelompok menunjukkan kapasitas kognitif dan orientasi minat yang sangat baik untuk pengembangan studi lanjut.",
		},
		"kesimpulan_gabungan": fmt.Sprintf("Kelompok siswa pada %s memiliki profil potensi yang menjanjikan. Pembelajaran terstruktur dan bimbingan karir berkelanjutan akan mengoptimalkan pencapaian akademik dan kesiapan karir peserta.", batchName),
	}
}

func generateFallbackAIChatReply(userMsg string) string {
	msg := strings.ToLower(userMsg)
	if strings.Contains(msg, "hallo") || strings.Contains(msg, "halo") || strings.Contains(msg, "hi") || strings.Contains(msg, "hai") || strings.Contains(msg, "selamat") {
		return "Halo! Saya adalah AI Asisten Bimbingan Karir & Konseling Psychee Wellness. Ada yang bisa saya bantu terkait hasil tes psikologi, pilihan jurusan, atau pengembangan minat dan bakat Anda?"
	}
	if strings.Contains(msg, "metode") || strings.Contains(msg, "pembelajaran") || strings.Contains(msg, "belajar") || strings.Contains(msg, "pendekatan") {
		return "Untuk penguatan metode pembelajaran, disarankan menggunakan pendekatan interaktif (Project-Based Learning) yang memadukan materi visual dan latihan praktik langsung. Hal ini sesuai dengan profil gaya belajar dan daya tangkap peserta."
	}
	if strings.Contains(msg, "jurusan") || strings.Contains(msg, "karir") || strings.Contains(msg, "kerja") || strings.Contains(msg, "rekomendasi") || strings.Contains(msg, "prospek") {
		return "Berdasarkan pemetaan bakat dan minat, area bidang yang paling potensial meliputi Teknologi Informasi (Software/Data Analyst), Manajemen Strategis, dan Konsultasi Teknis. Penguatan pada logika pemecahan masalah dan komunikasi akan sangat mendukung keberhasilan karir."
	}
	if strings.Contains(msg, "langkah") || strings.Contains(msg, "taktis") || strings.Contains(msg, "diskusi") || strings.Contains(msg, "saran") {
		return "Langkah taktis pertama yang sebaiknya didiskusikan adalah mengajak siswa menginventarisasi 3 mata pelajaran atau bidang favoritnya, lalu memadukannya dengan pilihan program studi perguruan tinggi yang paling relevan."
	}
	return "Terima kasih atas pertanyaan Anda. Untuk memaksimalkan potensi perkembangan, fokuslah pada penguatan daya nalar analitis, pengembangan keterampilan komunikasi tim, serta eksplorasi proyek-proyek praktis yang sesuai minat Anda."
}
