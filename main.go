package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/octaviocarpes/go-http-servers/internal/database"
	server "github.com/octaviocarpes/go-http-servers/server"
	utils "github.com/octaviocarpes/go-http-servers/utils"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	db             *database.Queries
}

func (config *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	metricsHandler := http.HandlerFunc(func(responseWriter http.ResponseWriter, req *http.Request) {
		config.fileserverHits.Add(1)
		// middlwares must use ServeHTTP
		responseWriter.Header().Add("Cache-Control", "no-cache")
		next.ServeHTTP(responseWriter, req)
	})

	return metricsHandler
}

func (config *apiConfig) healthHandler(responseWriter http.ResponseWriter, _ *http.Request) {
	responseWriter.Header().Add("Content-Type", "text/plain; charset=utf-8")
	responseWriter.WriteHeader(http.StatusOK)
	responseWriter.Write([]byte("OK"))
}

func (config *apiConfig) metricsHandler(responseWriter http.ResponseWriter, _ *http.Request) {
	responseWriter.Header().Add("Content-Type", "text/html; charset=utf-8")
	responseWriter.WriteHeader(http.StatusOK)

	response := fmt.Sprintf(`
    <html>
      <body>
        <h1>Welcome, Chirpy Admin</h1>
        <p>Chirpy has been visited %d times!</p>
      </body>
    </html>
  `, config.fileserverHits.Load())

	responseWriter.Write([]byte(response))
}

func (config *apiConfig) resetMetricsHandler(responseWriter http.ResponseWriter, req *http.Request) {
	env := os.Getenv("PLATFORM")

	if env != "dev" {
		responseWriter.WriteHeader(http.StatusForbidden)
		return
	}

	deleteAllUsersError := config.db.DeleteAllUsers(req.Context())

	if deleteAllUsersError != nil {
		server.SendInternalServerError(deleteAllUsersError, responseWriter)
		return
	}

	responseWriter.Header().Add("Content-Type", "text/plain; charset=utf-8")
	responseWriter.WriteHeader(http.StatusOK)
	config.fileserverHits.Swap(0)
}

func (config *apiConfig) validateChirpHandler(responseWriter http.ResponseWriter, req *http.Request) {
	const chirpSizeLimit = 140

	type requestBody struct {
		Body string `json:"body"`
	}

	decoder := json.NewDecoder(req.Body)
	reqBody := requestBody{}

	decodeError := decoder.Decode(&reqBody)

	if decodeError != nil {
		server.SendInternalServerError(decodeError, responseWriter)
		return
	}

	if len(reqBody.Body) > chirpSizeLimit {
		responsePayload := struct {
			Error string `json:"error"`
		}{
			Error: "Chirp is too long",
		}

		server.ResponseWithJson(responsePayload, http.StatusBadRequest, responseWriter)
		return
	}

	responsePhrase := strings.Split(reqBody.Body, " ")
	lowerCasePhrase := strings.Split(strings.ToLower(reqBody.Body), " ")

	for i := 0; i < len(responsePhrase); i++ {
		word := lowerCasePhrase[i]

		if utils.IsProfaneWord(word) {
			responsePhrase[i] = "****"
		}
	}

	cleanedBody := strings.Join(responsePhrase, " ")

	responsePayload := struct {
		CleanedBody string `json:"cleaned_body"`
	}{
		CleanedBody: cleanedBody,
	}

	server.ResponseWithJson(responsePayload, http.StatusOK, responseWriter)
}

func (config *apiConfig) createUser(responseWriter http.ResponseWriter, req *http.Request) {
	type createUserBody struct {
		Email string `json:"email"`
	}

	decoder := json.NewDecoder(req.Body)
	payload := createUserBody{}

	decodeError := decoder.Decode(&payload)

	if decodeError != nil {
		server.SendInternalServerError(decodeError, responseWriter)
		return
	}

	user, createUserError := config.db.CreateUser(req.Context(), payload.Email)

	if createUserError != nil {
		server.SendInternalServerError(createUserError, responseWriter)
		return
	}

	type createUserResponse struct {
		ID        string    `json:"id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Email     string    `json:"email"`
	}

	response := createUserResponse{
		ID:        user.ID,
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}

	server.ResponseWithJson(response, http.StatusCreated, responseWriter)
}

func main() {
	godotenv.Load()
	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)

	if err != nil {
		log.Fatal("failed to connect to database")
		return
	}

	dbQueries := database.New(db)

	const filepathRoot = "."
	const port = ":8080"

	config := apiConfig{
		fileserverHits: atomic.Int32{},
		db:             dbQueries,
	}

	mux := http.NewServeMux()

	handler := http.StripPrefix("/app", http.FileServer(http.Dir(filepathRoot)))

	mux.Handle("/app/", config.middlewareMetricsInc(handler))
	mux.HandleFunc("GET /api/healthz", config.healthHandler)
	mux.HandleFunc("POST /api/validate_chirp", config.validateChirpHandler)
	mux.HandleFunc("POST /api/users", config.createUser)
	mux.HandleFunc("GET /admin/metrics", config.metricsHandler)
	mux.HandleFunc("POST /admin/reset", config.resetMetricsHandler)

	server := http.Server{
		Addr:    port,
		Handler: mux,
	}

	log.Printf("Serving files from %s on port: %s\n", filepathRoot, port)

	log.Fatal(server.ListenAndServe())
}
