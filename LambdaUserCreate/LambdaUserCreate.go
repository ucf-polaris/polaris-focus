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
	"github.com/aws/aws-sdk-go/aws/session"
	lambdaCall "github.com/aws/aws-sdk-go/service/lambda"
	"github.com/google/uuid"
)

var table string
var client *dynamodb.Client
var lambdaClient *lambdaCall.Lambda

func init() {
	table = os.Getenv("TABLE_NAME")

	if table == "" {
		log.Fatal("missing environment variable TABLE_NAME")
	}

	//create session for dynamodb
	cfg, _ := config.LoadDefaultConfig(context.Background())
	client = dynamodb.NewFromConfig(cfg)

	//create session for lambda
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	lambdaClient = lambdaCall.New(sess, &aws.Config{Region: aws.String("us-east-2")})
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

func createToken(timeTil int, userID string, mode float64) (string, error) {
	//-----------------------------------------GET VARIABLES-----------------------------------------
	JWTFields := make(map[string]interface{})

	JWTFields["timeTil"] = timeTil
	JWTFields["mode"] = mode

	if userID != "" {
		JWTFields["UserID"] = userID
	}
	//-----------------------------------------PACKAGE RESPONSE-----------------------------------------
	payload, err := json.Marshal(JWTFields)
	if err != nil {
		return "", err
	}

	resultToken, err := lambdaClient.Invoke(&lambdaCall.InvokeInput{FunctionName: aws.String("token_create"), Payload: payload})
	if err != nil {
		return "", err
	}

	result_json := unpackRequest(string(resultToken.Payload))

	token := result_json["token"].(string)

	return token, nil
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
	var outputError string

	valUser, okUser := search["username"].(string)
	valPass, okPass := search["password"].(string)
	valEmail, okEmail := search["email"].(string)
	valTempPass, okTemp := search["temp_pass"].(string)

	if !okUser || !okPass || !okEmail || !okTemp {
		if !okUser {
			outputError += "user "
		}

		if !okPass {
			outputError += "password "
		}

		if !okEmail {
			outputError += "email "
		}

		if !okTemp {
			outputError += "temp pass "
		}

		return responseGeneration("field not set ("+outputError+")", http.StatusBadRequest)
	}

	item_email := make(map[string]types.AttributeValue)
	item_email[":email"] = &types.AttributeValueMemberS{Value: valEmail}

	if valTempPass != "potato" {
		return responseGeneration("temp pass wrong", http.StatusBadRequest)
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
	code := produceRandomNDigits(5)

	item["UserID"] = &types.AttributeValueMemberS{Value: uuid_new}
	item["email"] = &types.AttributeValueMemberS{Value: valEmail}
	item["password"] = &types.AttributeValueMemberS{Value: valPass}
	item["username"] = &types.AttributeValueMemberS{Value: valUser}
	item["name"] = &types.AttributeValueMemberS{Value: valName}
	item["timeTilExpire"] = &types.AttributeValueMemberN{Value: strconv.FormatInt(time.Now().UTC().Add(time.Minute*15).Unix(), 10)}
	item["verificationCode"] = &types.AttributeValueMemberN{Value: code}

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

	//-----------------------------------------SEND EMAIL CODE-----------------------------------------
	body := make(map[string]interface{})
	body["email"] = valEmail
	body["code"], err = strconv.ParseFloat(code, 64)

	if err != nil {
		return responseGeneration(err.Error(), http.StatusBadRequest)
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return responseGeneration(err.Error(), http.StatusBadRequest)
	}

	result, err := lambdaClient.Invoke(&lambdaCall.InvokeInput{FunctionName: aws.String("email_code"), Payload: payload})
	if err != nil {
		return responseGeneration("email error: "+err.Error(), http.StatusBadRequest)
	}

	log.Println(result.Payload)

	//-----------------------------------------CREATE TOKEN-----------------------------------------
	tokenRet, err := createToken(15, uuid_new, 2)
	if err != nil {
		return responseGeneration(err.Error(), http.StatusBadRequest)
	}

	//-----------------------------------------PACK RETURN VALUES-----------------------------------------
	ret := make(map[string]interface{})
	ret["token"] = tokenRet

	ret["UserID"] = uuid_new

	ret["email"] = valEmail

	js, err := json.Marshal(ret)

	if err != nil {
		return responseGeneration(err.Error(), http.StatusBadRequest)
	}

	return events.APIGatewayProxyResponse{StatusCode: http.StatusOK, Body: string(js), Headers: map[string]string{"content-type": "application/json"}}, nil
}
