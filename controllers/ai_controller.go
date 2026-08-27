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
	TestType    string                 `json:"test_type"`
	Title       string                 `json:"title"`
	StudentName string                 `json:"student_name"`
	Result      interface{}            `json:"result"`
	AllResults  map[string]interface{} `json:"all_results"`
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
	if key == "" || key == "your_gemini_api_key_here" {
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
		return "", 400, fmt.Errorf("GEMINI_API_KEY belum dikonfigurasi pada file .env.docker")
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
	var cacheKey string
	if len(req.AllResults) > 0 {
		cacheKey = getCacheHash(req.AllResults)
	} else {
		cacheKey = getCacheHash(req.Result)
	}
	var cacheFile string
	if cacheKey != "" {
		sanitizedType := strings.ToLower(strings.ReplaceAll(req.TestType, " ", "_"))
		cacheFile = fmt.Sprintf("data/ai_cache/test_v4_%s_%s.json", sanitizedType, cacheKey)
		if fileBytes, err := os.ReadFile(cacheFile); err == nil {
			var cachedData map[string]interface{}
			if err := json.Unmarshal(fileBytes, &cachedData); err == nil {
				if sum, ok := cachedData["summary"].(string); ok && !strings.Contains(sum, "Holland RIASEC (Nurdin Putra — Holland RIASEC)") {
					c.Data["json"] = aiResponse{Success: true, Data: cachedData}
					c.ServeJSON()
					return
				}
			}
		}
	}

	var resultJSON []byte
	if len(req.AllResults) > 0 {
		resultJSON, _ = json.MarshalIndent(req.AllResults, "", "  ")
	} else {
		resultJSON, _ = json.MarshalIndent(req.Result, "", "  ")
	}
	systemHint := "Anda adalah seorang Psikolog Pendidikan Senior dan Certified Career Consultant profesional. Jawablah dalam Bahasa Indonesia yang hangat, sangat jelas, mendalam, dan deskriptif. Jelaskan alasan mendasar di balik setiap rekomendasi jurusan, pekerjaan, dan pengembangan diri agar peserta dan konselor mendapatkan gambaran yang sangat terang dan berguna. PENTING: DILARANG KERAS menyertakan simbol/kode skor teknis mentah seperti (V=7), (C=7), (Z=7), (E=2), (G=3, T=5), (A=6) dalam seluruh kalimat. Terjemahkan seluruh data menjadi narasi Bahasa Indonesia yang mengalir alami, elegan, dan profesional tanpa menyebutkan simbol huruf/angka skor teknis tersebut. Jangan memberi diagnosis klinis."
	userPrompt := fmt.Sprintf(`Berdasarkan hasil tes psikologi berikut, buat analisis deskriptif dan komprehensif untuk peserta.

Subtes Aktif: %s
Judul: %s
Data hasil tes peserta (JSON):
%s

CATATAN KHUSUS: Integrasikan seluruh data tes yang tersedia di atas ke dalam analisis dan rekomendasi. DILARANG menyertakan kode skor seperti (V=7), (C=7) dalam teks narasi.

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
		parsed = generateFallbackTestSummary(req.TestType, req.Title, req.Result, req.AllResults)
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
		cacheFile = fmt.Sprintf("data/ai_cache/test_v4_%s_%s.json", sanitizedType, cacheKey)
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
		parsed = generateFallbackTestSummary(testType, "", resultData, nil)
	} else if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		parsed = generateFallbackTestSummary(testType, "", resultData, nil)
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

	// Strict enforcement: Align career roadmap, skill tracker, and recommendations with student's actual Holland dream jobs & profile
	parsed = enforceStudentProfileConstraints(parsed, req.Results)

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
		cacheFile = fmt.Sprintf("data/ai_cache/student_combined_v3_%s.json", cacheKey)
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

	// Strict enforcement: Align career roadmap, skill tracker, and recommendations with student's actual Holland dream jobs & profile
	parsed = enforceStudentProfileConstraints(parsed, results)

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

type DomainProfile struct {
	Letter             string
	TipeManusia        string
	Strengths          []string
	Developments       []string
	AcademicMajors     []string
	SkillItems         []string
	ActivityItems      []string
	CareerItems        []map[string]interface{}
	MapelItems         []string
	SkillTracker       []map[string]interface{}
	EmotionalAnalytics map[string]interface{}
}

func resolvePrimaryProfile(results map[string]interface{}) DomainProfile {
	letter := "E" // Default to Enterprising for general business/vocational

	if hollandRaw, ok := results["holland"]; ok && hollandRaw != nil {
		if hMap, ok2 := hollandRaw.(map[string]interface{}); ok2 {
			if code, ok3 := hMap["code"].(string); ok3 && len(strings.TrimSpace(code)) > 0 {
				first := strings.ToUpper(string(strings.TrimSpace(code)[0]))
				if strings.Contains("RIASEC", first) {
					letter = first
				}
			} else if top1, ok3 := hMap["top1"].(string); ok3 && len(strings.TrimSpace(top1)) > 0 {
				first := strings.ToUpper(string(strings.TrimSpace(top1)[0]))
				if strings.Contains("RIASEC", first) {
					letter = first
				}
			}
		}
	} else if rmibRaw, ok := results["rmib"]; ok && rmibRaw != nil {
		if rMap, ok2 := rmibRaw.(map[string]interface{}); ok2 {
			dom, _ := rMap["dominant_category"].(string)
			dom = strings.ToUpper(strings.TrimSpace(dom))
			rmibToLetter := map[string]string{
				"PERS": "E", "AEST": "A", "ART": "A", "MUS": "A", "LIT": "A",
				"SOC": "S", "COMP": "C", "CLER": "C", "PRAC": "R", "OUT": "R",
				"MEC": "R", "MECH": "R", "SCI": "I", "MED": "I",
			}
			if l, exists := rmibToLetter[dom]; exists {
				letter = l
			}
		}
	} else if studentRaw, ok := results["student"]; ok && studentRaw != nil {
		if sMap, ok2 := studentRaw.(map[string]interface{}); ok2 {
			jur, _ := sMap["jurusan"].(string)
			jur = strings.ToUpper(strings.TrimSpace(jur))
			if strings.Contains(jur, "BR") || strings.Contains(jur, "BD") || strings.Contains(jur, "PEMASARAN") || strings.Contains(jur, "BISNIS") || strings.Contains(jur, "RETAIL") {
				letter = "E"
			} else if strings.Contains(jur, "DKV") || strings.Contains(jur, "DESAIN") || strings.Contains(jur, "SENI") {
				letter = "A"
			} else if strings.Contains(jur, "AK") || strings.Contains(jur, "AKUN") || strings.Contains(jur, "KEUANGAN") || strings.Contains(jur, "OTKP") || strings.Contains(jur, "ADMIN") {
				letter = "C"
			} else if strings.Contains(jur, "TKJ") || strings.Contains(jur, "RPL") || strings.Contains(jur, "IPA") || strings.Contains(jur, "INFORMATIKA") {
				letter = "I"
			}
		}
	}

	// Extract dream jobs & favorite subjects if present
	var d1, d2, d3, favSubj string
	if hollandRaw, ok := results["holland"]; ok && hollandRaw != nil {
		if hMap, ok2 := hollandRaw.(map[string]interface{}); ok2 {
			d1, _ = hMap["dream_job_1"].(string)
			d2, _ = hMap["dream_job_2"].(string)
			d3, _ = hMap["dream_job_3"].(string)
			favSubj, _ = hMap["favorite_subject"].(string)

			d1 = strings.TrimSpace(d1)
			d2 = strings.TrimSpace(d2)
			d3 = strings.TrimSpace(d3)
			favSubj = strings.TrimSpace(favSubj)
		}
	}

	var baseProfile DomainProfile

	switch letter {
	case "E":
		baseProfile = DomainProfile{
			Letter:      "E",
			TipeManusia: "Enterprising & Persuasif (Bisnis & Kepemimpinan)",
			Strengths: []string{
				"Kemampuan negosiasi dan persuasi bisnis yang kuat",
				"Orientasi target pencapaian dan inisiatif tinggi",
				"Komunikasi publik dan kepemimpinan tim yang efektif",
				"Daya analisis peluang pasar dan strategi operasional",
			},
			Developments: []string{
				"Meningkatkan kesabaran dalam proses analisis teknis detail",
				"Memperdalam pengelolaan manajemen keuangan jangka panjang",
				"Melatih kontrol emosi saat menghadapi tekanan negosiasi",
			},
			AcademicMajors: []string{"Bisnis Digital / E-Commerce", "Bisnis Ritel / Pemasaran", "Manajemen Bisnis / Kewirausahaan", "Ilmu Komunikasi / Public Relations"},
			SkillItems:     []string{"Strategi Penjualan & Digital Marketing", "Negosiasi & Persuasi Bisnis", "Komunikasi & Public Speaking"},
			ActivityItems:  []string{"Kompetisi Inovasi Bisnis / E-Commerce", "Klub Kewirausahaan Muda", "Pelatihan Public Speaking & Negotiation"},
			CareerItems: []map[string]interface{}{
				{"name": "Business Development & Sales Manager", "match": 95, "icon": "briefcase"},
				{"name": "Digital Marketer & E-Commerce Specialist", "match": 92, "icon": "megaphone"},
				{"name": "Entrepreneur & Business Owner", "match": 89, "icon": "rocket"},
				{"name": "Retail & Marketing Operations Manager", "match": 86, "icon": "shopping-bag"},
			},
			MapelItems: []string{"Kewirausahaan & Bisnis", "Pemasaran Digital", "Ekonomi & Manajemen", "Komunikasi Bisnis"},
			SkillTracker: []map[string]interface{}{
				{"name": "Strategi Bisnis & Sales", "value": 88, "score": 88},
				{"name": "Negosiasi & Persuasi", "value": 85, "score": 85},
				{"name": "Komunikasi & Public Speaking", "value": 84, "score": 84},
				{"name": "Kepemimpinan Tim", "value": 82, "score": 82},
				{"name": "Manajemen Proyek", "value": 78, "score": 78},
				{"name": "Kreativitas Pemasaran", "value": 80, "score": 80},
			},
			EmotionalAnalytics: map[string]interface{}{"selfAwareness": 82, "selfRegulation": 75, "motivation": 88, "empathy": 76, "stressManagement": 78, "resilience": 84},
		}
	case "A":
		baseProfile = DomainProfile{
			Letter:      "A",
			TipeManusia: "Artistik & Kreatif (Desain & Inovasi Visual)",
			Strengths: []string{
				"Daya imajinasi dan estetika visual yang tinggi",
				"Orisinalitas ide dalam perancangan karya dan media",
				"Kreativitas ekspresif dalam penyampaian pesan",
				"Kepekaan terhadap tren visual dan konsep artistik",
			},
			Developments: []string{
				"Meningkatkan kedisiplinan dan manajemen deadline proyek",
				"Melatih toleransi terhadap kritik dan revisi karya",
				"Mengembangkan pemahaman manajemen bisnis kreatif",
			},
			AcademicMajors: []string{"Desain Komunikasi Visual (DKV)", "Desain Produk & Multimedia", "Penyiaran & Media Digital", "Seni Rupa & Desain Interior"},
			SkillItems:     []string{"Desain Graphic & Visual Branding", "UI/UX & Multimedia Editing", "Konsep Kreatif & Copywriting"},
			ActivityItems:  []string{"Pameran Karya & Kontes Desain", "Klub Multimedia & Fotografi", "Workshop Digital Illustration"},
			CareerItems: []map[string]interface{}{
				{"name": "Visual & Graphic Designer", "match": 94, "icon": "palette"},
				{"name": "Digital Media & Creative Strategist", "match": 91, "icon": "pen-tool"},
				{"name": "Content Creator & Copywriter", "match": 88, "icon": "video"},
				{"name": "UI/UX & Product Designer", "match": 85, "icon": "layout"},
			},
			MapelItems: []string{"Seni Rupa & Desain Visual", "Seni Media / Multimedia", "Bahasa & Literasi Kreatif", "Komunikasi Visual"},
			SkillTracker: []map[string]interface{}{
				{"name": "Kreativitas & Konsep", "value": 90, "score": 90},
				{"name": "Desain Visual & Estetika", "value": 88, "score": 88},
				{"name": "Media Digital & Video", "value": 85, "score": 85},
				{"name": "Komunikasi Visual", "value": 82, "score": 82},
				{"name": "Inovasi Produk", "value": 80, "score": 80},
				{"name": "Problem Solving", "value": 75, "score": 75},
			},
			EmotionalAnalytics: map[string]interface{}{"selfAwareness": 85, "selfRegulation": 70, "motivation": 82, "empathy": 80, "stressManagement": 72, "resilience": 76},
		}
	case "S":
		baseProfile = DomainProfile{
			Letter:      "S",
			TipeManusia: "Sosial & Edukatif (Pelayanan & Komunikasi)",
			Strengths: []string{
				"Empati tinggi dan kepekaan hubungan antar-manusia",
				"Komunikasi verbal yang persuasif dan menenangkan",
				"Kapasitas mendengarkan aktif dan mendampingi orang lain",
				"Kemampuan membangun kerja sama tim yang harmonis",
			},
			Developments: []string{
				"Meningkatkan ketegasan dalam pengambilan keputusan sulit",
				"Menjaga batas emosional pribadi agar tidak mudah lelah secara mental",
				"Melatih pemikiran analitis berbasis data kuantitatif",
			},
			AcademicMajors: []string{"Hubungan Masyarakat / Ilmu Komunikasi", "Manajemen SDM (HR)", "Psikologi", "Pendidikan & Keguruan"},
			SkillItems:     []string{"Interpersonal Communication & Counseling", "Public Relations & Event Management", "Talent & Team Development"},
			ActivityItems:  []string{"Organisasi Siswa & Komunitas Sosial", "Relawan & Bakti Sosial", "Klub Debat & Public Speaking"},
			CareerItems: []map[string]interface{}{
				{"name": "Public Relations & Communications", "match": 93, "icon": "users"},
				{"name": "Human Resources (HR) & Talent Specialist", "match": 90, "icon": "contact"},
				{"name": "Konselor & Educator", "match": 87, "icon": "graduation-cap"},
				{"name": "Event & Community Coordinator", "match": 84, "icon": "heart"},
			},
			MapelItems: []string{"Bahasa Indonesia / Inggris Komunikasi", "Sosiologi / Psikologi Sosial", "Komunikasi Publik", "Etika Bisnis"},
			SkillTracker: []map[string]interface{}{
				{"name": "Pelayanan & Empati", "value": 90, "score": 90},
				{"name": "Komunikasi Interpersonal", "value": 88, "score": 88},
				{"name": "Pengembangan SDM", "value": 85, "score": 85},
				{"name": "Kerja Sama Tim", "value": 86, "score": 86},
				{"name": "Resolusi Konflik", "value": 82, "score": 82},
				{"name": "Kepemimpinan", "value": 78, "score": 78},
			},
			EmotionalAnalytics: map[string]interface{}{"selfAwareness": 84, "selfRegulation": 78, "motivation": 80, "empathy": 90, "stressManagement": 75, "resilience": 80},
		}
	case "C":
		baseProfile = DomainProfile{
			Letter:      "C",
			TipeManusia: "Konvensional & Terstruktur (Keuangan & Administrasi)",
			Strengths: []string{
				"Ketelitian dan akurasi tinggi dalam pengolahan data",
				"Kepatuhan pada prosedur, aturan, dan standar baku",
				"Organisasi dokumen dan administrasi yang sangat rapi",
				"Konsistensi dan keandalan dalam tugas-tugas rutin",
			},
			Developments: []string{
				"Meningkatkan fleksibilitas terhadap perubahan prosedur mendadak",
				"Melatih pemikiran kreatif di luar panduan baku (out of the box)",
				"Mengembangkan keberanian mengambil risiko terukur",
			},
			AcademicMajors: []string{"Akuntansi & Keuangan", "Manajemen Operasional & Logistik", "Perbankan & Administrasi Bisnis", "Sistem Informasi Manajemen"},
			SkillItems:     []string{"Pengolahan Data & Financial Spreadsheet", "Administrasi Perkantoran & Audit", "Compliance & Quality Control"},
			ActivityItems:  []string{"Klub Akuntansi & Keuangan", "Simulasi Audit & Logistik", "Sertifikasi Olah Data (Excel/Spreadsheet)"},
			CareerItems: []map[string]interface{}{
				{"name": "Financial & Tax Analyst", "match": 94, "icon": "coins"},
				{"name": "Accounting & Audit Specialist", "match": 91, "icon": "calculator"},
				{"name": "Operations & Database Administrator", "match": 88, "icon": "database"},
				{"name": "Quality Control & Compliance Officer", "match": 85, "icon": "file-text"},
			},
			MapelItems: []string{"Akuntansi & Spreadsheet Data", "Matematika Ekonomi", "Administrasi Perkantoran", "Statistika Terapan"},
			SkillTracker: []map[string]interface{}{
				{"name": "Ketelitian & Akurasi Data", "value": 92, "score": 92},
				{"name": "Manajemen Keuangan & Admin", "value": 88, "score": 88},
				{"name": "Administrasi & Dokumentasi", "value": 86, "score": 86},
				{"name": "Perencanaan & Organisasi", "value": 85, "score": 85},
				{"name": "Manajemen Risiko", "value": 80, "score": 80},
				{"name": "Critical Thinking", "value": 78, "score": 78},
			},
			EmotionalAnalytics: map[string]interface{}{"selfAwareness": 80, "selfRegulation": 85, "motivation": 82, "empathy": 72, "stressManagement": 80, "resilience": 82},
		}
	case "R":
		baseProfile = DomainProfile{
			Letter:      "R",
			TipeManusia: "Realistis & Praktis (Teknik & Operasional)",
			Strengths: []string{
				"Keterampilan psikomotorik dan teknis lapangan yang handal",
				"Kemampuan pemecahan masalah mekanikal yang praktis",
				"Daya tahan kerja dan ketangguhan fisik/operasional",
				"Fokus pada hasil kerja nyata yang konkret dan terukur",
			},
			Developments: []string{
				"Meningkatkan keterampilan komunikasi tertulis dan presentasi",
				"Melatih kesabaran dalam urusan administrasi konseptual",
				"Memperluas pemahaman strategi bisnis berbasis data",
			},
			AcademicMajors: []string{"Teknik Industri / Manufaktur", "Logistik & Supply Chain", "Teknik Mesin / Otomotif", "Teknik Elektro & Otomatisasi"},
			SkillItems:     []string{"Teknik Operasional & Maintenance", "Troubleshooting Mekanikal / Sistem", "Manajemen K3 & Keselamatan Kerja"},
			ActivityItems:  []string{"Klub Otomasi & Robotika", "Praktikum Bengkel & Lapangan", "Kompetisi Rekayasa Teknis"},
			CareerItems: []map[string]interface{}{
				{"name": "Supervisor Operasional & Manufaktur", "match": 93, "icon": "settings"},
				{"name": "Teknisi Rekayasa & Otomatisasi", "match": 90, "icon": "cpu"},
				{"name": "Logistik & Supply Chain Specialist", "match": 87, "icon": "truck"},
				{"name": "Field Engineer & Maintenance", "match": 84, "icon": "wrench"},
			},
			MapelItems: []string{"Fisika / Sains Terapan", "Matematika Teknik", "Prakarya & Kewirausahaan", "Gambar Teknik"},
			SkillTracker: []map[string]interface{}{
				{"name": "Keterampilan Teknik", "value": 90, "score": 90},
				{"name": "Troubleshooting & Mekanikal", "value": 88, "score": 88},
				{"name": "Manajemen Operasional", "value": 84, "score": 84},
				{"name": "Keandalan Kerja Lapangan", "value": 86, "score": 86},
				{"name": "Penggunaan Alat & Teknologi", "value": 85, "score": 85},
				{"name": "Problem Solving", "value": 80, "score": 80},
			},
			EmotionalAnalytics: map[string]interface{}{"selfAwareness": 78, "selfRegulation": 82, "motivation": 85, "empathy": 70, "stressManagement": 82, "resilience": 86},
		}
	default:
		// I (Investigative)
		baseProfile = DomainProfile{
			Letter:      "I",
			TipeManusia: "Investigatif & Analitis (Peneliti & Analyst)",
			Strengths: []string{
				"Daya nalar analitis dan logika pemecahan masalah terstruktur",
				"Rasa ingin tahu ilmiah dan ketelitian observasi tinggi",
				"Kapasitas pemahaman konsep abstrak dan pemrosesan informasi kompleks",
				"Kemandirian belajar dan pendekatan berbasis riset data",
			},
			Developments: []string{
				"Meningkatkan keterampilan komunikasi publik dan keluwesan sosial",
				"Mengembangkan keterampilan eksekusi praktis berbasis waktu cepat",
				"Melatih keterbukaan terhadap pandangan non-analitis",
			},
			AcademicMajors: []string{"Data Science / Sistem Informasi", "Biologi / Kimia / Farmasi", "Statistika / Sains Riset", "Teknik Industri"},
			SkillItems:     []string{"Data Analysis & Problem Solving", "Riset & Metodologi Ilmiah", "Pemrosesan Logika & Riset Terapan"},
			ActivityItems:  []string{"Klub Sains & Data Research", "Olimpiade Sains & Penelitian", "Kursus Analisis Data"},
			CareerItems: []map[string]interface{}{
				{"name": "Data Analyst & Market Researcher", "match": 93, "icon": "bar-chart-2"},
				{"name": "Business Intelligence Analyst", "match": 90, "icon": "line-chart"},
				{"name": "Analyst Sistem & Inovasi", "match": 87, "icon": "search"},
				{"name": "Specialist Riset & Data", "match": 84, "icon": "database"},
			},
			MapelItems: []string{"Matematika Logika & Analisis Data", "Informatika / Sains Terapan", "Fisika / Kimia", "Statistika Terapan"},
			SkillTracker: []map[string]interface{}{
				{"name": "Penalaran Logis & Analitis", "value": 92, "score": 92},
				{"name": "Riset & Metodologi Data", "value": 88, "score": 88},
				{"name": "Pemecahan Masalah Kompleks", "value": 86, "score": 86},
				{"name": "Berpikir Kritis", "value": 85, "score": 85},
				{"name": "Ketelitian Observasi", "value": 84, "score": 84},
				{"name": "Komunikasi Data", "value": 76, "score": 76},
			},
			EmotionalAnalytics: map[string]interface{}{"selfAwareness": 82, "selfRegulation": 80, "motivation": 84, "empathy": 72, "stressManagement": 78, "resilience": 80},
		}
	}

	// Augment with student's explicit dream jobs if available
	if d1 != "" {
		var customCareers []map[string]interface{}
		customCareers = append(customCareers, map[string]interface{}{"name": d1, "match": 95, "icon": "crown"})
		if d2 != "" {
			customCareers = append(customCareers, map[string]interface{}{"name": d2, "match": 92, "icon": "briefcase"})
		}
		if d3 != "" {
			customCareers = append(customCareers, map[string]interface{}{"name": d3, "match": 89, "icon": "heart"})
		}

		for _, item := range baseProfile.CareerItems {
			name, _ := item["name"].(string)
			if name != d1 && name != d2 && name != d3 {
				customCareers = append(customCareers, item)
			}
		}
		baseProfile.CareerItems = customCareers
	}

	if favSubj != "" {
		var customMapels []string
		customMapels = append(customMapels, favSubj)
		for _, m := range baseProfile.MapelItems {
			if strings.ToLower(m) != strings.ToLower(favSubj) {
				customMapels = append(customMapels, m)
			}
		}
		baseProfile.MapelItems = customMapels
	}

	return baseProfile
}

func enforceStudentProfileConstraints(parsed map[string]interface{}, results map[string]interface{}) map[string]interface{} {
	if parsed == nil {
		parsed = make(map[string]interface{})
	}
	profile := resolvePrimaryProfile(results)

	// 1. Enforce Career Roadmap
	cr, _ := parsed["career_roadmap"].(map[string]interface{})
	if cr == nil {
		cr = make(map[string]interface{})
	}
	cr["careers"] = profile.CareerItems
	cr["preferred_subjects"] = profile.MapelItems
	if len(profile.CareerItems) > 0 {
		if n, ok := profile.CareerItems[0]["name"].(string); ok {
			cr["student_target_careers"] = []string{n}
		}
	}
	topMajor := "Jurusan Pilihan Sesuai Minat"
	if len(profile.AcademicMajors) > 0 {
		topMajor = profile.AcademicMajors[0]
	}
	topCareer := "Profesi Pilihan Utama"
	if len(profile.CareerItems) > 0 {
		if n, ok := profile.CareerItems[0]["name"].(string); ok {
			topCareer = n
		}
	}

	cr["roadmap"] = []map[string]interface{}{
		{
			"term":  "Rekomendasi Jurusan Perguruan Tinggi (Major Matches)",
			"items": profile.AcademicMajors,
		},
		{
			"term":  "Mata Pelajaran Pendukung Sekolah (Subject Matches)",
			"items": profile.MapelItems,
		},
		{
			"term":  "Rencana Karir Jangka Pendek (1-2 Tahun)",
			"items": []string{"Fokus penguatan nilai akademis mata pelajaran pendukung", "Mengikuti pelatihan/ekstrakurikuler relevan"},
		},
		{
			"term":  "Rencana Karir Jangka Menengah (3-5 Tahun)",
			"items": []string{fmt.Sprintf("Menempuh pendidikan perguruan tinggi di jurusan %s", topMajor), "Aktif magang & proyek industri"},
		},
		{
			"term":  "Rencana Karir Jangka Panjang (5+ Tahun)",
			"items": []string{fmt.Sprintf("Berkarir profesional sebagai %s", topCareer), "Sertifikasi keahlian tingkat lanjut"},
		},
	}
	parsed["career_roadmap"] = cr

	// 2. Enforce Skill Tracker
	parsed["skill_tracker"] = profile.SkillTracker

	// 3. Enforce AI Recommendations
	var topCareerNames []string
	for _, c := range profile.CareerItems {
		if name, ok := c["name"].(string); ok {
			topCareerNames = append(topCareerNames, name)
		}
	}
	if len(topCareerNames) > 3 {
		topCareerNames = topCareerNames[:3]
	}
	parsed["recommendations"] = []map[string]interface{}{
		{
			"color": "violet",
			"icon":  "graduation-cap",
			"title": "Rekomendasi Akademik & Jurusan Kuliah",
			"items": profile.AcademicMajors,
		},
		{
			"color": "blue",
			"icon":  "code-2",
			"title": "Rekomendasi Keahlian & Skill Kunci",
			"items": profile.SkillItems,
		},
		{
			"color": "pink",
			"icon":  "rocket",
			"title": "Rekomendasi Karir & Profesi Utama",
			"items": topCareerNames,
		},
	}

	return parsed
}

func generateFallbackTestSummary(testType, title string, result interface{}, allResults map[string]interface{}) map[string]interface{} {
	profile := resolvePrimaryProfile(allResults)

	var completedNames []string
	if len(allResults) > 0 {
		if r, ok := allResults["ist"]; ok && r != nil {
			if m, ok2 := r.(map[string]interface{}); ok2 && len(m) > 0 { completedNames = append(completedNames, "IST (Intelegensi)") }
		}
		if r, ok := allResults["holland"]; ok && r != nil {
			if m, ok2 := r.(map[string]interface{}); ok2 && len(m) > 0 { completedNames = append(completedNames, "Holland RIASEC") }
		}
		if r, ok := allResults["learning_style"]; ok && r != nil {
			if m, ok2 := r.(map[string]interface{}); ok2 && len(m) > 0 { completedNames = append(completedNames, "Gaya Belajar VAK") }
		}
		if r, ok := allResults["kraepelin"]; ok && r != nil {
			if m, ok2 := r.(map[string]interface{}); ok2 && len(m) > 0 { completedNames = append(completedNames, "Kraepelin") }
		}
		if r, ok := allResults["rmib"]; ok && r != nil {
			if m, ok2 := r.(map[string]interface{}); ok2 && len(m) > 0 { completedNames = append(completedNames, "RMIB") }
		}
		if r, ok := allResults["papi"]; ok && r != nil {
			if m, ok2 := r.(map[string]interface{}); ok2 && len(m) > 0 { completedNames = append(completedNames, "PAPI-Kostick") }
		}
	}

	summaryText := ""
	if len(completedNames) > 0 {
		summaryText = fmt.Sprintf("Berdasarkan integrasi evaluasi seluruh tes psikologi yang telah diselesaikan (%s), peserta menunjukkan profil potensi %s dengan orientasi minat yang kuat serta penyesuaian kerja yang adaptif.", strings.Join(completedNames, ", "), profile.TipeManusia)
	} else {
		displayTest := testType
		if displayTest == "" {
			displayTest = "Asesmen Psikologi"
		}
		summaryText = fmt.Sprintf("Berdasarkan evaluasi tes psikologi (%s), peserta menunjukkan profil potensi %s dengan daya nalar terstruktur dan minat kerja yang positif.", displayTest, profile.TipeManusia)
	}

	var rekomendasiKarir []map[string]string
	for _, c := range profile.CareerItems {
		name, _ := c["name"].(string)
		rekomendasiKarir = append(rekomendasiKarir, map[string]string{
			"posisi": name,
			"alasan": fmt.Sprintf("Sangat sesuai dengan tipe orientasi %s dan profil potensi peserta.", profile.TipeManusia),
		})
	}

	return map[string]interface{}{
		"summary":                    summaryText,
		"tipe_manusia":               profile.TipeManusia,
		"kekuatan":                   profile.Strengths,
		"area_pengembangan":          profile.Developments,
		"rekomendasi_karir":          rekomendasiKarir,
		"rekomendasi_jurusan":        profile.AcademicMajors,
		"rekomendasi_mata_pelajaran": profile.MapelItems,
		"rekomendasi_siswa": []string{
			"Kembangkan minat dan keahlian utama melalui kegiatan praktis terstruktur.",
			"Ikuti pelatihan atau proyek tim untuk mengasah keterampilan kolaborasi.",
			"Latih manajemen waktu dan stres secara berkala.",
		},
		"rekomendasi_ortu": []string{
			"Berikan dukungan moral dan ruang diskusi terbuka untuk eksplorasi minat anak.",
			"Fasilitasi sarana belajar mandiri dan kegiatan ekstrakurikuler yang relevan.",
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
	profile := resolvePrimaryProfile(results)

	topCareerName := "Spesialis Profesional"
	if len(profile.CareerItems) > 0 {
		if name, ok := profile.CareerItems[0]["name"].(string); ok {
			topCareerName = name
		}
	}

	return map[string]interface{}{
		"kesimpulan_detail": map[string]interface{}{
			"ist":            "Kemampuan intelegensi kognitif peserta berada pada tingkat yang mendukung, menunjukkan kapasitas nalar logis dan pemahaman instruksi yang baik.",
			"holland":        fmt.Sprintf("Minat dominan berorientasi pada tipe %s, menunjukkan preferensi kerja yang jelas dan berorientasi hasil.", profile.TipeManusia),
			"learning_style": "Gaya belajar peserta mendukung penyerapan informasi secara efektif melalui demonstrasi visual dan instruksi terstruktur.",
			"kraepelin":      "Ketelitian dan kecepatan kerja menunjukkan konsistensi ritme yang stabil dengan daya tahan tugas yang baik.",
			"rmib":           fmt.Sprintf("Orientasi minat pekerjaan RMIB mengonfirmasi kecenderungan kuat pada bidang %s.", profile.TipeManusia),
			"papi":           "Dinamika kepribadian mencerminkan komitmen terhadap tugas, kerja sama tim yang baik, serta adaptasi kerja yang fleksibel.",
		},
		"kesimpulan_gabungan": fmt.Sprintf("Secara keseluruhan, %s memiliki profil potensi kognitif dan minat yang saling mendukung. Orientasi %s memberikan pondasi kuat untuk pengembangan karir dan studi lanjut.", studentName, profile.TipeManusia),
		"strengths":           profile.Strengths,
		"developments":        profile.Developments,
		"recommendations": []map[string]interface{}{
			{
				"color": "violet",
				"icon":  "graduation-cap",
				"title": "Rekomendasi Akademik & Jurusan Kuliah",
				"items": profile.AcademicMajors,
			},
			{
				"color": "blue",
				"icon":  "code-2",
				"title": "Rekomendasi Keahlian & Skill Kunci",
				"items": profile.SkillItems,
			},
			{
				"color": "pink",
				"icon":  "rocket",
				"title": "Rekomendasi Kegiatan & Ekstrakurikuler",
				"items": profile.ActivityItems,
			},
		},
		"potential":      88,
		"potential_desc": fmt.Sprintf("Peserta memiliki potensi perkembangan yang sangat baik di bidang %s jika didukung lingkungan yang terstruktur.", profile.TipeManusia),
		"insight":        fmt.Sprintf("Dukungan pada penguatan bidang %s dan bimbingan karir berkala akan mengoptimalkan pencapaian %s.", profile.TipeManusia, studentName),
		"emotional_analytics": profile.EmotionalAnalytics,
		"skill_tracker":       profile.SkillTracker,
		"career_roadmap": map[string]interface{}{
			"preferred_subjects":     profile.MapelItems,
			"student_target_careers": []string{topCareerName},
			"careers":                profile.CareerItems,
			"roadmap": []map[string]interface{}{
				{
					"term":  "Rekomendasi Jurusan Perguruan Tinggi",
					"items": profile.AcademicMajors,
				},
				{
					"term":  "Mata Pelajaran Pendukung Sekolah",
					"items": profile.MapelItems,
				},
				{
					"term":  "Rencana Karir Jangka Pendek (1-2 Tahun)",
					"items": []string{"Fokus penguatan nilai mata pelajaran pendukung", "Mengikuti kegiatan ekstrakurikuler/pelatihan relevan"},
				},
				{
					"term":  "Rencana Karir Jangka Menengah (3-5 Tahun)",
					"items": []string{fmt.Sprintf("Menempuh kuliah di jurusan %s", profile.AcademicMajors[0]), "Aktif magang dan proyek praktis"},
				},
				{
					"term":  "Rencana Karir Jangka Panjang (5+ Tahun)",
					"items": []string{fmt.Sprintf("Berkarir profesional sebagai %s", topCareerName), "Mengambil sertifikasi keahlian profesional"},
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
	if strings.Contains(msg, "pelajar") || strings.Contains(msg, "pelajr") || strings.Contains(msg, "mapel") || strings.Contains(msg, "matpel") || strings.Contains(msg, "metode") || strings.Contains(msg, "pembelajaran") || strings.Contains(msg, "belajar") {
		return "Mata pelajaran pendukung utama yang paling cocok dikembangkan siswa meliputi: Matematika & Logika Analitis, Informatika / Pemrograman Dasar, serta Bahasa Asing (Inggris). Mata pelajaran ini sangat cocok untuk memperkuat daya nalar dan kapasitas kognitif siswa."
	}
	if strings.Contains(msg, "jurusan") || strings.Contains(msg, "karir") || strings.Contains(msg, "kerja") || strings.Contains(msg, "rekomendasi") || strings.Contains(msg, "prospek") {
		return "Berdasarkan pemetaan bakat dan minat, area bidang yang paling potensial meliputi Teknologi Informasi (Software/Data Analyst), Manajemen Strategis, dan Konsultasi Teknis. Penguatan pada logika pemecahan masalah dan komunikasi akan sangat mendukung keberhasilan karir."
	}
	if strings.Contains(msg, "langkah") || strings.Contains(msg, "taktis") || strings.Contains(msg, "diskusi") || strings.Contains(msg, "saran") {
		return "Langkah taktis pertama yang sebaiknya didiskusikan adalah mengajak siswa menginventarisasi 3 mata pelajaran atau bidang favoritnya, lalu memadukannya dengan pilihan program studi perguruan tinggi yang paling relevan."
	}
	return "Untuk pertanyaan spesifik ini, fokus pengembangan dapat diarahkan pada penguatan daya nalar analitis, eksplorasi mata pelajaran eksak/logika, serta pengembangan keterampilan komunikasi tim yang relevan dengan hasil asesmen siswa."
}
