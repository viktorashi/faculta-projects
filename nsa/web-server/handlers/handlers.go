package handlers

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
)

//go:embed templates/* static/*
var EmbedFS embed.FS

const sessionCookieName = "session_token"

var hmacSecret = []byte("astraea-space-secret-key-change-me") // secret key for signing cookies

type Satellite struct {
	ID                     int     `json:"id"`
	Name                   string  `json:"name" binding:"required"`
	SemimajorAxis          float64 `json:"semimajor_axis" binding:"required,gt=6378"`
	Eccentricity           float64 `json:"eccentricity" binding:"required,gte=0.0,lt=1.0"`
	Inclination            float64 `json:"inclination" binding:"required,gte=0.0,lte=180.0"`
	LongitudeAscendingNode float64 `json:"longitude_ascending_node" binding:"required,gte=0.0,lte=360.0"`
	ArgumentOfPerigee      float64 `json:"argument_of_perigee" binding:"required,gte=0.0,lte=360.0"`
}

type App struct {
	DBMaster *sql.DB
	DBSlave  *sql.DB
}

func InitDB(dbHost string) (*sql.DB, error) {
	dbUser := getEnv("DB_USER", "root")
	dbPassword := getEnv("DB_PASSWORD", "root")
	dbPort := getEnv("DB_PORT", "3306")
	dbName := getEnv("DB_NAME", "satelites")

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true", dbUser, dbPassword, dbHost, dbPort, dbName)

	var db *sql.DB
	var err error

	maxRetries := 25
	for i := 1; i <= maxRetries; i++ {
		log.Printf("Connecting to database at %s (attempt %d/%d)...", dbHost, i, maxRetries)
		db, err = sql.Open("mysql", dsn)
		if err == nil {
			err = db.Ping()
			if err == nil {
				log.Printf("Successfully connected to the database at %s!", dbHost)
				return db, nil
			}
		}
		log.Printf("Database connection failed: %v. Retrying in 2 seconds...", err)
		time.Sleep(2 * time.Second)
	}

	return nil, fmt.Errorf("could not connect to database after %d attempts: %w", maxRetries, err)
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

// Session Helpers
func generateSessionToken(userID int) string {
	expiration := time.Now().Add(24 * time.Hour).Unix()
	payload := fmt.Sprintf("%d_%d", userID, expiration)

	h := hmac.New(sha256.New, hmacSecret)
	h.Write([]byte(payload))
	signature := hex.EncodeToString(h.Sum(nil))

	return fmt.Sprintf("%s_%s", payload, signature)
}

func parseSessionToken(token string) (int, error) {
	parts := strings.Split(token, "_")
	if len(parts) != 3 {
		return 0, fmt.Errorf("invalid token format")
	}

	userIDStr, expirationStr, signature := parts[0], parts[1], parts[2]

	payload := fmt.Sprintf("%s_%s", userIDStr, expirationStr)
	h := hmac.New(sha256.New, hmacSecret)
	h.Write([]byte(payload))
	expectedSignature := hex.EncodeToString(h.Sum(nil))

	if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
		return 0, fmt.Errorf("signature verification failed")
	}

	expiration, err := strconv.ParseInt(expirationStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid expiration time")
	}

	if time.Now().Unix() > expiration {
		return 0, fmt.Errorf("token expired")
	}

	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		return 0, fmt.Errorf("invalid user id")
	}

	return userID, nil
}

// Auth Middleware
func (app *App) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(sessionCookieName)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			c.Abort()
			return
		}

		userID, err := parseSessionToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			c.Abort()
			return
		}

		c.Set("userID", userID)
		c.Next()
	}
}

func StartServer(dbMaster, dbSlave *sql.DB) {
	app := &App{DBMaster: dbMaster, DBSlave: dbSlave}

	r := gin.Default()

	// Load templates
	templ := template.Must(template.New("").ParseFS(EmbedFS, "templates/*.html"))
	r.SetHTMLTemplate(templ)

	// Static files mapping
	subFS, err := fs.Sub(EmbedFS, "static")
	if err != nil {
		log.Fatalf("failed to create static sub fs: %v", err)
	}
	r.StaticFS("/static", http.FS(subFS))

	// Hostname middleware to inject X-Handled-By header
	hostname, _ := os.Hostname()
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("X-Handled-By", hostname)
		c.Next()
	})

	// Page route
	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{
			"Hostname": hostname,
		})
	})

	// Auth Routes
	r.POST("/api/register", app.registerHandler)
	r.POST("/api/login", app.loginHandler)
	r.POST("/api/logout", app.logoutHandler)
	r.POST("/api/forgot-password", app.forgotPasswordHandler)
	r.POST("/api/reset-password", app.resetPasswordHandler)

	// Protected API Routes
	api := r.Group("/api")
	api.Use(app.AuthMiddleware())
	{
		api.GET("/satellites", app.listSatellites)
		api.GET("/satellites/:id", app.getSatellite)
		api.POST("/satellites", app.createSatellite)
		api.PUT("/satellites/:id", app.updateSatellite)
		api.DELETE("/satellites/:id", app.deleteSatellite)
	}

	port := getEnv("PORT", "8000")
	log.Printf("Server starting on port %s...", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}

// Existing Satellite Handlers (Ported to Gin)
func (app *App) listSatellites(c *gin.Context) {
	sats, err := app.getAllSatellites()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, sats)
}

func (app *App) getSatellite(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid satellite ID"})
		return
	}

	var s Satellite
	err = app.DBSlave.QueryRow(
		"SELECT id, name, semimajor_axis, eccentricity, inclination, longitude_ascending_node, argument_of_perigee FROM satelite WHERE id = ?",
		id,
	).Scan(&s.ID, &s.Name, &s.SemimajorAxis, &s.Eccentricity, &s.Inclination, &s.LongitudeAscendingNode, &s.ArgumentOfPerigee)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Satellite not found"})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, s)
}

func (app *App) createSatellite(c *gin.Context) {
	var s Satellite
	if err := c.ShouldBindJSON(&s); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	res, err := app.DBMaster.Exec(
		"INSERT INTO satelite (name, semimajor_axis, eccentricity, inclination, longitude_ascending_node, argument_of_perigee) VALUES (?, ?, ?, ?, ?, ?)",
		s.Name, s.SemimajorAxis, s.Eccentricity, s.Inclination, s.LongitudeAscendingNode, s.ArgumentOfPerigee,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to insert record: %v", err)})
		return
	}

	lastID, _ := res.LastInsertId()
	s.ID = int(lastID)

	c.JSON(http.StatusCreated, s)
}

func (app *App) updateSatellite(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid satellite ID"})
		return
	}

	var s Satellite
	if err := c.ShouldBindJSON(&s); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var tempID int
	err = app.DBMaster.QueryRow("SELECT id FROM satelite WHERE id = ?", id).Scan(&tempID)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Satellite not found"})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	_, err = app.DBMaster.Exec(
		"UPDATE satelite SET name = ?, semimajor_axis = ?, eccentricity = ?, inclination = ?, longitude_ascending_node = ?, argument_of_perigee = ? WHERE id = ?",
		s.Name, s.SemimajorAxis, s.Eccentricity, s.Inclination, s.LongitudeAscendingNode, s.ArgumentOfPerigee, id,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to update record: %v", err)})
		return
	}

	s.ID = id
	c.JSON(http.StatusOK, s)
}

func (app *App) deleteSatellite(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid satellite ID"})
		return
	}

	res, err := app.DBMaster.Exec("DELETE FROM satelite WHERE id = ?", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Satellite not found"})
		return
	}

	c.Status(http.StatusNoContent)
}

func (app *App) getAllSatellites() ([]Satellite, error) {
	rows, err := app.DBSlave.Query("SELECT id, name, semimajor_axis, eccentricity, inclination, longitude_ascending_node, argument_of_perigee FROM satelite ORDER BY id DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sats []Satellite
	for rows.Next() {
		var s Satellite
		err = rows.Scan(&s.ID, &s.Name, &s.SemimajorAxis, &s.Eccentricity, &s.Inclination, &s.LongitudeAscendingNode, &s.ArgumentOfPerigee)
		if err != nil {
			return nil, err
		}
		sats = append(sats, s)
	}

	if sats == nil {
		sats = []Satellite{}
	}
	return sats, nil
}

// --- NEW AUTHENTICATION HANDLERS ---

type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Username string `json:"username" binding:"required,min=3,max=30"`
	Password string `json:"password" binding:"required,min=6"`
}

func (app *App) registerHandler(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to encrypt password"})
		return
	}

	// Insert user
	_, err = app.DBMaster.Exec(
		"INSERT INTO user (email, username, password_hash) VALUES (?, ?, ?)",
		req.Email, req.Username, string(hashedPassword),
	)
	if err != nil {
		if strings.Contains(err.Error(), "Error 1062") {
			if strings.Contains(err.Error(), "email") {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Email address already registered"})
				return
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": "Username already taken"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create account"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Registration successful"})

	// Send SMTP Welcome Email via Mailpit
	go func() {
		body := fmt.Sprintf("To: %s\r\n"+
			"Subject: Welcome!\r\n"+
			"MIME-Version: 1.0\r\n"+
			"Content-Type: text/plain; charset=\"UTF-8\"\r\n"+
			"\r\n"+
			"Doar ce te-ai inscris boss, welcome!", req.Email)

		err := smtp.SendMail("mailpit:1025", nil, "no-reply@astraea.space", []string{req.Email}, []byte(body))
		if err != nil {
			log.Printf("SMTP Error: Failed to send welcome email: %v", err)
		}
	}()
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (app *App) loginHandler(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var userID int
	var passwordHash string
	err := app.DBSlave.QueryRow(
		"SELECT id, password_hash FROM user WHERE username = ? OR email = ?",
		req.Username, req.Username,
	).Scan(&userID, &passwordHash)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// Verify password
	err = bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
		return
	}

	// Set session cookie
	token := generateSessionToken(userID)
	c.SetCookie(sessionCookieName, token, 86400, "/", "", false, true)

	c.JSON(http.StatusOK, gin.H{"message": "Login successful"})
}

func (app *App) logoutHandler(c *gin.Context) {
	// Clear cookie by setting max age to -1
	c.SetCookie(sessionCookieName, "", -1, "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{"message": "Logout successful"})
}

type ForgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

func (app *App) forgotPasswordHandler(c *gin.Context) {
	var req ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var userID int
	var username string
	err := app.DBSlave.QueryRow("SELECT id, username FROM user WHERE email = ?", req.Email).Scan(&userID, &username)
	if err == sql.ErrNoRows {
		// Silently succeed to prevent account enumeration
		c.JSON(http.StatusOK, gin.H{"message": "If this email is registered, a reset link has been sent"})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// Generate random reset token
	b := make([]byte, 20)
	_, _ = rand.Read(b)
	token := hex.EncodeToString(b)
	expiry := time.Now().Add(15 * time.Minute)

	// Save token to master DB
	_, err = app.DBMaster.Exec("UPDATE user SET reset_token = ?, reset_token_expiry = ? WHERE id = ?", token, expiry, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	host := c.Request.Host
	if forwardedHost := c.GetHeader("X-Forwarded-Host"); forwardedHost != "" {
		host = forwardedHost
	}

	go func() {
		resetLink := fmt.Sprintf("%s://%s/?reset_token=%s", scheme, host, token)
		body := fmt.Sprintf("Subject: Astraea Password Reset\r\n"+
			"MIME-version: 1.0;\r\nContent-Type: text/html; charset=\"UTF-8\";\r\n\r\n"+
			"Hello %s,<br><br>A request was received to reset your password. Click the link below to set a new password:<br><br>"+
			"<a href=\"%s\" style=\"background-color:#00f2fe; color:#06070d; padding:10px 20px; text-decoration:none; font-weight:bold; border-radius:6px;\">Reset Password</a><br><br>"+
			"This link will expire in 15 minutes.<br><br>If you did not request this, you can safely ignore this email.", username, resetLink)

		err := smtp.SendMail("mailpit:1025", nil, "no-reply@astraea.space", []string{req.Email}, []byte(body))
		if err != nil {
			log.Printf("SMTP Error: Failed to send password reset email: %v", err)
		}
	}()

	c.JSON(http.StatusOK, gin.H{"message": "If this email is registered, a reset link has been sent"})
}

type ResetPasswordRequest struct {
	Token    string `json:"token" binding:"required"`
	Password string `json:"password" binding:"required,min=6"`
}

func (app *App) resetPasswordHandler(c *gin.Context) {
	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var userID int
	var expiry time.Time
	err := app.DBSlave.QueryRow("SELECT id, reset_token_expiry FROM user WHERE reset_token = ?", req.Token).Scan(&userID, &expiry)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or expired token"})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	if time.Now().After(expiry) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or expired token"})
		return
	}

	// Encrypt new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to encrypt password"})
		return
	}

	// Update password and clear token on Master DB
	_, err = app.DBMaster.Exec("UPDATE user SET password_hash = ?, reset_token = NULL, reset_token_expiry = NULL WHERE id = ?", string(hashedPassword), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update password"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password updated successfully"})
}

func InitWebServer() {
	dbHostMaster := getEnv("DB_HOST_MASTER", "mysql-master")
	dbHostSlave := getEnv("DB_HOST_SLAVE", "mysql-slave")

	dbMaster, err := InitDB(dbHostMaster)
	if err != nil {
		log.Fatalf("Fatal: Master Database initialization failed: %v", err)
	}

	dbSlave, err := InitDB(dbHostSlave)
	if err != nil {
		log.Fatalf("Fatal: Slave Database initialization failed: %v", err)
	}

	defer dbMaster.Close()
	defer dbSlave.Close()

	StartServer(dbMaster, dbSlave)
}
