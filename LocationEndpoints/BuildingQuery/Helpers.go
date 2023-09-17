package main

import (
	"encoding/json"

	"github.com/aws/aws-lambda-go/events"
)

func responseGeneration(err error, status int) (events.APIGatewayProxyResponse, error) {
	return events.APIGatewayProxyResponse{StatusCode: status, Body: ""}, err
}

func getTokens(request events.APIGatewayProxyRequest) (string, string, error) {
	var token string
	var rfsTkn string

	if request.RequestContext.Authorizer != nil {
		contextString := request.RequestContext.Authorizer["stringKey"].(string)

		ctxt := map[string]any{}
		err := json.Unmarshal([]byte(contextString), &ctxt)
		if err != nil {
			return "", "", nil
		}

		if val, ok := ctxt["token"].(string); ok {
			token = val
		}

		if val, ok := ctxt["refreshToken"].(string); ok {
			rfsTkn = val
		}
	}

	return token, rfsTkn, nil
}
