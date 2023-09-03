package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/golang-jwt/jwt/v4"
)

type Claims struct {
	//Username string `json:"username"`
	jwt.RegisteredClaims
}

type User struct {
	UserID       string   `json:"UserID"`
	Email        string   `json:"email"`
	Password     string   `json:"password,omitempty"`
	Schedule     []string `json:"schedule"`
	Username     string   `json:"username"`
	Name         string   `json:"name"`
	Token        string   `json:"token,omitempty"`
	RefreshToken string   `json:"refreshToken,omitempty"`
}

var table string
var tokenKey []byte
var secretKey []byte
var client *dynamodb.Client

func init() {
	table = os.Getenv("TABLE_NAME")

	if table == "" {
		log.Fatal("missing environment variable TABLE_NAME")
	}

	tokenKeyStr := os.Getenv("TOKEN_KEY")
	secretKeyStr := os.Getenv("SECRET_KEY")

	tokenKey = []byte(tokenKeyStr)
	secretKey = []byte(secretKeyStr)

	cfg, _ := config.LoadDefaultConfig(context.Background())
	client = dynamodb.NewFromConfig(cfg)
}

func main() {
	lambda.Start(handler)
}

func unpackRequest(body string) User {
	if body == "" {
		return User{}
	}

	log.Println("body: ", body)

	search := User{}
	err := json.Unmarshal([]byte(body), &search)

	if err != nil {
		panic(err)
	}

	return search
}

func responseGeneration(msg string, status int) (events.APIGatewayProxyResponse, error) {
	return events.APIGatewayProxyResponse{StatusCode: status, Body: "Error: " + msg}, errors.New(msg)
}

func generateJWT(timeTil int, key []byte) (string, error) {
	//The claims
	expirationTime := time.Now().UTC().Add(time.Duration(timeTil) * time.Minute)

	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS512, claims)
	tokenString, err := token.SignedString(key)
	if err != nil {
		panic(err)
	}
	return tokenString, nil
}

// TO-DO: create a function that handles response returns (more clean and more info/debug info)
func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	//-----------------------------------------PREPARATION-----------------------------------------
	//get the body
	search := unpackRequest(request.Body)

	//error check username and pass
	if search.Username == "" || search.Password == "" {
		return responseGeneration("field not set", http.StatusBadRequest)
	}
	//-----------------------------------------THE QUERY-----------------------------------------
	//pass parameters into query
	item_username := make(map[string]types.AttributeValue)
	item_username[":username"] = &types.AttributeValueMemberS{Value: search.Username}

	//the query
	QueryResults, err := client.Query(context.Background(), &dynamodb.QueryInput{
		TableName:                 aws.String(table),
		IndexName:                 aws.String("username-index"),
		KeyConditionExpression:    aws.String("username = :username"),
		ExpressionAttributeValues: item_username,
	})
	//-----------------------------------------ERROR CHECKING-----------------------------------------
	//General error occured
	if err != nil {
		return responseGeneration(err.Error(), http.StatusBadRequest)
	}

	//No username found
	if QueryResults.Count == 0 {
		return responseGeneration("invalid username/password", http.StatusBadRequest)
	}

	//More than one username found (shouldn't happen, but could)
	if QueryResults.Count > 1 {
		return responseGeneration("more than one username found", http.StatusBadRequest)
	}
	//-----------------------------------------PACKING RESULTS-----------------------------------------
	//get results in
	newUser := User{}
	attributevalue.UnmarshalMap(QueryResults.Items[0], &newUser)

	//store and hide the password
	check_pass := newUser.Password
	newUser.Password = ""

	//-----------------------------------------TOKEN-----------------------------------------
	//make and return token and refresh token
	tkn, err := generateJWT(15, tokenKey)
	if err != nil {
		return responseGeneration(err.Error(), http.StatusBadRequest)
	}

	rfs, err := generateJWT(1440, secretKey)
	if err != nil {
		return responseGeneration(err.Error(), http.StatusBadRequest)
	}

	newUser.Token = tkn
	newUser.RefreshToken = rfs

	//package the results
	js, err := json.Marshal(newUser)

	if err != nil {
		return responseGeneration(err.Error(), http.StatusBadRequest)
	}

	//checking the password, if nothing return error
	if check_pass != search.Password {
		return responseGeneration("invalid username/password", http.StatusBadRequest)
	}

	return events.APIGatewayProxyResponse{StatusCode: http.StatusOK, Body: string(js), Headers: map[string]string{"content-type": "application/json"}}, nil
}
