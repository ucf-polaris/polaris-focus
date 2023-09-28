// How to call the token create function from another function
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"polaris-api/pkg/Helpers"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
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
	ret["token"] = token
	ret["refreshToken"] = refreshToken
	js, _ = json.Marshal(ret)

	return events.APIGatewayProxyResponse{StatusCode: http.StatusOK, Body: string(js), Headers: map[string]string{"content-type": "application/json"}}, nil

}
