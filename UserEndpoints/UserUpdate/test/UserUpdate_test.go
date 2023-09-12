package main

import (
	"testing"
	"context"
	"github.com/aws/aws-lambda-go/events"
)

func TestHandlerInputParsing(t *testing.T) {
	testReq := events.APIGatewayProxyRequest {
		Body: `{
			"UserID": "1dce0035-0cdc-4dcf-9e0f-47c9af5cb005",
			"email": "mine5784@gmail.com",
			"password": "also super mine",
			"schedule": ["CAP 3219", "Some class"],
			"username": "mine",
			"name": "Bartholomew"
		}`,
	}

	response, err := handler(context.Background(), testReq)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}

	if response.StatusCode != 200 {
		t.Fatalf("Failed, status code: %d", response.StatusCode)
	}
}