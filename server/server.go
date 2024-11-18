package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func SendInternalServerError(payload any, responseWriter http.ResponseWriter) {
	responseWriter.WriteHeader(http.StatusInternalServerError)
	responseWriter.Header().Set("Content-Type", "application/json")
	message := fmt.Sprintf(`{"message": "Something went wrong", "error":"%v"}`, payload)
	responseWriter.Write([]byte(message))
}

func ResponseWithJson(payload any, status int, responseWriter http.ResponseWriter) {
	response, encodeError := json.Marshal(payload)

	if encodeError != nil {
		SendInternalServerError(payload, responseWriter)
		return
	}

	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(status)
	responseWriter.Write(response)
}

func DecodeBody[T any](body io.ReadCloser) (T, error) {
	decoder := json.NewDecoder(body)
	var payload T

	decodeError := decoder.Decode(&payload)

	if decodeError != nil {
		return payload, decodeError
	}

	return payload, nil
}
