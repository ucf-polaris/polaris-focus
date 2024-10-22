package main

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand"
	"net/http"
	"polaris-api/pkg/Helpers"
	"strconv"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	lambdaCall "github.com/aws/aws-sdk-go/service/lambda"
)

var lambdaClient *lambdaCall.Lambda
var table string
var client *dynamodb.Client

type EmailQuery struct {
	Email string `json:"email"`
}

func init() {
	//dynamo db
	client, table = Helpers.ConstructDynamoHost()

	//lambda stuff
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))

	lambdaClient = lambdaCall.New(sess, &aws.Config{Region: aws.String("us-east-2")})
}

func main() {
	lambda.Start(handler)
}

func GetUserIDfromEmail(email string) (string, error) {
	item := make(map[string]types.AttributeValue)

	item[":email"] = &types.AttributeValueMemberS{Value: email}
	QueryInput, err := client.Query(context.Background(), &dynamodb.QueryInput{
		TableName:                 aws.String(table),
		ExpressionAttributeValues: item,
		IndexName:                 aws.String("email-index"),
		KeyConditionExpression:    aws.String("email = :email"),
		ProjectionExpression:      aws.String("UserID"),
	})

	if err != nil {
		return "", err
	}

	//no code returned
	if QueryInput.Count == 0 {
		return "", errors.New("no email found")
	}

	ret := make(map[string]interface{})
	attributevalue.UnmarshalMap(QueryInput.Items[0], &ret)
	userid := ret["UserID"].(string)

	return userid, nil
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
	//-----------------------------------------UNPACK REQUEST-----------------------------------------
	search := EmailQuery{}
	json.Unmarshal([]byte(request.Body), &search)
	//-----------------------------------------PREPARE FIELDS-----------------------------------------
	code := produceRandomNDigits(5)
	//set 15 minutes to verify code (checked by other endpoint)
	timeFrame := time.Now().UTC().Add(time.Minute * 15).Unix()

	if search.Email == "" {
		return Helpers.ResponseGeneration("Empty email field", http.StatusOK)
	}

	UserID, err := GetUserIDfromEmail(search.Email)
	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}

	key := map[string]types.AttributeValue{
		"UserID": &types.AttributeValueMemberS{Value: UserID},
	}

	item := map[string]types.AttributeValue{
		":UserID":                 &types.AttributeValueMemberS{Value: UserID},
		":resetCode":              &types.AttributeValueMemberS{Value: code},
		":resetRequestExpireTime": &types.AttributeValueMemberN{Value: strconv.FormatInt(timeFrame, 10)},
	}

	//-----------------------------------------UPDATE DATABASE TO HAVE FIELDS-----------------------------------------
	input := &dynamodb.UpdateItemInput{
		ExpressionAttributeValues: item,
		TableName:                 aws.String(table),
		Key:                       key,
		ReturnValues:              types.ReturnValueUpdatedNew,
		UpdateExpression:          aws.String("SET resetCode = :resetCode, resetRequestExpireTime = :resetRequestExpireTime"),
		ConditionExpression:       aws.String("UserID = :UserID"),
	}

	_, err = client.UpdateItem(context.Background(), input)
	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusBadRequest)
	}

	//-----------------------------------------PACK BODY FOR EMAIL TEMPLATE-----------------------------------------
	pre_js := map[string]interface{}{
		"code":  code,
		"email": search.Email,
		"type":  1.0,
	}

	js, err := json.Marshal(pre_js)
	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}

	_, err = lambdaClient.Invoke(&lambdaCall.InvokeInput{FunctionName: aws.String("email_code"), Payload: js})
	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}

	//-----------------------------------------CREATE REGISTRATION TOKEN-----------------------------------------
	tokenRet, err := Helpers.CreateToken(lambdaClient, 15, UserID, 2)
	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}

	//-----------------------------------------PACK RESPONSE-----------------------------------------
	ret := make(map[string]interface{})
	ret["tokens"] = map[string]string{
		"token": tokenRet,
	}
	ret["UserID"] = UserID

	js, err = json.Marshal(ret)
	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}

	return events.APIGatewayProxyResponse{StatusCode: http.StatusOK, Body: string(js), Headers: map[string]string{"content-type": "application/json"}}, nil
}
