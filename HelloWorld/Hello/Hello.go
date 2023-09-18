package main

import (
	"net/http"
	"polarisapi/pkg/helpers"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-lambda-go/events"
)

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	res := "Hello " + Helpers.Helloworld()
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body: 		res,
	}, nil
}

func main() {
	lambda.Start(handler)
}