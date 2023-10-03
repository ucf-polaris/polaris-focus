// How to call the token create function from another function
package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"polaris-api/pkg/Helpers"

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
	Email  string `json:"email"`
	UserID string `json:"UserID"`
	Type   int    `json:"type"`
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

var codes []string = []string{
	"verificationCode",
	"resetCode",
}

func GetEmailWithUserID(UserID string) (string, error) {
	item := make(map[string]types.AttributeValue)

	item["UserID"] = &types.AttributeValueMemberS{Value: UserID}

	TheInput, err := client.GetItem(context.Background(), &dynamodb.GetItemInput{
		TableName:            aws.String(table),
		Key:                  item,
		ProjectionExpression: aws.String("email"),
	})

	//-----------------------------------------ERROR CHECKING-----------------------------------------
	//General error occured
	if err != nil {
		return "", err
	}
	//-----------------------------------------PACK RESULTS-----------------------------------------
	//get results in
	results := map[string]any{}
	attributevalue.UnmarshalMap(TheInput.Item, &results)

	email, emailOk := results["email"].(string)

	if !emailOk {
		return "", errors.New("no email field found")
	}

	return email, nil
}

func QueryCodes(email string, codeType int) (int, error) {
	item := make(map[string]types.AttributeValue)

	item[":email"] = &types.AttributeValueMemberS{Value: email}
	QueryInput, err := client.Query(context.Background(), &dynamodb.QueryInput{
		TableName:                 aws.String(table),
		ExpressionAttributeValues: item,
		IndexName:                 aws.String("email-index"),
		KeyConditionExpression:    aws.String("email = :email"),
		ProjectionExpression:      aws.String(codes[codeType%len(codes)]),
	})

	if err != nil {
		return -1, err
	}

	//no code returned
	if QueryInput.Count == 0 {
		return -1, errors.New("no code found")
	}

	ret := make(map[string]interface{})
	attributevalue.UnmarshalMap(QueryInput.Items[0], &ret)
	code := ret[codes[codeType%len(codes)]].(float64)

	return int(code), nil
}

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	token, refreshToken, err := Helpers.GetTokens(request)
	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}

	search := EmailQuery{}
	log.Println(request.Body)
	json.Unmarshal([]byte(request.Body), &search)
	//-----------------------------------------EMAIL OR USERID-----------------------------------------
	//get email if it doesn't exist
	if search.UserID != "" && search.Email == "" {
		search.Email, err = GetEmailWithUserID(search.UserID)
		if err != nil {
			return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
		}
	} else if search.UserID == "" && search.Email == "" {
		return Helpers.ResponseGeneration("Email and UserID doesn't exist", http.StatusOK)
	}
	//-----------------------------------------GET CODE WITH EMAIL-----------------------------------------
	code, err := QueryCodes(search.Email, search.Type)
	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}
	//-----------------------------------------PACKING RESULTS-----------------------------------------
	pre_js := make(map[string]interface{})
	pre_js["code"] = code
	pre_js["email"] = search.Email
	pre_js["type"] = search.Type

	js, err := json.Marshal(pre_js)
	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}

	result, err := lambdaClient.Invoke(&lambdaCall.InvokeInput{FunctionName: aws.String("email_code"), Payload: js})
	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}

	log.Println(result)

	ret := make(map[string]interface{})
	tokens := make(map[string]interface{})
	if token != "" {
		tokens["token"] = token
	}
	if refreshToken != "" {
		tokens["refreshToken"] = refreshToken
	}

	ret["tokens"] = tokens

	js, _ = json.Marshal(ret)

	return events.APIGatewayProxyResponse{StatusCode: http.StatusOK, Body: string(js), Headers: map[string]string{"content-type": "application/json"}}, nil

}
