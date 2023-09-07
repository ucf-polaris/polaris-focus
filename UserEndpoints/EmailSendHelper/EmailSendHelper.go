// How to call the token create function from another function
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

var lambdaClient *lambdaCall.Lambda
var table string
var client *dynamodb.Client

func init() {
	//dynamo db
	table = os.Getenv("TABLE_NAME")

	if table == "" {
		log.Fatal("missing environment variable TABLE_NAME")
	}

	cfg, _ := config.LoadDefaultConfig(context.Background())
	client = dynamodb.NewFromConfig(cfg)

	//lambda stuff
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))

	lambdaClient = lambdaCall.New(sess, &aws.Config{Region: aws.String("us-east-2")})
}

func main() {
	lambda.Start(handler)
}

func responseGeneration(msg string, status int) (events.APIGatewayProxyResponse, error) {
	return events.APIGatewayProxyResponse{StatusCode: status, Body: "Error: " + msg}, nil
}

func unpackRequest(body string) map[string]interface{} {
	if body == "" {
		return make(map[string]interface{})
	}

	log.Println("body: ", body)

	search := make(map[string]interface{})
	err := json.Unmarshal([]byte(body), &search)

	if err != nil {
		panic(err)
	}

	return search
}

func queryWithEmail(email string) (map[string]interface{}, error) {
	//-----------------------------------------THE QUERY (if email exists)-----------------------------------------
	//pass parameters into query
	item_username := make(map[string]types.AttributeValue)
	item_username[":email"] = &types.AttributeValueMemberS{Value: email}

	//the query
	QueryResults, err := client.Query(context.Background(), &dynamodb.QueryInput{
		TableName:                 aws.String(table),
		IndexName:                 aws.String("email-index"),
		KeyConditionExpression:    aws.String("email = :email"),
		ProjectionExpression:      aws.String("verificationCode"),
		ExpressionAttributeValues: item_username,
	})

	//-----------------------------------------ERROR CHECKING-----------------------------------------
	//General error occured
	if err != nil {
		return nil, err
	}

	//No email found
	if QueryResults.Count == 0 {
		return nil, errors.New("no email found")
	}

	//More than one email found (shouldn't happen, but could)
	if QueryResults.Count > 1 {
		return nil, errors.New("more than one email found")
	}
	//-----------------------------------------PACK RESULTS-----------------------------------------
	//get results in
	results := map[string]any{}
	attributevalue.UnmarshalMap(QueryResults.Items[0], &results)

	_, codeOk := results["verificationCode"].(float64)

	if !codeOk {
		return nil, errors.New("no verificationCode field found")
	}

	return results, nil
}
func getWithUserID(UserID string) (map[string]interface{}, error) {
	item := make(map[string]types.AttributeValue)

	item["UserID"] = &types.AttributeValueMemberS{Value: UserID}

	TheInput, err := client.GetItem(context.Background(), &dynamodb.GetItemInput{
		TableName:            aws.String(table),
		Key:                  item,
		ProjectionExpression: aws.String("verificationCode, email"),
	})

	//-----------------------------------------ERROR CHECKING-----------------------------------------
	//General error occured
	if err != nil {
		return nil, err
	}
	//-----------------------------------------PACK RESULTS-----------------------------------------
	//get results in
	results := map[string]any{}
	attributevalue.UnmarshalMap(TheInput.Item, &results)

	_, emailOk := results["email"].(string)
	_, codeOk := results["verificationCode"].(float64)

	if !emailOk {
		return nil, errors.New("no email field found")
	}

	if !codeOk {
		return nil, errors.New("no verificationCode field found")
	}

	return results, nil
}

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	body := unpackRequest(request.Body)

	var email string
	var userID string
	var contextString string

	if val, ok := body["email"].(string); ok {
		email = val
	} else {
		email = ""
	}

	if val, ok := body["UserID"].(string); ok {
		userID = val
	} else {
		userID = ""
	}

	if request.RequestContext.Authorizer != nil {
		contextString = request.RequestContext.Authorizer["stringKey"].(string)
		log.Println(contextString)
		ctxt := map[string]any{}
		json.Unmarshal([]byte(contextString), &ctxt)

		if val, ok := ctxt["UserID"].(string); ok {
			userID = val
		}
	}

	if userID == "" && email == "" {
		return responseGeneration("email and userID fields both empty", http.StatusBadRequest)
	}
	//-----------------------------------------EMAIL OR USERID-----------------------------------------
	var code float64

	if userID != "" {
		result, errs := getWithUserID(userID)

		if errs != nil {
			return responseGeneration(errs.Error(), http.StatusBadRequest)
		}

		email = result["email"].(string)
		code = result["verificationCode"].(float64)

	} else if email != "" {
		result, errs := queryWithEmail(email)

		if errs != nil {
			return responseGeneration(errs.Error(), http.StatusBadRequest)
		}

		code = result["verificationCode"].(float64)
	}
	//-----------------------------------------PACKING RESULTS-----------------------------------------

	pre_js := make(map[string]interface{})
	pre_js["code"] = code
	pre_js["email"] = email

	js, err := json.Marshal(pre_js)
	if err != nil {
		return responseGeneration(err.Error(), http.StatusBadRequest)
	}

	result, err := lambdaClient.Invoke(&lambdaCall.InvokeInput{FunctionName: aws.String("email_code"), Payload: js})
	if err != nil {
		return responseGeneration(err.Error(), http.StatusBadRequest)
	}

	log.Println(result)

	return events.APIGatewayProxyResponse{StatusCode: http.StatusOK, Body: "successfully sent email", Headers: map[string]string{"content-type": "application/json"}}, nil

}
