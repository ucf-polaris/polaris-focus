// Function used to generate Token (will be invoked from other AWS Lambda Functions)
package main

import (
	"errors"
	"log"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/golang-jwt/jwt/v4"
)

type Claims struct {
	UserID string `json:"userID,omitempty"`
	jwt.RegisteredClaims
}

var secretKey []byte
var refreshKey []byte
var registerKey []byte

func init() {
	key := os.Getenv("SECRET_KEY")

	if key == "" {
		log.Fatal("missing environment variable SECRET_KEY")
	}

	keyR := os.Getenv("REFRESH_KEY")

	if keyR == "" {
		log.Fatal("missing environment variable REFRESH_KEY")
	}

	keyReg := os.Getenv("REGISTER_KEY")

	if keyR == "" {
		log.Fatal("missing environment variable REFRESH_KEY")
	}

	registerKey = []byte(keyReg)
	refreshKey = []byte(keyR)
	secretKey = []byte(key)

}

func main() {
	lambda.Start(handler)
}

func generateJWT(timeExp int, id string, signing []byte) (string, error) {
	//The claims (24 hours till expire)
	expirationTime := time.Now().UTC().Add(time.Duration(timeExp) * time.Minute)

	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
		UserID: id,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS512, claims)
	tokenString, err := token.SignedString(signing)
	if err != nil {
		panic(err)
	}
	return tokenString, nil
}

func handler(body map[string]interface{}) (map[string]interface{}, error) {
	//-----------------------------------------EXTRACT FIELDS-----------------------------------------
	var extraStr string
	var mode float64
	var signing []byte

	if _, ok := (body["timeTil"].(float64)); !ok {
		return nil, errors.New("timeTil field not found")
	}

	if val, ok := (body["UserID"].(string)); ok {
		extraStr = val
	} else {
		extraStr = ""
	}

	if val, ok := (body["mode"].(float64)); ok {
		mode = val
	} else {
		mode = 0
	}

	switch mode {
	//mode 1 = refresh key
	case 1:
		log.Println("refresh key!")
		signing = refreshKey
	//mode 2 = register key
	case 2:
		log.Println("register key!")
		signing = registerKey
	//mode 0 or other = secret key
	default:
		log.Println("secret key!")
		signing = secretKey
	}

	//-----------------------------------------GET TOKEN-----------------------------------------
	token, err := generateJWT(int(body["timeTil"].(float64)), extraStr, signing)
	if err != nil {
		return make(map[string]interface{}), err
	}

	response := make(map[string]interface{})
	response["token"] = token

	return response, nil
}
