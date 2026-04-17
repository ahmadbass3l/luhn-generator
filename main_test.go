package main

import (
	"bytes"
	"encoding/json"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// seeded rng for deterministic tests
var testRng = rand.New(rand.NewSource(42))

// ---------------------------------------------------------------------------
// Unit tests — Luhn core
// ---------------------------------------------------------------------------

func TestLuhnCheckDigit_WikipediaExample(t *testing.T) {
	// Payload "7992739871" → check digit 3  (Wikipedia canonical example)
	digits := []int{7, 9, 9, 2, 7, 3, 9, 8, 7, 1}
	if got := luhnCheckDigit(digits); got != 3 {
		t.Errorf("expected 3, got %d", got)
	}
}

func TestLuhnValid(t *testing.T) {
	valid := []string{"79927398713", "4532015112830366", "18"}
	for _, s := range valid {
		digits, _ := parseDigits(s)
		if !luhnValid(digits) {
			t.Errorf("%s should be valid", s)
		}
	}

	invalid := []string{"79927398710", "4532015112830360"}
	for _, s := range invalid {
		digits, _ := parseDigits(s)
		if luhnValid(digits) {
			t.Errorf("%s should be invalid", s)
		}
	}
}

// ---------------------------------------------------------------------------
// Unit tests — GenerateByLength
// ---------------------------------------------------------------------------

func TestGenerateByLength_ValidOutput(t *testing.T) {
	rng := rand.New(rand.NewSource(99))
	for _, length := range []int{2, 8, 10, 16, 19} {
		num, err := GenerateByLength(rng, length)
		if err != nil {
			t.Fatalf("length=%d: unexpected error: %v", length, err)
		}
		if len(num) != length {
			t.Errorf("length=%d: got string of length %d", length, len(num))
		}
		digits, _ := parseDigits(num)
		if !luhnValid(digits) {
			t.Errorf("length=%d: generated %s does not pass Luhn", length, num)
		}
	}
}

func TestGenerateByLength_TooShort(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	_, err := GenerateByLength(rng, 1)
	if err == nil {
		t.Error("expected error for length=1")
	}
}

func TestGenerateByLength_Randomness(t *testing.T) {
	// Two calls with different seeds should (almost certainly) produce different numbers
	n1, _ := GenerateByLength(rand.New(rand.NewSource(1)), 16)
	n2, _ := GenerateByLength(rand.New(rand.NewSource(2)), 16)
	if n1 == n2 {
		t.Error("expected different numbers from different seeds")
	}
}

// ---------------------------------------------------------------------------
// Unit tests — GenerateByPrefix
// ---------------------------------------------------------------------------

func TestGenerateByPrefix_ValidOutput(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	cases := []struct {
		prefix string
		length int
	}{
		{"4", 16},
		{"4532", 16},
		{"378282", 15}, // Amex-style
		{"6011", 16},
	}
	for _, c := range cases {
		num, err := GenerateByPrefix(rng, c.prefix, c.length)
		if err != nil {
			t.Fatalf("prefix=%s len=%d: unexpected error: %v", c.prefix, c.length, err)
		}
		if len(num) != c.length {
			t.Errorf("prefix=%s: expected length %d, got %d", c.prefix, c.length, len(num))
		}
		if !strings.HasPrefix(num, c.prefix) {
			t.Errorf("prefix=%s: result %s does not start with prefix", c.prefix, num)
		}
		digits, _ := parseDigits(num)
		if !luhnValid(digits) {
			t.Errorf("prefix=%s: generated %s does not pass Luhn", c.prefix, num)
		}
	}
}

func TestGenerateByPrefix_LengthTooShort(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	_, err := GenerateByPrefix(rng, "4532", 4) // prefix len == total_length, no room for check digit
	if err == nil {
		t.Error("expected error when total_length <= prefix length")
	}
}

func TestGenerateByPrefix_NonDigitPrefix(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	_, err := GenerateByPrefix(rng, "45XX", 16)
	if err == nil {
		t.Error("expected error for non-digit prefix")
	}
}

func TestGenerateByPrefix_SeparatorsStripped(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	num, err := GenerateByPrefix(rng, "4532 ", 16)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(num, "4532") {
		t.Errorf("expected prefix 4532, got %s", num)
	}
}

// ---------------------------------------------------------------------------
// HTTP handler tests — /generate/by-length
// ---------------------------------------------------------------------------

func postJSON(handler http.HandlerFunc, path string, body any) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler(rec, req)
	return rec
}

func TestHandleByLength_OK(t *testing.T) {
	h := makeHandleByLength(testRng)
	rec := postJSON(h, "/generate/by-length", map[string]any{"length": 16})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp generateResponse
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if len(resp.Number) != 16 {
		t.Errorf("expected 16 digits, got %d", len(resp.Number))
	}
	digits, _ := parseDigits(resp.Number)
	if !luhnValid(digits) {
		t.Errorf("generated number %s does not pass Luhn", resp.Number)
	}
}

func TestHandleByLength_MissingLength(t *testing.T) {
	h := makeHandleByLength(testRng)
	rec := postJSON(h, "/generate/by-length", map[string]any{})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestHandleByLength_LengthOne(t *testing.T) {
	h := makeHandleByLength(testRng)
	rec := postJSON(h, "/generate/by-length", map[string]any{"length": 1})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestHandleByLength_MethodNotAllowed(t *testing.T) {
	h := makeHandleByLength(testRng)
	req := httptest.NewRequest(http.MethodGet, "/generate/by-length", nil)
	rec := httptest.NewRecorder()
	h(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}

func TestHandleByLength_FormBody(t *testing.T) {
	h := makeHandleByLength(testRng)
	body := strings.NewReader("length=10")
	req := httptest.NewRequest(http.MethodPost, "/generate/by-length", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// HTTP handler tests — /generate/by-prefix
// ---------------------------------------------------------------------------

func TestHandleByPrefix_OK(t *testing.T) {
	h := makeHandleByPrefix(testRng)
	rec := postJSON(h, "/generate/by-prefix", map[string]any{
		"prefix":       "4532",
		"total_length": 16,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp generateResponse
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if !strings.HasPrefix(resp.Number, "4532") {
		t.Errorf("expected prefix 4532, got %s", resp.Number)
	}
	if len(resp.Number) != 16 {
		t.Errorf("expected length 16, got %d", len(resp.Number))
	}
	digits, _ := parseDigits(resp.Number)
	if !luhnValid(digits) {
		t.Errorf("generated number %s does not pass Luhn", resp.Number)
	}
}

func TestHandleByPrefix_MissingPrefix(t *testing.T) {
	h := makeHandleByPrefix(testRng)
	rec := postJSON(h, "/generate/by-prefix", map[string]any{"total_length": 16})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestHandleByPrefix_MissingTotalLength(t *testing.T) {
	h := makeHandleByPrefix(testRng)
	rec := postJSON(h, "/generate/by-prefix", map[string]any{"prefix": "4532"})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestHandleByPrefix_InvalidPrefix(t *testing.T) {
	h := makeHandleByPrefix(testRng)
	rec := postJSON(h, "/generate/by-prefix", map[string]any{
		"prefix":       "45XX",
		"total_length": 16,
	})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", rec.Code)
	}
}

func TestHandleByPrefix_MethodNotAllowed(t *testing.T) {
	h := makeHandleByPrefix(testRng)
	req := httptest.NewRequest(http.MethodGet, "/generate/by-prefix", nil)
	rec := httptest.NewRecorder()
	h(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// Health check
// ---------------------------------------------------------------------------

func TestHandleHealth(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handleHealth(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}
