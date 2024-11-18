package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	bcrypt "golang.org/x/crypto/bcrypt"
)

const cost = 10

func HashPassword(password string) (string, error) {
	hash, hashError := bcrypt.GenerateFromPassword([]byte(password), cost)

	if hashError != nil {
		log.Fatal("failed to hash user password")
	}

	return string(hash), nil
}

func CheckPasswordHash(password, hash string) error {
	compareError := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))

	if compareError != nil {
		fmt.Printf("%v\n", compareError)
		return errors.New("wrong credentials")
	}

	return nil
}

func MakeJWT(userID uuid.UUID, tokenSecret string, expiresIn time.Duration) (string, error) {
	claims := jwt.RegisteredClaims{
		Issuer:    "chirpy",
		IssuedAt:  &jwt.NumericDate{time.Now()},
		ExpiresAt: &jwt.NumericDate{time.Now().Add(expiresIn)},
		Subject:   userID.String(),
	}

	unsignedToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	token, signError := unsignedToken.SignedString([]byte(tokenSecret))

	if signError != nil {
		return "", signError
	}

	return token, nil
}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error) {

	token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(tokenSecret), nil
	})

	if err != nil {
		return uuid.UUID{}, errors.New("failed to decode token")
	}

	id, getTokenClaimsError := token.Claims.GetSubject()

	if getTokenClaimsError != nil {
		return uuid.UUID{}, errors.New("failed to decode token - sub")
	}

	uniqueId, fromBytesErr := uuid.Parse(id)

	if fromBytesErr != nil {
		return uuid.UUID{}, errors.New("failed to decode token - from bytes")
	}

	return uniqueId, nil
}

func GetBearerToken(headers http.Header) (string, error) {
	authHeader := headers.Get("Authorization")

	if len(authHeader) == 0 {
		return "", errors.New("no authorization header")
	}

	return strings.ReplaceAll(authHeader, "Bearer ", ""), nil
}

func MakeRefreshToken() (string, error) {
	c := 32
	b := make([]byte, c)
	_, err := rand.Read(b)
	if err != nil {
		fmt.Println("error:", err)
		return "", err
	}

	token := hex.EncodeToString(b)

	return token, nil
}
