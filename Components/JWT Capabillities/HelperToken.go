// How to call the token create function from another function
package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	lambdaCall "github.com/aws/aws-sdk-go/service/lambda"
)

type JWTConstructor struct {
	TimeTil int `json:"timeTil"`
}

type JWTPackage struct {
	RefreshToken string `json:"refreshToken"`
	Token        string `json:"token"`
}

type JWTResponse struct {
	Token string `json:"token"`
}

var client *lambdaCall.Lambda
var funct_name string

func init() {
	funct_name = os.Getenv("FUNC_NAME")

	if funct_name == "" {
		log.Fatal("missing environment variable FUNC_NAME")
	}
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))

	client = lambdaCall.New(sess, &aws.Config{Region: aws.String("us-east-2")})
}

func main() {
	lambda.Start(handler)
}

func responseGeneration(msg string, status int) (events.APIGatewayProxyResponse, error) {
	return events.APIGatewayProxyResponse{StatusCode: status, Body: "Error: " + msg}, errors.New(msg)
}

func unpackRequest(body string) JWTConstructor {
	if body == "" {
		return JWTConstructor{}
	}

	log.Println("body: ", body)

	search := JWTConstructor{}
	err := json.Unmarshal([]byte(body), &search)

	if err != nil {
		panic(err)
	}

	return search
}

func handler(request events.APIGatewayProxyResponse) (events.APIGatewayProxyResponse, error) {
	body := unpackRequest(request.Body)

	newJWT := JWTConstructor{TimeTil: body.TimeTil}
	payload, err := json.Marshal(newJWT)
	if err != nil {
		return responseGeneration(err.Error(), http.StatusBadRequest)
	}

	result, err := client.Invoke(&lambdaCall.InvokeInput{FunctionName: aws.String(funct_name), Payload: payload})
	if err != nil {
		return responseGeneration(err.Error(), http.StatusBadRequest)
	}

	return events.APIGatewayProxyResponse{StatusCode: http.StatusOK, Body: string(result.Payload), Headers: map[string]string{"content-type": "application/json"}}, nil

}
