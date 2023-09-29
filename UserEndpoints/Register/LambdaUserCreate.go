package main

import (
	"context"
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"polaris-api/pkg/Helpers"
	"strconv"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
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
	client, table = Helpers.ConstructDynamoHost()

	if table == "" {
		log.Fatal("missing environment variable TABLE_NAME")
	}

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

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	//-----------------------------------------EXTRACT FIELDS-----------------------------------------
	search := Helpers.UnpackRequest(request.Body)

	item, _, _, err := Helpers.ExtractFields(
		[]string{"email", "username", "password", "name", "schedule", "favorite", "visited"},
		search,
		false,
		false,
	)
	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}

	//-----------------------------------------FORMAT SCHEDULE-----------------------------------------
	Helpers.ListToStringSet(
		[]string{"schedule", "favorite", "visited"},
		item,
		false,
	)
	//-----------------------------------------EXTRACT FORMATTED EMAIL-----------------------------------------
	item_email, _, _, err := Helpers.ExtractFields(
		[]string{"email"},
		search,
		true,
		false,
	)

	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}

	//-----------------------------------------CHECK QUERY TO PREVENT DUPLICATE EMAILS-----------------------------------------
	TheInput, err := client.Query(context.Background(), &dynamodb.QueryInput{
		TableName:                 aws.String(table),
		IndexName:                 aws.String("email-index"),
		KeyConditionExpression:    aws.String("email = :email"),
		ExpressionAttributeValues: item_email,
	})

	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}

	if TheInput.Count != 0 {
		return Helpers.ResponseGeneration("email already in use", http.StatusOK)
	}

	//-----------------------------------------NEW USER CONSTRUCTION-----------------------------------------
	uuid_new := uuid.Must(uuid.NewRandom()).String()
	code := produceRandomNDigits(5)

	item["UserID"] = &types.AttributeValueMemberS{Value: uuid_new}
	item["timeTilExpire"] = &types.AttributeValueMemberN{Value: strconv.FormatInt(time.Now().UTC().Add(time.Minute*15).Unix(), 10)}
	item["verificationCode"] = &types.AttributeValueMemberN{Value: code}

	//-----------------------------------------PUT UNVERIFIED USER INTO DATABASE-----------------------------------------
	_, err = client.PutItem(context.Background(), &dynamodb.PutItemInput{
		TableName: aws.String(table),
		Item:      item,
	})
	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}

	//-----------------------------------------SEND EMAIL CODE-----------------------------------------
	body := make(map[string]interface{})
	body["email"] = search["email"].(string)
	body["code"], err = strconv.ParseFloat(code, 64)

	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}

	if !Helpers.IsLambdaLocal() {
		_, err = lambdaClient.Invoke(&lambdaCall.InvokeInput{FunctionName: aws.String("email_code"), Payload: payload})
		if err != nil {
			return Helpers.ResponseGeneration("email error: "+err.Error(), http.StatusOK)
		}
	}

	//log.Println(result.Payload)

	//-----------------------------------------CREATE TOKEN-----------------------------------------
	tokenRet, err := Helpers.CreateToken(lambdaClient, 15, uuid_new, 2)
	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}

	//-----------------------------------------PACK RETURN VALUES-----------------------------------------
	ret := make(map[string]interface{})
	user := make(map[string]interface{})

	user["UserID"] = uuid_new
	user["email"] = search["email"].(string)

	ret["token"] = tokenRet
	//put user fields in its own field (easier documentation)
	ret["User"] = user

	js, err := json.Marshal(ret)

	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}

	return events.APIGatewayProxyResponse{StatusCode: http.StatusOK, Body: string(js), Headers: map[string]string{"content-type": "application/json"}}, nil
}
