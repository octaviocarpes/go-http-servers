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

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	auth "github.com/octaviocarpes/go-http-servers/internal/auth"
	"github.com/octaviocarpes/go-http-servers/internal/database"
	server "github.com/octaviocarpes/go-http-servers/server"
	utils "github.com/octaviocarpes/go-http-servers/utils"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	db             *database.Queries
	secret         string
	polkaKey       string
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
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	decoder := json.NewDecoder(req.Body)
	payload := createUserBody{}

	decodeError := decoder.Decode(&payload)

	if decodeError != nil {
		server.SendInternalServerError(decodeError, responseWriter)
		return
	}

	hashedPassword, hashError := auth.HashPassword(payload.Password)

	if hashError != nil {
		server.SendInternalServerError(hashError, responseWriter)
		return
	}

	user, createUserError := config.db.CreateUser(req.Context(), database.CreateUserParams{
		Email:          payload.Email,
		HashedPassword: hashedPassword,
	})

	if createUserError != nil {
		server.SendInternalServerError(createUserError, responseWriter)
		return
	}

	type createUserResponse struct {
		ID          string    `json:"id"`
		CreatedAt   time.Time `json:"created_at"`
		UpdatedAt   time.Time `json:"updated_at"`
		Email       string    `json:"email"`
		IsChirpyRed bool      `json:"is_chirpy_red"`
	}

	response := createUserResponse{
		ID:          user.ID,
		Email:       user.Email,
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
		IsChirpyRed: user.IsChirpyRed.Bool,
	}

	server.ResponseWithJson(response, http.StatusCreated, responseWriter)
}

func (config *apiConfig) createChirp(responseWriter http.ResponseWriter, req *http.Request) {
	token, getTokenErr := auth.GetBearerToken(req.Header)

	if getTokenErr != nil {
		responseWriter.WriteHeader(http.StatusUnauthorized)
		responseWriter.Header().Set("Content-Type", "application/json")
		responseWriter.Write([]byte(`{"error": "Unauthorized"}`))
		return
	}

	userUUID, invalidTokenError := auth.ValidateJWT(token, config.secret)

	if invalidTokenError != nil {
		responseWriter.WriteHeader(http.StatusUnauthorized)
		responseWriter.Header().Set("Content-Type", "application/json")
		responseWriter.Write([]byte(`{"error": "Unauthorized"}`))
		return
	}

	type createChirpBody struct {
		Body string `json:"body"`
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
		UserID: userUUID.String(),
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
	authorID := req.URL.Query().Get("author_id")
	sort := req.URL.Query().Get("sort")

	var authorParam sql.NullString
	sortParam := true

	if len(authorID) == 0 {
		authorParam = sql.NullString{
			Valid: false,
		}
	} else {
		authorParam = sql.NullString{
			Valid:  true,
			String: authorID,
		}
	}

	if len(sort) > 0 && (sort != "desc" && sort != "asc") {
		responseWriter.WriteHeader(http.StatusBadRequest)
		responseWriter.Header().Set("Content-Type", "application/json")
		responseWriter.Write([]byte(`{"error": "sort param can only be desc or asc"}`))
		return
	}

	if sort == "desc" {
		sortParam = false
	}

	chirps, listChirpsError := config.db.ListChirps(req.Context(), database.ListChirpsParams{
		AuthorID: authorParam,
		Column1:  sortParam,
	})

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

func (config *apiConfig) login(responseWriter http.ResponseWriter, req *http.Request) {
	type loginBody struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	decodedPayload, decodeError := server.DecodeBody[loginBody](req.Body)

	if decodeError != nil {
		server.SendInternalServerError(decodeError, responseWriter)
		return
	}

	user, getUserError := config.db.GetUserByEmail(req.Context(), decodedPayload.Email)

	if getUserError != nil {
		server.SendInternalServerError(getUserError, responseWriter)
		return
	}

	checkPasswordError := auth.CheckPasswordHash(decodedPayload.Password, user.HashedPassword)

	if checkPasswordError != nil {
		fmt.Printf("%v", checkPasswordError)
		responseWriter.Header().Set("Content-Type", "application/json")
		responseWriter.WriteHeader(http.StatusUnauthorized)
		responseWriter.Write([]byte(`{"error": "wrong credentials"}`))
		return
	}

	type userResponse struct {
		ID           string    `json:"id"`
		CreatedAt    time.Time `json:"created_at"`
		UpdatedAt    time.Time `json:"updated_at"`
		Email        string    `json:"email"`
		IsChirpyRed  bool      `json:"is_chirpy_red"`
		Token        string    `json:"token"`
		RefreshToken string    `json:"refresh_token"`
	}

	userUUID, uuidErr := uuid.Parse(user.ID)

	if uuidErr != nil {
		server.SendInternalServerError(uuidErr, responseWriter)
		return
	}

	expiration := time.Duration(time.Hour * 1)

	token, createTokenErr := auth.MakeJWT(userUUID, config.secret, expiration)

	if createTokenErr != nil {
		server.SendInternalServerError(createTokenErr, responseWriter)
		return
	}

	refreshToken, makeRefreshTokenError := auth.MakeRefreshToken()

	if makeRefreshTokenError != nil {
		server.SendInternalServerError(makeRefreshTokenError, responseWriter)
		return
	}

	createRefreshTokenPaylod := database.CreateRefreshTokenParams{
		Token:     refreshToken,
		UserID:    user.ID,
		ExpiresAt: time.Now().Add(time.Duration(24*time.Hour) * 60),
	}

	rToken, createRefreshTokenError := config.db.CreateRefreshToken(req.Context(), createRefreshTokenPaylod)

	if createRefreshTokenError != nil {
		server.SendInternalServerError(createRefreshTokenError, responseWriter)
		return
	}

	server.ResponseWithJson(userResponse{
		ID:           user.ID,
		Email:        user.Email,
		CreatedAt:    user.CreatedAt,
		UpdatedAt:    user.UpdatedAt,
		IsChirpyRed:  user.IsChirpyRed.Bool,
		Token:        token,
		RefreshToken: rToken.Token,
	}, http.StatusOK, responseWriter)
}

func (config *apiConfig) refreshSession(responseWriter http.ResponseWriter, req *http.Request) {
	token, getTokenErr := auth.GetBearerToken(req.Header)

	if getTokenErr != nil {
		responseWriter.WriteHeader(http.StatusUnauthorized)
		responseWriter.Header().Set("Content-Type", "application/json")
		responseWriter.Write([]byte(`{"error": "Unauthorized"}`))
		return
	}

	dbToken, getTokenErr := config.db.GetRefreshToken(req.Context(), token)

	if getTokenErr != nil {
		responseWriter.WriteHeader(http.StatusUnauthorized)
		responseWriter.Header().Set("Content-Type", "application/json")
		responseWriter.Write([]byte(`{"error": "Unauthorized"}`))
		return
	}

	if dbToken.ExpiresAt.Before(time.Now()) {
		responseWriter.WriteHeader(http.StatusUnauthorized)
		responseWriter.Header().Set("Content-Type", "application/json")
		responseWriter.Write([]byte(`{"error": "Unauthorized"}`))
		return
	}

	if dbToken.RevokedAt.Valid {
		responseWriter.WriteHeader(http.StatusUnauthorized)
		responseWriter.Header().Set("Content-Type", "application/json")
		responseWriter.Write([]byte(`{"error": "Unauthorized"}`))
		return
	}

	userId, err := uuid.Parse(dbToken.UserID)

	if err != nil {
		server.SendInternalServerError(err, responseWriter)
		return
	}

	expiration := time.Duration(time.Hour * 1)

	token, createTokenErr := auth.MakeJWT(userId, config.secret, expiration)

	if createTokenErr != nil {
		server.SendInternalServerError(createTokenErr, responseWriter)
		return
	}

	responseWriter.WriteHeader(http.StatusOK)
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.Write([]byte(fmt.Sprintf(`{"token": "%v"}`, token)))
}

func (config *apiConfig) revokeSession(responseWriter http.ResponseWriter, req *http.Request) {
	token, getTokenErr := auth.GetBearerToken(req.Header)

	if getTokenErr != nil {
		responseWriter.WriteHeader(http.StatusUnauthorized)
		responseWriter.Header().Set("Content-Type", "application/json")
		responseWriter.Write([]byte(`{"error": "Unauthorized"}`))
		return
	}

	revokeError := config.db.RevokeToken(req.Context(), token)

	if revokeError != nil {
		server.SendInternalServerError(revokeError, responseWriter)
		return
	}

	responseWriter.WriteHeader(http.StatusNoContent)
}

func (config *apiConfig) updateUser(responseWriter http.ResponseWriter, req *http.Request) {
	token, getTokenErr := auth.GetBearerToken(req.Header)

	if getTokenErr != nil {
		responseWriter.WriteHeader(http.StatusUnauthorized)
		responseWriter.Header().Set("Content-Type", "application/json")
		responseWriter.Write([]byte(`{"error": "Unauthorized"}`))
		return
	}

	userUUID, invalidTokenError := auth.ValidateJWT(token, config.secret)

	if invalidTokenError != nil {
		responseWriter.WriteHeader(http.StatusUnauthorized)
		responseWriter.Header().Set("Content-Type", "application/json")
		responseWriter.Write([]byte(`{"error": "Unauthorized"}`))
		return
	}

	type updateUserBody struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	decodedPayload, decodeError := server.DecodeBody[updateUserBody](req.Body)

	if decodeError != nil {
		server.SendInternalServerError(decodeError, responseWriter)
		return
	}

	hashedPassword, hashError := auth.HashPassword(decodedPayload.Password)

	if hashError != nil {
		server.SendInternalServerError(hashError, responseWriter)
		return
	}

	updatedUser, updateUserError := config.db.UpdateUser(req.Context(), database.UpdateUserParams{
		Email:          decodedPayload.Email,
		HashedPassword: hashedPassword,
		ID:             userUUID.String(),
	})

	if updateUserError != nil {
		server.SendInternalServerError(updateUserError, responseWriter)
		return
	}

	type responseBody struct {
		Email string `json:"email"`
	}

	response := responseBody{
		Email: updatedUser.Email,
	}

	server.ResponseWithJson(response, http.StatusOK, responseWriter)
}

func (config *apiConfig) deleteChirp(responseWriter http.ResponseWriter, req *http.Request) {
	token, getTokenErr := auth.GetBearerToken(req.Header)

	if getTokenErr != nil {
		responseWriter.WriteHeader(http.StatusUnauthorized)
		responseWriter.Header().Set("Content-Type", "application/json")
		responseWriter.Write([]byte(`{"error": "Unauthorized"}`))
		return
	}

	userUUID, invalidTokenError := auth.ValidateJWT(token, config.secret)

	if invalidTokenError != nil {
		responseWriter.WriteHeader(http.StatusUnauthorized)
		responseWriter.Header().Set("Content-Type", "application/json")
		responseWriter.Write([]byte(`{"error": "Unauthorized"}`))
		return
	}

	chirpID := req.PathValue("id")

	chirp, getChirpError := config.db.GetChirpByID(req.Context(), chirpID)

	if getChirpError != nil {
		responseWriter.Header().Set("Content-Type", "application/json")
		responseWriter.WriteHeader(http.StatusNotFound)
		responseWriter.Write([]byte(`{"error": "chirp not found"}`))
		return
	}

	if chirp.UserID != userUUID.String() {
		responseWriter.WriteHeader(http.StatusForbidden)
		return
	}

	deleteChirpError := config.db.DeleteChirp(req.Context(), database.DeleteChirpParams{
		ID:     chirpID,
		UserID: userUUID.String(),
	})

	if deleteChirpError != nil {
		server.SendInternalServerError(deleteChirpError, responseWriter)
		return
	}

	responseWriter.WriteHeader(http.StatusNoContent)
}

func (config *apiConfig) polkaWebhooks(responseWriter http.ResponseWriter, req *http.Request) {
	apiKey, getTokenErr := auth.GetApiKey(req.Header)

	if getTokenErr != nil {
		responseWriter.WriteHeader(http.StatusUnauthorized)
		responseWriter.Header().Set("Content-Type", "application/json")
		responseWriter.Write([]byte(`{"error": "Unauthorized"}`))
		return
	}

	if apiKey != config.polkaKey {
		responseWriter.WriteHeader(http.StatusUnauthorized)
		responseWriter.Header().Set("Content-Type", "application/json")
		responseWriter.Write([]byte(`{"error": "Unauthorized"}`))
		return
	}

	type webhookBody struct {
		Event string `json:"event"`
		Data  struct {
			UserID string `json:"user_id"`
		} `json:"data"`
	}

	decodedPayload, decodeError := server.DecodeBody[webhookBody](req.Body)

	if decodeError != nil {
		server.SendInternalServerError(decodeError, responseWriter)
		return
	}

	if decodedPayload.Event != "user.upgraded" {
		responseWriter.WriteHeader(http.StatusNoContent)
		return
	}

	if decodedPayload.Event == "user.upgraded" {

		_, getUserError := config.db.GetUserByID(req.Context(), decodedPayload.Data.UserID)

		if getUserError != nil {

			if strings.Contains(getUserError.Error(), "no rows in result set") {
				responseWriter.WriteHeader(http.StatusNotFound)
				responseWriter.Header().Set("Content-Type", "application/json")
				responseWriter.Write([]byte(`{"error": "user not found"}`))
				return
			}

			server.SendInternalServerError(getUserError, responseWriter)
			return
		}

		_, upgradeError := config.db.UpdateChirpyRedUser(req.Context(), database.UpdateChirpyRedUserParams{
			IsChirpyRed: sql.NullBool{Bool: true, Valid: true},
			ID:          decodedPayload.Data.UserID,
		})

		if upgradeError != nil {
			server.SendInternalServerError(upgradeError, responseWriter)
			return
		}

		responseWriter.WriteHeader(http.StatusNoContent)
		return
	}
}

func main() {
	godotenv.Load()
	dbURL := os.Getenv("DB_URL")
	polkaKey := os.Getenv("POLKA_KEY")
	jwtSecret := os.Getenv("JWT_SECRET")
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
		secret:         jwtSecret,
		polkaKey:       polkaKey,
	}

	mux := http.NewServeMux()

	handler := http.StripPrefix("/app", http.FileServer(http.Dir(filepathRoot)))

	mux.Handle("/app/", config.middlewareMetricsInc(handler))

	mux.HandleFunc("GET /api/healthz", config.healthHandler)
	mux.HandleFunc("POST /api/users", config.createUser)
	mux.HandleFunc("PUT /api/users", config.updateUser)
	mux.HandleFunc("GET /api/chirps", config.listChirps)
	mux.HandleFunc("GET /api/chirps/{id}", config.getChirpById)
	mux.HandleFunc("DELETE /api/chirps/{id}", config.deleteChirp)
	mux.HandleFunc("POST /api/chirps", config.createChirp)
	mux.HandleFunc("POST /api/login", config.login)
	mux.HandleFunc("POST /api/refresh", config.refreshSession)
	mux.HandleFunc("POST /api/revoke", config.revokeSession)
	mux.HandleFunc("POST /api/polka/webhooks", config.polkaWebhooks)

	mux.HandleFunc("GET /admin/metrics", config.metricsHandler)
	mux.HandleFunc("POST /admin/reset", config.resetMetricsHandler)

	server := http.Server{
		Addr:    port,
		Handler: mux,
	}

	log.Printf("Serving files from %s on port: %s\n", filepathRoot, port)

	log.Fatal(server.ListenAndServe())
}
