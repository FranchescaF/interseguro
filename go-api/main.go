// El paquete principal implementa la API de descomposición de códigos QR utilizando el framework Fiber.
// Flow: Client → Go API (QR decompose) → Node.js API (statistics) → Client
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/golang-jwt/jwt/v5"
)

// ─────────────────────────────────────────────
//  MODELS
// ─────────────────────────────────────────────

// MatrixRequest is the JSON body expected by POST /api/v1/matrix/qr
type MatrixRequest struct {
	Matrix [][]float64 `json:"matrix"`
}

// QRResult holds the two matrices produced by QR factorisation.
// A = Q · R, where Q has orthonormal columns and R is upper-triangular.
type QRResult struct {
	Q [][]float64 `json:"Q"` // m × n  orthonormal columns
	R [][]float64 `json:"R"` // n × n  upper-triangular
}

// APIResponse is the envelope returned to the caller.
type APIResponse struct {
	Success    bool        `json:"success"`
	Message    string      `json:"message,omitempty"`
	QR         *QRResult   `json:"qr,omitempty"`
	Statistics interface{} `json:"statistics,omitempty"`
}

// ─────────────────────────────────────────────
//  QR DECOMPOSITION — Modified Gram-Schmidt
// ─────────────────────────────────────────────

// dot returns the inner product of two same-length slices.
func dot(a, b []float64) float64 {
	s := 0.0
	for i := range a {
		s += a[i] * b[i]
	}
	return s
}

// vecNorm returns the Euclidean norm of v.
func vecNorm(v []float64) float64 { return math.Sqrt(dot(v, v)) }

// round8 rounds to 8 decimal places and zeroes values < 1e-10.
func round8(x float64) float64 {
	if math.Abs(x) < 1e-10 {
		return 0
	}
	return math.Round(x*1e8) / 1e8
}

// validateMatrix checks the matrix is rectangular and has rows ≥ cols.
func validateMatrix(A [][]float64) error {
	if len(A) == 0 {
		return errors.New("matrix must not be empty")
	}
	n := len(A[0])
	if n == 0 {
		return errors.New("matrix rows must not be empty")
	}
	for i, row := range A {
		if len(row) != n {
			return fmt.Errorf("row %d has %d columns, expected %d (not rectangular)", i, len(row), n)
		}
	}
	if len(A) < n {
		return fmt.Errorf("QR requires rows ≥ columns; received %d×%d", len(A), n)
	}
	return nil
}

// qrDecompose performs the economy QR decomposition via Modified Gram-Schmidt.
// For an m×n matrix A (m ≥ n):
//   - Q  is m×n with orthonormal columns
//   - R  is n×n and upper-triangular
//   - A  = Q · R
func qrDecompose(A [][]float64) (*QRResult, error) {
	if err := validateMatrix(A); err != nil {
		return nil, err
	}
	m, n := len(A), len(A[0])

	// Copy A's columns into a mutable working set.
	V := make([][]float64, n)
	for j := 0; j < n; j++ {
		V[j] = make([]float64, m)
		for i := 0; i < m; i++ {
			V[j][i] = A[i][j]
		}
	}

	// Initialise R (n×n) and the orthonormal column store for Q.
	R := make([][]float64, n)
	for i := range R {
		R[i] = make([]float64, n)
	}
	Qcols := make([][]float64, n)

	for j := 0; j < n; j++ {
		// Subtract projections onto previously computed orthonormal columns.
		for k := 0; k < j; k++ {
			R[k][j] = round8(dot(Qcols[k], V[j]))
			for i := 0; i < m; i++ {
				V[j][i] -= R[k][j] * Qcols[k][i]
			}
		}
		R[j][j] = round8(vecNorm(V[j]))
		Qcols[j] = make([]float64, m)
		if R[j][j] > 1e-12 {
			for i := 0; i < m; i++ {
				Qcols[j][i] = round8(V[j][i] / R[j][j])
			}
		}
	}

	// Assemble Q in row-major order.
	Q := make([][]float64, m)
	for i := 0; i < m; i++ {
		Q[i] = make([]float64, n)
		for j := 0; j < n; j++ {
			Q[i][j] = Qcols[j][i]
		}
	}
	return &QRResult{Q: Q, R: R}, nil
}

// ─────────────────────────────────────────────
//  INTER-SERVICE COMMUNICATION
// ─────────────────────────────────────────────

func nodeAPIURL() string {
	base := os.Getenv("NODE_API_URL")
	if base == "" {
		base = "http://127.0.0.1:3001"
	}
	return base + "/api/v1/statistics"
}

// fetchStatistics POSTs the QR result to the Node.js service and returns parsed stats.
func fetchStatistics(result *QRResult) (interface{}, error) {
	body, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("serialise error: %w", err)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(nodeAPIURL(), "application/json", bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("statistics service unreachable: %w", err)
	}
	defer resp.Body.Close()

	var out interface{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("cannot decode response: %w", err)
	}
	return out, nil
}

// ─────────────────────────────────────────────
//  JWT — optional, enabled via JWT_ENABLED=true
// ─────────────────────────────────────────────

func jwtKey() []byte {
	if s := os.Getenv("JWT_SECRET"); s != "" {
		return []byte(s)
	}
	return []byte("interseguro-secret-2026")
}

// jwtMiddleware validates Bearer tokens when JWT_ENABLED=true.
func jwtMiddleware(c *fiber.Ctx) error {
	if os.Getenv("JWT_ENABLED") != "true" {
		return c.Next()
	}
	auth := c.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false, "message": "missing or malformed Authorization header",
		})
	}
	tokenStr := strings.TrimPrefix(auth, "Bearer ")
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return jwtKey(), nil
	})
	if err != nil || !token.Valid {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false, "message": "invalid or expired token",
		})
	}
	return c.Next()
}

// handleLogin issues a 24-hour JWT for valid credentials.
func handleLogin(c *fiber.Ctx) error {
	var creds struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.BodyParser(&creds); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "invalid body"})
	}
	// Hardcoded demo credentials — use a DB in production.
	if creds.Username != "admin" || creds.Password != "interseguro2026" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "invalid credentials"})
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": creds.Username,
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	})
	signed, err := token.SignedString(jwtKey())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "token generation failed"})
	}
	return c.JSON(fiber.Map{"success": true, "token": signed})
}

// ─────────────────────────────────────────────
//  HANDLERS
// ─────────────────────────────────────────────

// handleQR is the main endpoint: accepts a matrix, runs QR, fetches stats, returns all.
func handleQR(c *fiber.Ctx) error {
	var req MatrixRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(APIResponse{
			Success: false, Message: "invalid JSON: " + err.Error(),
		})
	}

	result, err := qrDecompose(req.Matrix)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(APIResponse{
			Success: false, Message: err.Error(),
		})
	}

	stats, err := fetchStatistics(result)
	if err != nil {
		log.Printf("[WARN] statistics service error: %v", err)
		return c.JSON(APIResponse{
			Success: true,
			Message: "QR complete (statistics unavailable: " + err.Error() + ")",
			QR:      result,
		})
	}

	return c.JSON(APIResponse{
		Success:    true,
		Message:    "QR decomposition and statistics completed successfully",
		QR:         result,
		Statistics: stats,
	})
}

// ─────────────────────────────────────────────
//  ENTRY POINT
// ─────────────────────────────────────────────

func main() {
	app := fiber.New(fiber.Config{AppName: "Interseguro QR API v1.0"})
	app.Use(recover.New(), logger.New(), cors.New())

	// Health check (no auth)
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "service": "go-api"})
	})

	v1 := app.Group("/api/v1")
	v1.Post("/auth/login", handleLogin)            // obtain JWT
	v1.Post("/matrix/qr", jwtMiddleware, handleQR) // protected endpoint

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	log.Printf("Go QR API starting on :%s  (JWT_ENABLED=%s)", port, os.Getenv("JWT_ENABLED"))
	log.Fatal(app.Listen(":" + port))
}
