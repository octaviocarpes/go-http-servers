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

func (config *apiConfig) createChirp(responseWriter http.ResponseWriter, req *http.Request) {
	type createChirpBody struct {
		Body   string `json:"body"`
		UserId string `json:"user_id"`
	}

	const chirpSizeLimit = 140

	decodedPayload, decodeError := server.DecodeBody[createChirpBody](req.Body)

	if decodeError != nil {
		server.SendInternalServerError(decodeError, responseWriter)
		return
	}

	if len(decodedPayload.Body) > chirpSizeLimit {
		responsePayload := struct {
			Error string `json:"error"`
		}{
			Error: "Chirp is too long",
		}

		server.ResponseWithJson(responsePayload, http.StatusBadRequest, responseWriter)
		return
	}

	responsePhrase := strings.Split(decodedPayload.Body, " ")
	lowerCasePhrase := strings.Split(strings.ToLower(decodedPayload.Body), " ")

	for i := 0; i < len(responsePhrase); i++ {
		word := lowerCasePhrase[i]

		if utils.IsProfaneWord(word) {
			responsePhrase[i] = "****"
		}
	}

	cleanedBody := strings.Join(responsePhrase, " ")

	payload := database.CreateChirpParams{
		Body:   cleanedBody,
		UserID: decodedPayload.UserId,
	}

	chirp, createChirpError := config.db.CreateChirp(req.Context(), payload)

	if createChirpError != nil {
		server.SendInternalServerError(createChirpError, responseWriter)
		return
	}

	responsePayload := struct {
		Id        string    `json:"id"`
		Body      string    `json:"body"`
		UserId    string    `json:"user_id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
	}{
		Id:        chirp.ID,
		Body:      chirp.Body,
		UserId:    chirp.UserID,
		CreatedAt: chirp.CreatedAt,
		UpdatedAt: chirp.UpdatedAt,
	}

	server.ResponseWithJson(responsePayload, http.StatusCreated, responseWriter)
}

func (config *apiConfig) listChirps(responseWriter http.ResponseWriter, req *http.Request) {
	chirps, listChirpsError := config.db.ListChirps(req.Context())

	if listChirpsError != nil {
		server.SendInternalServerError(listChirpsError, responseWriter)
		return
	}

	type chirpResponse struct {
		ID        string    `json:"id"`
		Body      string    `json:"body"`
		UserID    string    `json:"user_id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
	}

	response := make([]chirpResponse, len(chirps))

	for i, chirp := range chirps {
		response[i] = chirpResponse{
			ID:        chirp.ID,
			Body:      chirp.Body,
			UserID:    chirp.UserID,
			CreatedAt: chirp.CreatedAt,
			UpdatedAt: chirp.UpdatedAt,
		}
	}

	server.ResponseWithJson(response, http.StatusOK, responseWriter)
}

func (config *apiConfig) getChirpById(responseWriter http.ResponseWriter, req *http.Request) {
	id := req.PathValue("id")
	chirp, getChirpError := config.db.GetChirpByID(req.Context(), id)

	if getChirpError != nil {
		responseWriter.Header().Set("Content-Type", "application/json")
		responseWriter.WriteHeader(http.StatusNotFound)
		responseWriter.Write([]byte(`{"error": "chirp not found"}`))
		return
	}

	type chirpResponse struct {
		ID        string    `json:"id"`
		Body      string    `json:"body"`
		UserID    string    `json:"user_id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
	}

	response := chirpResponse{
		ID:        chirp.ID,
		Body:      chirp.Body,
		UserID:    chirp.UserID,
		CreatedAt: chirp.CreatedAt,
		UpdatedAt: chirp.UpdatedAt,
	}

	server.ResponseWithJson(response, http.StatusOK, responseWriter)
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
	mux.HandleFunc("POST /api/users", config.createUser)
	mux.HandleFunc("GET /api/chirps", config.listChirps)
	mux.HandleFunc("GET /api/chirps/{id}", config.getChirpById)
	mux.HandleFunc("POST /api/chirps", config.createChirp)

	mux.HandleFunc("GET /admin/metrics", config.metricsHandler)
	mux.HandleFunc("POST /admin/reset", config.resetMetricsHandler)

	server := http.Server{
		Addr:    port,
		Handler: mux,
	}

	log.Printf("Serving files from %s on port: %s\n", filepathRoot, port)

	log.Fatal(server.ListenAndServe())
}
