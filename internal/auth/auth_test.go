package auth

import (
	"fmt"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestMakeJWT(t *testing.T) {
	id := "c9e88594-f26f-496f-bb20-192ed5cc80ba"
	userID, _ := uuid.Parse(id)
	secret := "super_secret"
	expiresIn := time.Duration(1 * float64(time.Hour))

	token, err := MakeJWT(userID, secret, expiresIn)

	tokenType := reflect.TypeOf(token).Kind()

	if !(tokenType == reflect.String) {
		t.Fatalf("MakeJWT failed - token type is not of type string\n")
	}

	if err != nil {
		fmt.Printf("%v\n", err)
		t.Fatalf("MakeJWT failed - throwed error\n")
	}
}

func TestValidateJWT(t *testing.T) {
	id := "c9e88594-f26f-496f-bb20-192ed5cc80ba"
	userID, _ := uuid.Parse(id)
	secret := "super_secret"
	expiresIn := time.Duration(1 * float64(time.Hour))

	token, err := MakeJWT(userID, secret, expiresIn)

	if err != nil {
		fmt.Printf("%v\n", err)
		t.Fatalf("MakeJWT failed - throwed error\n")
	}

	userUUID, validateError := ValidateJWT(token, secret)

	if validateError != nil {
		t.Fatalf("ValidateJWT failed - throwed error:\n")
		fmt.Printf("%v\n", validateError)
	}

	if userUUID.String() != id {
		t.Fatalf("ValidateJWT failed - ids do not match\n")
	}
}

func TestGetBearerToken(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer token")

	token, err := GetBearerToken(req.Header)

	if err != nil {
		t.Fatalf("GetBearerToken failed - %v\n", err)
	}

	if token != "token" {
		t.Fatalf("GetBearerToken failed - tokens do not match\n")
	}
}
