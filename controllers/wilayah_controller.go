package controllers

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	beego "github.com/beego/beego/v2/server/web"
)

// WilayahController memproxy data Provinsi/Kota/Kecamatan dari sumber publik
// (emsifa/api-wilayah-indonesia) lewat backend, agar bebas dari CORS dan
// mixed-content redirect (HTTPS -> HTTP) yang terjadi pada GitHub Pages.
type WilayahController struct {
	beego.Controller
}

// Sumber data utama + fallback. Saat satu URL gagal, otomatis dicoba berikutnya.
var wilayahBases = []string{
	// Mirror unofficial / fallback (HTTPS, mengembalikan format yang sama)
	"https://www.emsifa.com/api-wilayah-indonesia/api",
	"https://emsifa.github.io/api-wilayah-indonesia/api",
}

// Cache sederhana di memori (TTL panjang karena data jarang berubah).
type cacheEntry struct {
	data      []byte
	expiresAt time.Time
}

var (
	wilayahCacheMu sync.RWMutex
	wilayahCache   = map[string]cacheEntry{}
	wilayahTTL     = 24 * time.Hour
	httpCli        = &http.Client{Timeout: 8 * time.Second}
)

func fetchWilayah(path string) ([]byte, error) {
	// Cek cache dulu
	wilayahCacheMu.RLock()
	if c, ok := wilayahCache[path]; ok && time.Now().Before(c.expiresAt) {
		wilayahCacheMu.RUnlock()
		return c.data, nil
	}
	wilayahCacheMu.RUnlock()

	var lastErr error
	for _, base := range wilayahBases {
		url := base + path
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("User-Agent", "psikologi-apps/1.0")
		resp, err := httpCli.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			lastErr = &httpError{status: resp.StatusCode, body: string(body)}
			continue
		}
		// Validasi minimum: harus berupa JSON array
		trimmed := strings.TrimSpace(string(body))
		if !strings.HasPrefix(trimmed, "[") {
			lastErr = &httpError{status: resp.StatusCode, body: "respons bukan JSON array"}
			continue
		}

		// Simpan ke cache
		wilayahCacheMu.Lock()
		wilayahCache[path] = cacheEntry{data: body, expiresAt: time.Now().Add(wilayahTTL)}
		wilayahCacheMu.Unlock()
		return body, nil
	}
	return nil, lastErr
}

type httpError struct {
	status int
	body   string
}

func (e *httpError) Error() string {
	return e.body
}

func (c *WilayahController) writeJSONBytes(b []byte) {
	c.Ctx.Output.Header("Content-Type", "application/json; charset=utf-8")
	// Cache di browser/CDN 6 jam supaya tidak repetitif
	c.Ctx.Output.Header("Cache-Control", "public, max-age=21600")
	_, _ = c.Ctx.ResponseWriter.Write(b)
}

func (c *WilayahController) writeError(status int, msg string) {
	c.Ctx.Output.SetStatus(status)
	c.Data["json"] = map[string]interface{}{"success": false, "message": msg}
	c.ServeJSON()
}

// @router /api/wilayah/provinces [get]
func (c *WilayahController) Provinces() {
	data, err := fetchWilayah("/provinces.json")
	if err != nil {
		c.writeError(502, "Gagal memuat data provinsi: "+err.Error())
		return
	}
	c.writeJSONBytes(data)
}

// @router /api/wilayah/regencies/:id [get]
func (c *WilayahController) Regencies() {
	id := c.Ctx.Input.Param(":id")
	if id == "" || !isAllNumeric(id) {
		c.writeError(400, "ID provinsi tidak valid")
		return
	}
	data, err := fetchWilayah("/regencies/" + id + ".json")
	if err != nil {
		c.writeError(502, "Gagal memuat data kota: "+err.Error())
		return
	}
	c.writeJSONBytes(data)
}

// @router /api/wilayah/districts/:id [get]
func (c *WilayahController) Districts() {
	id := c.Ctx.Input.Param(":id")
	if id == "" || !isAllNumeric(id) {
		c.writeError(400, "ID kota tidak valid")
		return
	}
	data, err := fetchWilayah("/districts/" + id + ".json")
	if err != nil {
		c.writeError(502, "Gagal memuat data kecamatan: "+err.Error())
		return
	}
	c.writeJSONBytes(data)
}

func isAllNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// Ensure encoding/json import is kept for potential future use without breaking
// build (otherwise compiler complains). Use it in a no-op helper.
var _ = json.Valid
