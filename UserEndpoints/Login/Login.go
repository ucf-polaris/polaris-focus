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

type UserQuery struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type User struct {
	UserID           string   `json:"UserID"`
	Email            string   `json:"email"`
	Password         string   `json:"password"`
	Schedule         []string `json:"schedule"`
	Visited          []string `json:"visited"`
	Favorite         []string `json:"favorite"`
	Username         string   `json:"username"`
	Name             string   `json:"name"`
	VerificationCode int      `json:"verificationCode"`
}

var table string
var client *dynamodb.Client
var lambdaClient *lambdaCall.Lambda

func init() {
	//dynamoDB
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

func ConstructVerified(queryResponse User, password string) (string, error) {
	//store and hide the password
	if queryResponse.Password == "" {
		return "", errors.New("query returned no password field")
	}

	//checking the password, if nothing return error
	if queryResponse.Password != password {
		return "", errors.New("invalid email/password")
	}

	queryResponse.Password = ""
	js, _ := json.Marshal(queryResponse)

	//-----------------------------------------TOKEN-----------------------------------------
	ret := make(map[string]interface{})
	json.Unmarshal(js, &ret)
	delete(ret, "verificationCode")

	tokens := make(map[string]interface{})
	//make and return token and refresh token
	tkn, err := Helpers.CreateToken(lambdaClient, 15, "", 0)
	if err != nil {
		return "", err
	}

	rfs, err := Helpers.CreateToken(lambdaClient, 1440, "", 1)
	if err != nil {
		return "", err
	}

	tokens["token"] = tkn
	tokens["refreshToken"] = rfs

	ret["tokens"] = tokens

	//package the results
	js, err = json.Marshal(ret)
	if err != nil {
		return "", err
	}

	return string(js), nil
}

func ConstructNonVerified(queryResponse User) (string, error) {
	if queryResponse.UserID == "" {
		return "", errors.New("ID field not found")
	}

	if queryResponse.Email == "" {
		return "", errors.New("email field not found")
	}

	ret := make(map[string]interface{})

	ret["UserID"] = queryResponse.UserID
	ret["email"] = queryResponse.Email

	//construct token with userID embedded
	regtkn, err := Helpers.CreateToken(lambdaClient, 15, queryResponse.UserID, 2)
	if err != nil {
		return "", err
	}

	ret["tokens"] = map[string]string{
		"token": regtkn,
	}

	js, err := json.Marshal(ret)
	if err != nil {
		return "", err
	}

	return string(js), nil
}

// TO-DO: create a function that handles response returns (more clean and more info/debug info)
func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	//-----------------------------------------PREPARATION-----------------------------------------
	//field checking and extract username and password fields
	search := UserQuery{}
	err := json.Unmarshal([]byte(request.Body), &search)
	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}

	//error check username and pass
	if search.Email == "" || search.Password == "" {
		return Helpers.ResponseGeneration("field not set", http.StatusOK)
	}
	//-----------------------------------------THE QUERY-----------------------------------------
	//pass parameters into query
	item_username := make(map[string]types.AttributeValue)
	item_username[":email"] = &types.AttributeValueMemberS{Value: search.Email}

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
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}

	//No username found
	if QueryResults.Count == 0 {
		return Helpers.ResponseGeneration("invalid email/password", http.StatusOK)
	}

	//More than one username found (shouldn't happen, but could)
	if QueryResults.Count > 1 {
		return Helpers.ResponseGeneration("more than one email found", http.StatusOK)
	}
	//-----------------------------------------PACKING RESULTS-----------------------------------------
	//get results in
	newUser := User{}
	attributevalue.UnmarshalMap(QueryResults.Items[0], &newUser)

	var ret string

	//user not verified
	if newUser.VerificationCode != 0 {
		ret, err = ConstructNonVerified(newUser)

		if err != nil {
			return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
		}
		//user verified
	} else {
		ret, err = ConstructVerified(newUser, search.Password)
		if err != nil {
			return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
		}
	}

	return events.APIGatewayProxyResponse{StatusCode: http.StatusOK, Body: ret, Headers: map[string]string{"content-type": "application/json"}}, nil
}
