package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	lambdaCall "github.com/aws/aws-sdk-go/service/lambda"
)

var table string
var client *dynamodb.Client
var lambdaClient *lambdaCall.Lambda

func init() {
	//dynamoDB
	table = os.Getenv("TABLE_NAME")

	if table == "" {
		log.Fatal("missing environment variable TABLE_NAME")
	}

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
	return events.APIGatewayProxyResponse{StatusCode: status, Body: "Error: " + msg}, errors.New(msg)
}

func ConstructVerified(queryResponse map[string]interface{}, password string) (string, error) {
	//store and hide the password
	check_pass, ok := queryResponse["password"].(string)
	if !ok {
		return "", errors.New("query returned no password field")
	}

	//checking the password, if nothing return error
	if check_pass != password {
		return "", errors.New("invalid username/password")
	}

	delete(queryResponse, "password")

	//-----------------------------------------TOKEN-----------------------------------------
	//make and return token and refresh token
	tkn, err := createToken(15, "", 0)
	if err != nil {
		return "", err
	}

	rfs, err := createToken(1440, "", 1)
	if err != nil {
		return "", err
	}

	queryResponse["token"] = tkn
	queryResponse["refreshToken"] = rfs

	//package the results
	js, err := json.Marshal(queryResponse)
	if err != nil {
		return "", err
	}

	return string(js), nil
}

func ConstructNonVerified(queryResponse map[string]interface{}) (string, error) {
	val, okID := queryResponse["UserID"].(string)
	if !okID {
		return "", errors.New("ID field not found")
	}

	valEmail, okEmail := queryResponse["email"].(string)
	if !okEmail {
		return "", errors.New("email field not found")
	}

	newResponse := make(map[string]interface{})
	newResponse["UserID"] = val

	newResponse["email"] = valEmail

	regtkn, err := createToken(15, val, 2)
	if err != nil {
		return "", err
	}

	newResponse["token"] = regtkn

	js, err := json.Marshal(newResponse)
	if err != nil {
		return "", err
	}

	return string(js), nil
}

// TO-DO: create a function that handles response returns (more clean and more info/debug info)
func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	//-----------------------------------------PREPARATION-----------------------------------------
	//get the body
	search := unpackRequest(request.Body)

	//field checking and extract username and password fields
	var email string
	var password string

	if val, ok := search["password"].(string); ok {
		password = val
	}

	if val, ok := search["email"].(string); ok {
		email = val
	}

	//error check username and pass
	if email == "" || password == "" {
		return responseGeneration("field not set", http.StatusBadRequest)
	}
	//-----------------------------------------THE QUERY-----------------------------------------
	//pass parameters into query
	item_username := make(map[string]types.AttributeValue)
	item_username[":email"] = &types.AttributeValueMemberS{Value: email}

	//the query
	QueryResults, err := client.Query(context.Background(), &dynamodb.QueryInput{
		TableName:                 aws.String(table),
		IndexName:                 aws.String("email-index"),
		KeyConditionExpression:    aws.String("email = :email"),
		ExpressionAttributeValues: item_username,
	})
	//-----------------------------------------ERROR CHECKING-----------------------------------------
	//General error occured
	if err != nil {
		return responseGeneration(err.Error(), http.StatusBadRequest)
	}

	//No username found
	if QueryResults.Count == 0 {
		return responseGeneration("invalid email/password", http.StatusBadRequest)
	}

	//More than one username found (shouldn't happen, but could)
	if QueryResults.Count > 1 {
		return responseGeneration("more than one email found", http.StatusBadRequest)
	}
	//-----------------------------------------PACKING RESULTS-----------------------------------------
	//get results in
	newUser := map[string]any{}
	attributevalue.UnmarshalMap(QueryResults.Items[0], &newUser)

	var ret string

	//user not verified
	if _, ok := newUser["verificationCode"].(float64); ok {
		ret, err = ConstructNonVerified(newUser)

		if err != nil {
			return responseGeneration(err.Error(), http.StatusBadRequest)
		}
		//user verified
	} else {
		ret, err = ConstructVerified(newUser, password)
		if err != nil {
			return responseGeneration(err.Error(), http.StatusBadRequest)
		}
	}

	return events.APIGatewayProxyResponse{StatusCode: http.StatusOK, Body: ret, Headers: map[string]string{"content-type": "application/json"}}, nil
}
