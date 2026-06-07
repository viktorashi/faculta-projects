package handlers

import (
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

//go:embed templates/* static/*
var EmbedFS embed.FS

type Satellite struct {
	ID                     int     `json:"id"`
	Name                   string  `json:"name"`
	SemimajorAxis          float64 `json:"semimajor_axis"`
	Eccentricity           float64 `json:"eccentricity"`
	Inclination            float64 `json:"inclination"`
	LongitudeAscendingNode float64 `json:"longitude_ascending_node"`
	ArgumentOfPerigee      float64 `json:"argument_of_perigee"`
}

type MergeRequest struct {
	KeepID   int    `json:"keep_id"`
	MergeIDs []int  `json:"merge_ids"`
	Name     string `json:"name"`
}

type App struct {
	DB *sql.DB
}

func InitDB() (*sql.DB, error) {
	dbUser := getEnv("DB_USER", "root")
	dbPassword := getEnv("DB_PASSWORD", "root")
	dbHost := getEnv("DB_HOST", "mysql")
	dbPort := getEnv("DB_PORT", "3306")
	dbName := getEnv("DB_NAME", "satelites")

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true", dbUser, dbPassword, dbHost, dbPort, dbName)

	var db *sql.DB
	var err error

	// Retry connecting to DB because MySQL container might take time to start
	maxRetries := 25
	for i := 1; i <= maxRetries; i++ {
		log.Printf("Connecting to database (attempt %d/%d)...", i, maxRetries)
		db, err = sql.Open("mysql", dsn)
		if err == nil {
			err = db.Ping()
			if err == nil {
				log.Println("Successfully connected to the database!")
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

func StartServer(db *sql.DB) {
	app := &App{DB: db}
	mux := http.NewServeMux()

	// Static files & Templates
	mux.HandleFunc("/", app.handleIndex)
	mux.Handle("/static/", http.FileServer(http.FS(EmbedFS)))

	// API Routes
	mux.HandleFunc("/api/satellites", app.handleSatellites)
	mux.HandleFunc("/api/satellites/", app.handleSatelliteByID)
	mux.HandleFunc("/api/duplicates", app.handleDuplicates)
	mux.HandleFunc("/api/merge", app.handleMerge)

	port := getEnv("PORT", "8000")
	log.Printf("Server starting on port %s...", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}

func (app *App) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	tmpl, err := template.ParseFS(EmbedFS, "templates/index.html")
	if err != nil {
		http.Error(w, fmt.Sprintf("Error loading template: %v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl.Execute(w, nil)
}

func (app *App) handleSatellites(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		app.listSatellites(w, r)
	case http.MethodPost:
		app.createSatellite(w, r)
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (app *App) handleSatelliteByID(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/api/satellites/")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		http.Error(w, "Invalid satellite ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		app.getSatellite(w, r, id)
	case http.MethodPut:
		app.updateSatellite(w, r, id)
	case http.MethodDelete:
		app.deleteSatellite(w, r, id)
	default:
		w.Header().Set("Allow", "GET, PUT, DELETE")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (app *App) listSatellites(w http.ResponseWriter, _ *http.Request) {
	sats, err := app.getAllSatellites()
	if err != nil {
		http.Error(w, fmt.Sprintf("Database error: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sats)
}

func (app *App) getSatellite(w http.ResponseWriter, _ *http.Request, id int) {
	var s Satellite
	err := app.DB.QueryRow(
		"SELECT id, name, semimajor_axis, eccentricity, inclination, longitude_ascending_node, argument_of_perigee FROM satelite WHERE id = ?",
		id,
	).Scan(&s.ID, &s.Name, &s.SemimajorAxis, &s.Eccentricity, &s.Inclination, &s.LongitudeAscendingNode, &s.ArgumentOfPerigee)

	if err == sql.ErrNoRows {
		http.Error(w, "Satellite not found", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, fmt.Sprintf("Database error: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s)
}

func (app *App) createSatellite(w http.ResponseWriter, r *http.Request) {
	var s Satellite
	if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
		http.Error(w, "Invalid input data", http.StatusBadRequest)
		return
	}

	// Basic validation
	if strings.TrimSpace(s.Name) == "" {
		http.Error(w, "Name cannot be empty", http.StatusBadRequest)
		return
	}
	if s.SemimajorAxis <= 0 {
		http.Error(w, "Semi-major axis must be positive", http.StatusBadRequest)
		return
	}
	if s.Eccentricity < 0 || s.Eccentricity >= 1 {
		http.Error(w, "Eccentricity must be between 0 and 1 (non-inclusive)", http.StatusBadRequest)
		return
	}

	// Check for duplicates
	strict := r.URL.Query().Get("strict") == "true"
	existing, err := app.getAllSatellites()
	if err != nil {
		http.Error(w, fmt.Sprintf("Database error: %v", err), http.StatusInternalServerError)
		return
	}

	var duplicateMatch *Satellite
	for _, ext := range existing {
		if areDuplicates(s, ext) {
			duplicateMatch = &ext
			break
		}
	}

	if duplicateMatch != nil && strict {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":     "Duplicate satellite detected",
			"duplicate": duplicateMatch,
		})
		return
	}

	// Insert into DB
	res, err := app.DB.Exec(
		"INSERT INTO satelite (name, semimajor_axis, eccentricity, inclination, longitude_ascending_node, argument_of_perigee) VALUES (?, ?, ?, ?, ?, ?)",
		s.Name, s.SemimajorAxis, s.Eccentricity, s.Inclination, s.LongitudeAscendingNode, s.ArgumentOfPerigee,
	)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to insert record: %v", err), http.StatusInternalServerError)
		return
	}

	lastID, _ := res.LastInsertId()
	s.ID = int(lastID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(s)
}

func (app *App) updateSatellite(w http.ResponseWriter, r *http.Request, id int) {
	var s Satellite
	if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
		http.Error(w, "Invalid input data", http.StatusBadRequest)
		return
	}

	// Basic validation
	if strings.TrimSpace(s.Name) == "" {
		http.Error(w, "Name cannot be empty", http.StatusBadRequest)
		return
	}
	if s.SemimajorAxis <= 0 {
		http.Error(w, "Semi-major axis must be positive", http.StatusBadRequest)
		return
	}
	if s.Eccentricity < 0 || s.Eccentricity >= 1 {
		http.Error(w, "Eccentricity must be between 0 and 1 (non-inclusive)", http.StatusBadRequest)
		return
	}

	// Check if satellite exists
	var tempID int
	err := app.DB.QueryRow("SELECT id FROM satelite WHERE id = ?", id).Scan(&tempID)
	if err == sql.ErrNoRows {
		http.Error(w, "Satellite not found", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, fmt.Sprintf("Database error: %v", err), http.StatusInternalServerError)
		return
	}

	// Check duplicates (excluding self)
	strict := r.URL.Query().Get("strict") == "true"
	existing, err := app.getAllSatellites()
	if err != nil {
		http.Error(w, fmt.Sprintf("Database error: %v", err), http.StatusInternalServerError)
		return
	}

	var duplicateMatch *Satellite
	for _, ext := range existing {
		if ext.ID != id && areDuplicates(s, ext) {
			duplicateMatch = &ext
			break
		}
	}

	if duplicateMatch != nil && strict {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":     "Duplicate satellite detected",
			"duplicate": duplicateMatch,
		})
		return
	}

	// Update DB
	_, err = app.DB.Exec(
		"UPDATE satelite SET name = ?, semimajor_axis = ?, eccentricity = ?, inclination = ?, longitude_ascending_node = ?, argument_of_perigee = ? WHERE id = ?",
		s.Name, s.SemimajorAxis, s.Eccentricity, s.Inclination, s.LongitudeAscendingNode, s.ArgumentOfPerigee, id,
	)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to update record: %v", err), http.StatusInternalServerError)
		return
	}

	s.ID = id
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s)
}

func (app *App) deleteSatellite(w http.ResponseWriter, r *http.Request, id int) {
	res, err := app.DB.Exec("DELETE FROM satelite WHERE id = ?", id)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete record: %v", err), http.StatusInternalServerError)
		return
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		http.Error(w, "Satellite not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (app *App) handleDuplicates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sats, err := app.getAllSatellites()
	if err != nil {
		http.Error(w, fmt.Sprintf("Database error: %v", err), http.StatusInternalServerError)
		return
	}

	groups := FindDuplicateGroups(sats)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(groups)
}

func (app *App) handleMerge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req MergeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid input data", http.StatusBadRequest)
		return
	}

	if req.KeepID <= 0 || len(req.MergeIDs) == 0 {
		http.Error(w, "Missing keep_id or merge_ids", http.StatusBadRequest)
		return
	}

	// Begin Transaction
	tx, err := app.DB.Begin()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to start transaction: %v", err), http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	// Update kept satellite's name if provided
	if strings.TrimSpace(req.Name) != "" {
		_, err = tx.Exec("UPDATE satelite SET name = ? WHERE id = ?", req.Name, req.KeepID)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to update merged satellite name: %v", err), http.StatusInternalServerError)
			return
		}
	}

	// Delete merged duplicates
	for _, id := range req.MergeIDs {
		if id == req.KeepID {
			continue // Skip deleting the one we want to keep
		}
		_, err = tx.Exec("DELETE FROM satelite WHERE id = ?", id)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to delete duplicate satellite (ID: %d): %v", id, err), http.StatusInternalServerError)
			return
		}
	}

	if err = tx.Commit(); err != nil {
		http.Error(w, fmt.Sprintf("Failed to commit merge transaction: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Merge successful"})
}

func (app *App) getAllSatellites() ([]Satellite, error) {
	rows, err := app.DB.Query("SELECT id, name, semimajor_axis, eccentricity, inclination, longitude_ascending_node, argument_of_perigee FROM satelite ORDER BY id DESC")
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

func FindDuplicateGroups(sats []Satellite) [][]Satellite {
	visited := make(map[int]bool)
	var groups [][]Satellite

	for i := 0; i < len(sats); i++ {
		if visited[sats[i].ID] {
			continue
		}

		group := []Satellite{sats[i]}
		for j := i + 1; j < len(sats); j++ {
			if visited[sats[j].ID] {
				continue
			}

			if areDuplicates(sats[i], sats[j]) {
				group = append(group, sats[j])
				visited[sats[j].ID] = true
			}
		}

		if len(group) > 1 {
			groups = append(groups, group)
			visited[sats[i].ID] = true
		}
	}
	return groups
}

func areDuplicates(s1, s2 Satellite) bool {
	// 1. Name matches after normalization
	if normalizeName(s1.Name) == normalizeName(s2.Name) {
		return true
	}

	// 2. Orbits match closely
	if s1.SemimajorAxis > 0 {
		pctDiff := math.Abs(s1.SemimajorAxis-s2.SemimajorAxis) / s1.SemimajorAxis
		// 1% difference in semi-major axis, 0.005 eccentricity, 0.5 degrees inclination, etc.
		if pctDiff < 0.01 &&
			math.Abs(s1.Eccentricity-s2.Eccentricity) < 0.005 &&
			math.Abs(s1.Inclination-s2.Inclination) < 0.5 &&
			math.Abs(s1.LongitudeAscendingNode-s2.LongitudeAscendingNode) < 1.0 &&
			math.Abs(s1.ArgumentOfPerigee-s2.ArgumentOfPerigee) < 1.0 {
			return true
		}
	}
	return false
}

func normalizeName(name string) string {
	s := strings.ToLower(name)
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, "_", "")
	s = strings.ReplaceAll(s, ".", "")
	return s
}

func InitWebServer() {
	db, err := InitDB()
	if err != nil {
		log.Fatalf("Fatal: Database initialization failed: %v", err)
	}
	defer db.Close()

	StartServer(db)
}
