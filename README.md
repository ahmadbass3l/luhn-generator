# luhn-generator

A Go microservice that **generates** Luhn-valid numbers from two different starting points:

| Endpoint | Input | What it does |
|---|---|---|
| `POST /generate/by-length` | desired digit count | Generates a fully random Luhn-valid number of exactly N digits |
| `POST /generate/by-prefix` | a prefix + total length | Fills in random middle digits and appends the correct check digit |

Zero external dependencies. Single binary. Docker-ready.

---

## Relation to luhn-service

| Service | Role |
|---|---|
| [luhn-service](https://github.com/ahmadbass3l/luhn-service) | **Transform** — takes an existing number and appends its check digit |
| **luhn-generator** (this) | **Generate** — produces a brand-new Luhn-valid number from parameters |

---

## Endpoints

### `POST /generate/by-length`

Generates a random Luhn-valid number of exactly `length` digits.

**Request (JSON):**
```json
{ "length": 16 }
```

**Request (form):**
```
length=16
```

**Response:**
```json
{
  "number":      "4532015112830366",
  "length":      16,
  "check_digit": 6
}
```

---

### `POST /generate/by-prefix`

Generates a Luhn-valid number that starts with a specific prefix and has exactly `total_length` digits. The digits between the prefix and the check digit are filled randomly.

**Request (JSON):**
```json
{
  "prefix":       "4532",
  "total_length": 16
}
```

**Response:**
```json
{
  "number":      "4532748291053847",
  "length":      16,
  "check_digit": 7
}
```

The prefix may contain spaces or hyphens — they are stripped before processing.

---

### `GET /health`

```json
{ "status": "ok" }
```

---

## Quick start

### Run with Go

```bash
git clone https://github.com/ahmadbass3l/luhn-generator.git
cd luhn-generator
go run .
```

### Run with Docker

```bash
docker build -t luhn-generator .
docker run -p 8080:8080 luhn-generator
```

### Example calls

```bash
# Generate a random 16-digit Luhn-valid number
curl -s -X POST http://localhost:8080/generate/by-length \
  -H "Content-Type: application/json" \
  -d '{"length": 16}' | jq

# Generate a Visa-style number (prefix 4) of 16 digits
curl -s -X POST http://localhost:8080/generate/by-prefix \
  -H "Content-Type: application/json" \
  -d '{"prefix": "4", "total_length": 16}' | jq

# Generate an Amex-style number (prefix 378282, length 15)
curl -s -X POST http://localhost:8080/generate/by-prefix \
  -H "Content-Type: application/json" \
  -d '{"prefix": "378282", "total_length": 15}' | jq
```

---

## Algorithm

### by-length

```
1. Generate (length - 1) random digits as the payload
2. Compute Luhn check digit over the payload
3. Append check digit → Luhn-valid number of exactly `length` digits
```

### by-prefix

```
1. Parse the prefix into digits
2. Build payload = prefix + random digits, until len(payload) == total_length - 1
3. Compute Luhn check digit over the payload
4. Append check digit → Luhn-valid number of exactly `total_length` digits
   that starts with the given prefix
```

### Check digit formula

```
For payload P = [d₁, d₂, ..., dₙ]:
  - Starting from the rightmost digit, double every second digit
  - If doubling exceeds 9, subtract 9
  - Sum all values
  - check_digit = (10 − (sum mod 10)) mod 10
```

---

## Running tests

```bash
go test -v -race ./...
```

Tests cover:

- Luhn core (Wikipedia canonical example `7992739871` → check digit `3`)
- `GenerateByLength` for lengths 2, 8, 10, 16, 19
- `GenerateByPrefix` for common card prefixes (Visa, Amex, Discover)
- Error paths: prefix too long, non-digit prefix, missing fields
- All HTTP handlers (JSON + form body, method guard, validation)

---

## Configuration

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | TCP port to listen on |

---

## Project structure

```
luhn-generator/
├── main.go           # Generator logic + HTTP handlers
├── main_test.go      # Unit and HTTP handler tests
├── go.mod            # Module definition
├── Dockerfile        # Multi-stage build → scratch image
└── .github/
    └── workflows/
        └── ci.yml    # GitHub Actions: test + build on every push
```

---

## License

MIT
