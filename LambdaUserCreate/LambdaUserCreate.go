package main

import (
	"context"
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
)

type User struct {
	UserID   string   `json:"UserID"`
	Email    string   `json:"email"`
	Password string   `json:"password"`
	Schedule []string `json:"schedule"`
	Username string   `json:"username"`
	Name     string   `json:"name"`
}

type Claims struct {
	UserID string `json:"userID"`
	jwt.RegisteredClaims
}

var table string
var secretKey []byte
var client *dynamodb.Client

func init() {
	table = os.Getenv("TABLE_NAME")

	if table == "" {
		log.Fatal("missing environment variable TABLE_NAME")
	}

	key := os.Getenv("SECRET_KEY")

	if key == "" {
		log.Fatal("missing environment variable SECRET_KEY")
	}

	secretKey = []byte(key)

	cfg, _ := config.LoadDefaultConfig(context.Background())
	client = dynamodb.NewFromConfig(cfg)
}

func main() {
	lambda.Start(handler)
}

func produceRandomNDigits(N int) string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	var number string

	for i := 0; i < N; i++ {
		digit := r.Intn(10)
		number += strconv.Itoa(digit)
	}

	return number
}

func generateJWT(timeTil int, addOn string) (string, error) {
	//The claims
	expirationTime := time.Now().UTC().Add(time.Duration(timeTil) * time.Minute)

	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
		UserID: addOn,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS512, claims)
	tokenString, err := token.SignedString(secretKey)
	if err != nil {
		panic(err)
	}
	return tokenString, nil
}

func responseGeneration(msg string, status int) (events.APIGatewayProxyResponse, error) {
	return events.APIGatewayProxyResponse{StatusCode: status, Body: "Error: " + msg}, nil
}

func unpackRequest(body string) map[string]interface{} {
	if body == "" {
		return nil
	}

	log.Println("body: ", body)

	search := map[string]any{}
	err := json.Unmarshal([]byte(body), &search)

	if err != nil {
		panic(err)
	}

	return search
}

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	//-----------------------------------------EXTRACT REQUIRED FIELDS-----------------------------------------
	search := unpackRequest(request.Body)

	valUser, okUser := search["username"].(string)
	valPass, okPass := search["password"].(string)
	valEmail, okEmail := search["email"].(string)
	valTempPass, okTemp := search["temp_pass"].(string)

	if !okUser || !okPass || !okEmail || !okTemp {
		return responseGeneration("field not set", http.StatusBadRequest)
	}

	item_email := make(map[string]types.AttributeValue)
	item_email[":email"] = &types.AttributeValueMemberS{Value: valEmail}

	if valTempPass != "potato" {
		return responseGeneration("field not set", http.StatusBadRequest)
	}

	//-----------------------------------------EXTRACT NONREQUIRED FIELDS-----------------------------------------
	var valName string
	var valSchedule []interface{}
	var okName, okSchedule bool

	if valName, okName = search["name"].(string); !okName {
		valName = ""
	}
	if valSchedule, okSchedule = search["schedule"].([]interface{}); !okSchedule {
		valSchedule = make([]interface{}, 0)
	}

	//-----------------------------------------CHECK QUERY TO PREVENT DUPLICATE EMAILS-----------------------------------------
	TheInput, err := client.Query(context.Background(), &dynamodb.QueryInput{
		TableName:                 aws.String(table),
		IndexName:                 aws.String("email-index"),
		KeyConditionExpression:    aws.String("email = :email"),
		ExpressionAttributeValues: item_email,
	})

	if err != nil {
		panic(err)
	}

	if TheInput.Count != 0 {
		return responseGeneration("email already in use", http.StatusBadRequest)
	}

	//-----------------------------------------NEW USER CONSTRUCTION-----------------------------------------
	item := make(map[string]types.AttributeValue)
	uuid_new := uuid.Must(uuid.NewRandom()).String()

	item["UserID"] = &types.AttributeValueMemberS{Value: uuid_new}
	item["email"] = &types.AttributeValueMemberS{Value: valEmail}
	item["password"] = &types.AttributeValueMemberS{Value: valPass}
	item["username"] = &types.AttributeValueMemberS{Value: valUser}
	item["name"] = &types.AttributeValueMemberS{Value: valName}
	item["timeTilExpire"] = &types.AttributeValueMemberN{Value: strconv.FormatInt(time.Now().UTC().Add(time.Minute*15).Unix(), 10)}
	item["verificationCode"] = &types.AttributeValueMemberN{Value: produceRandomNDigits(5)}

	//put list of strings into correct format
	av, err := attributevalue.MarshalList(valSchedule)

	if err != nil {
		return responseGeneration(err.Error(), http.StatusBadRequest)
	}

	item["schedule"] = &types.AttributeValueMemberL{Value: av}

	//-----------------------------------------PUT UNVERIFIED USER INTO DATABASE-----------------------------------------
	_, err = client.PutItem(context.Background(), &dynamodb.PutItemInput{
		TableName: aws.String(table),
		Item:      item,
	})
	if err != nil {
		return responseGeneration(err.Error(), http.StatusBadRequest)
	}

	//-----------------------------------------RETURN FIELDS-----------------------------------------
	ret := make(map[string]interface{})

	ret["UserID"] = uuid_new
	ret["token"], err = generateJWT(15, uuid_new)

	if err != nil {
		return responseGeneration(err.Error(), http.StatusBadRequest)
	}

	js, err := json.Marshal(ret)

	if err != nil {
		return responseGeneration(err.Error(), http.StatusBadRequest)
	}

	return events.APIGatewayProxyResponse{StatusCode: http.StatusOK, Body: string(js), Headers: map[string]string{"content-type": "application/json"}}, nil
}
