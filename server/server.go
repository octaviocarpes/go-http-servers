package server

import (
	"encoding/json"
	"log"
	"net/http"
)

func SendInternalServerError(payload any, responseWriter http.ResponseWriter) {
	log.Printf("Error encoding parameters: %v", payload)
	responseWriter.WriteHeader(http.StatusInternalServerError)
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.Write([]byte(`{"error": "Something went wrong"}`))
}

func ResponseWithJson(payload any, status int, responseWriter http.ResponseWriter) {
	response, encodeError := json.Marshal(payload)

	if encodeError != nil {
		SendInternalServerError(payload, responseWriter)
		return
	}

	responseWriter.WriteHeader(status)
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.Write(response)
}
