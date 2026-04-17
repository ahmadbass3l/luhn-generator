package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Luhn core
// ---------------------------------------------------------------------------

// luhnCheckDigit computes the single check digit that must be appended to
// digits so that the resulting number passes the Luhn algorithm.
func luhnCheckDigit(digits []int) int {
	sum := 0
	double := true // rightmost payload digit is in "double" position
	for i := len(digits) - 1; i >= 0; i-- {
		d := digits[i]
		if double {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
		double = !double
	}
	return (10 - (sum % 10)) % 10
}

// luhnValid returns true when the full number (including its last digit as the
// check digit) satisfies the Luhn algorithm.
func luhnValid(digits []int) bool {
	if len(digits) < 2 {
		return false
	}
	sum := 0
	double := false
	for i := len(digits) - 1; i >= 0; i-- {
		d := digits[i]
		if double {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
		double = !double
	}
	return sum%10 == 0
}

// ---------------------------------------------------------------------------
// Generation
// ---------------------------------------------------------------------------

// GenerateByLength returns a cryptographically-seeded random Luhn-valid
// number string of exactly `length` digits (length >= 2).
func GenerateByLength(rng *rand.Rand, length int) (string, error) {
	if length < 2 {
		return "", fmt.Errorf("length must be at least 2")
	}
	// Fill (length-1) random payload digits, then append the check digit.
	payload := make([]int, length-1)
	for i := range payload {
		payload[i] = rng.Intn(10)
	}
	check := luhnCheckDigit(payload)
	all := append(payload, check)
	return digitsToString(all), nil
}

// GenerateByPrefix returns a Luhn-valid number of exactly `totalLength`
// digits that starts with the given prefix. The remaining digits (except the
// last) are filled randomly; the last digit is the computed check digit.
//
// Constraints:
//   - len(prefix) < totalLength  (prefix must leave room for at least the check digit)
//   - prefix must contain only digit characters
func GenerateByPrefix(rng *rand.Rand, prefix string, totalLength int) (string, error) {
	prefix = strings.ReplaceAll(prefix, " ", "")
	prefix = strings.ReplaceAll(prefix, "-", "")

	if prefix == "" {
		return "", fmt.Errorf("prefix must not be empty")
	}
	prefixDigits, err := parseDigits(prefix)
	if err != nil {
		return "", fmt.Errorf("prefix: %w", err)
	}
	if totalLength < len(prefixDigits)+1 {
		return "", fmt.Errorf(
			"total_length (%d) must be greater than prefix length (%d)",
			totalLength, len(prefixDigits),
		)
	}

	// Build payload: prefix + random middle digits (totalLength - 1 digits total)
	payloadLen := totalLength - 1
	payload := make([]int, payloadLen)
	copy(payload, prefixDigits)
	for i := len(prefixDigits); i < payloadLen; i++ {
		payload[i] = rng.Intn(10)
	}

	check := luhnCheckDigit(payload)
	all := append(payload, check)
	return digitsToString(all), nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func parseDigits(s string) ([]int, error) {
	digits := make([]int, len(s))
	for i, ch := range s {
		d, err := strconv.Atoi(string(ch))
		if err != nil {
			return nil, fmt.Errorf("non-digit character %q at position %d", ch, i)
		}
		digits[i] = d
	}
	return digits, nil
}

func digitsToString(digits []int) string {
	b := make([]byte, len(digits))
	for i, d := range digits {
		b[i] = byte('0' + d)
	}
	return string(b)
}

// ---------------------------------------------------------------------------
// HTTP
// ---------------------------------------------------------------------------

type generateResponse struct {
	Number     string `json:"number"`
	Length     int    `json:"length"`
	CheckDigit int    `json:"check_digit"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func intParam(r *http.Request, body map[string]any, key string, fallback int) (int, error) {
	// JSON body takes precedence, then form/query
	if v, ok := body[key]; ok {
		switch val := v.(type) {
		case float64:
			return int(val), nil
		case string:
			n, err := strconv.Atoi(val)
			if err != nil {
				return 0, fmt.Errorf("field %q must be an integer", key)
			}
			return n, nil
		}
	}
	raw := r.FormValue(key)
	if raw == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("field %q must be an integer", key)
	}
	return n, nil
}

func stringParam(r *http.Request, body map[string]any, key string) string {
	if v, ok := body[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return r.FormValue(key)
}

// parseBody reads JSON or form body and returns a generic map plus the
// original request (body already consumed for JSON).
func parseBody(r *http.Request) (map[string]any, error) {
	out := make(map[string]any)
	ct := r.Header.Get("Content-Type")
	if strings.Contains(ct, "application/json") {
		if err := json.NewDecoder(r.Body).Decode(&out); err != nil {
			return nil, fmt.Errorf("invalid JSON: %w", err)
		}
	} else {
		if err := r.ParseForm(); err != nil {
			return nil, fmt.Errorf("could not parse form: %w", err)
		}
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

// POST /generate/by-length
//
// Body: { "length": 16 }
// Returns a random Luhn-valid number of exactly `length` digits.
func makeHandleByLength(rng *rand.Rand) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "only POST is allowed"})
			return
		}

		body, err := parseBody(r)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
			return
		}

		length, err := intParam(r, body, "length", 0)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
			return
		}
		if length < 2 {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: `"length" must be an integer >= 2`})
			return
		}

		number, err := GenerateByLength(rng, length)
		if err != nil {
			writeJSON(w, http.StatusUnprocessableEntity, errorResponse{Error: err.Error()})
			return
		}

		checkDigit, _ := strconv.Atoi(string(number[len(number)-1]))
		writeJSON(w, http.StatusOK, generateResponse{
			Number:     number,
			Length:     len(number),
			CheckDigit: checkDigit,
		})
	}
}

// POST /generate/by-prefix
//
// Body: { "prefix": "4532", "total_length": 16 }
// Returns a Luhn-valid number of `total_length` digits starting with `prefix`.
func makeHandleByPrefix(rng *rand.Rand) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "only POST is allowed"})
			return
		}

		body, err := parseBody(r)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
			return
		}

		prefix := stringParam(r, body, "prefix")
		if prefix == "" {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: `"prefix" is required`})
			return
		}

		totalLength, err := intParam(r, body, "total_length", 0)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
			return
		}
		if totalLength < 2 {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: `"total_length" must be an integer >= 2`})
			return
		}

		number, err := GenerateByPrefix(rng, prefix, totalLength)
		if err != nil {
			writeJSON(w, http.StatusUnprocessableEntity, errorResponse{Error: err.Error()})
			return
		}

		checkDigit, _ := strconv.Atoi(string(number[len(number)-1]))
		writeJSON(w, http.StatusOK, generateResponse{
			Number:     number,
			Length:     len(number),
			CheckDigit: checkDigit,
		})
	}
}

// GET /health
func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ---------------------------------------------------------------------------
// Entry point
// ---------------------------------------------------------------------------

func main() {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/generate/by-length", makeHandleByLength(rng))
	mux.HandleFunc("/generate/by-prefix", makeHandleByPrefix(rng))
	mux.HandleFunc("/health", handleHealth)

	addr := ":" + port
	log.Printf("luhn-generator listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
