// Function used to generate Token (will be invoked from other AWS Lambda Functions)
package main

import (
	"log"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/golang-jwt/jwt/v4"
)

type Claims struct {
	//Username string `json:"username"`
	jwt.RegisteredClaims
}

type JWTConstructor struct {
	TimeTil int `json:"timeTil"`
}

type JWTResponse struct {
	Token string `json:"token"`
}

var secretKey []byte

func init() {
	key := os.Getenv("SECRET_KEY")

	if key == "" {
		log.Fatal("missing environment variable SECRET_KEY")
	}
	secretKey = []byte(key)
}

func main() {
	lambda.Start(handler)
}

func generateJWT(timeExp int) (string, error) {
	//The claims (24 hours till expire)
	expirationTime := time.Now().UTC().Add(time.Duration(timeExp) * time.Minute)

	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS512, claims)
	tokenString, err := token.SignedString(secretKey)
	if err != nil {
		panic(err)
	}
	return tokenString, nil
}

func handler(payload JWTConstructor) (JWTResponse, error) {
	token, err := generateJWT(payload.TimeTil)
	if err != nil {
		return JWTResponse{}, err
	}
	response := JWTResponse{Token: token}
	return response, nil
}
