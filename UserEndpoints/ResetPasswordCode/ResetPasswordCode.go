package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"polaris-api/pkg/Helpers"
	"strconv"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	lambdaCall "github.com/aws/aws-sdk-go/service/lambda"
	"github.com/golang-jwt/jwt/v4"
)

type Claims struct {
	UserID string `json:"userID,omitempty"`
	jwt.RegisteredClaims
}

type CodeQuery struct {
	UserID string `json:"UserID"`
	Code   int    `json:"code,omitempty"`
}

var table string
var client *dynamodb.Client
var lambdaClient *lambdaCall.Lambda

func init() {
	//dynamo stuff
	client, table = Helpers.ConstructDynamoHost()

	//set up lambda client
	//create session for lambda
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	lambdaClient = lambdaCall.New(sess, &aws.Config{Region: aws.String("us-east-2")})

}

func main() {
	lambda.Start(handler)
}

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	//-----------------------------------------EXTRACT FIELDS-----------------------------------------
	var contextString string
	query := CodeQuery{}
	ctxt := make(map[string]interface{})

	err := json.Unmarshal([]byte(request.Body), &query)
	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusBadRequest)
	}

	if request.RequestContext.Authorizer != nil {
		contextString = request.RequestContext.Authorizer["stringKey"].(string)
		log.Println(contextString)
	}

	json.Unmarshal([]byte(contextString), &ctxt)

	//look for email from JWT first, if not there look in passed in body
	if val, ok := ctxt["UserID"].(string); ok && query.UserID == "" {
		query.UserID = val
	}

	if query.Code == 0 {
		return Helpers.ResponseGeneration("code field not set", http.StatusOK)
	}

	codeStr := strconv.Itoa(query.Code)
	//-----------------------------------------VAIDATE USER-----------------------------------------
	if err := CheckIfValid(query.UserID); err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}
	//-----------------------------------------THE UPDATE CALL-----------------------------------------
	//pass changes into update
	item := make(map[string]types.AttributeValue)
	item[":code"] = &types.AttributeValueMemberN{Value: codeStr}

	//who we're trying to find
	key := make(map[string]types.AttributeValue)
	key["UserID"] = &types.AttributeValueMemberS{Value: query.UserID}

	input := &dynamodb.UpdateItemInput{
		ExpressionAttributeValues: item,
		TableName:                 aws.String(table),
		Key:                       key,
		UpdateExpression:          aws.String("remove resetCode, resetRequestExpireTime"),
		ConditionExpression:       aws.String("resetCode = :code"),
	}

	_, err = client.UpdateItem(context.Background(), input)
	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}

	//-----------------------------------------RESULTS PROCESSING-----------------------------------------
	map_output := make(map[string]interface{})
	ret := make(map[string]interface{})
	ret["success"] = true
	//-----------------------------------------CREATE TOKENS-----------------------------------------
	map_output["token"], err = Helpers.CreateToken(lambdaClient, 15, "", 0)
	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}

	map_output["refreshToken"], err = Helpers.CreateToken(lambdaClient, 1440, "", 1)
	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}

	ret["tokens"] = map_output

	js, err := json.Marshal(ret)
	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}

	return events.APIGatewayProxyResponse{StatusCode: http.StatusOK, Body: string(js), Headers: map[string]string{"content-type": "application/json"}}, nil
}