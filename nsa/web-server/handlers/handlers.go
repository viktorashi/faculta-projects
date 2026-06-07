package handlers

import (
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
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

	// Retry connecting to DB because MySQL container might take time to start
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

func StartServer(dbMaster, dbSlave *sql.DB) {
	app := &App{DBMaster: dbMaster, DBSlave: dbSlave}
	mux := http.NewServeMux()

	// Static files & Templates
	mux.HandleFunc("/", app.handleIndex)
	mux.Handle("/static/", http.FileServer(http.FS(EmbedFS)))

	// API Routes
	mux.HandleFunc("/api/satellites", app.handleSatellites)
	mux.HandleFunc("/api/satellites/", app.handleSatelliteByID)

	port := getEnv("PORT", "8000")
	log.Printf("Server starting on port %s...", port)

	hostname, _ := os.Hostname()
	wrappedMux := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Handled-By", hostname)
		mux.ServeHTTP(w, r)
	})

	if err := http.ListenAndServe(":"+port, wrappedMux); err != nil {
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
	hostname, _ := os.Hostname()
	tmpl.Execute(w, map[string]interface{}{
		"Hostname": hostname,
	})
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
	err := app.DBSlave.QueryRow(
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

func (s *Satellite) Validate() error {
	if strings.TrimSpace(s.Name) == "" {
		return fmt.Errorf("Name cannot be empty")
	}
	if s.SemimajorAxis <= 0 {
		return fmt.Errorf("Semi-major axis must be positive")
	}
	if s.Eccentricity < 0 || s.Eccentricity >= 1 {
		return fmt.Errorf("Eccentricity must be between 0 and 1 (non-inclusive)")
	}
	return nil
}

func (app *App) createSatellite(w http.ResponseWriter, r *http.Request) {
	var s Satellite
	if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
		http.Error(w, "Invalid input data", http.StatusBadRequest)
		return
	}

	if err := s.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Insert into DB
	res, err := app.DBMaster.Exec(
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

	if err := s.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Check if satellite exists
	var tempID int
	err := app.DBMaster.QueryRow("SELECT id FROM satelite WHERE id = ?", id).Scan(&tempID)
	if err == sql.ErrNoRows {
		http.Error(w, "Satellite not found", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, fmt.Sprintf("Database error: %v", err), http.StatusInternalServerError)
		return
	}

	// Update DB
	_, err = app.DBMaster.Exec(
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

func (app *App) deleteSatellite(w http.ResponseWriter, _ *http.Request, id int) {
	res, err := app.DBMaster.Exec("DELETE FROM satelite WHERE id = ?", id)
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
