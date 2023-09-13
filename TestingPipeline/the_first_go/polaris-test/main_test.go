package main

import (
	"context"
	"polaris-test/Helpers"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	dyn2 "github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

func onShutdown(err error) {
	_, errs := client.DeleteTable(context.TODO(), &dyn2.DeleteTableInput{
		TableName: aws.String("THENEWTABLE")})
	if errs != nil {
		panic(errs)
	}

	if err != nil {
		panic(err)
	}

}

func TestGET(t *testing.T) {
	a := &dyn2.ListTablesInput{}
	result, _ := client.ListTables(context.TODO(), a)

	//setup table and get keys/schema
	partition, sort, schema := Helpers.Setup(client)

	//if table doesn't exist, create one
	if len(result.TableNames) == 0 {
		err := Helpers.GenerateTable(client, partition, sort, schema)
		if err != nil {
			onShutdown(err)
		}
	}

	//add a value to table
	values, err := Helpers.AddToTable(client, partition, sort, schema)
	if err != nil {
		onShutdown(err)
	}

	//setup json requests for test cases
	values, tokens := Helpers.ProduceToken(values)
	tkn_str := Helpers.MarshalWrapper(tokens)

	//setup incorrect keys
	wrong_vals := Helpers.ProduceIncorrectKeys(partition, sort, schema, values)

	testCases := []struct {
		name          string
		request       events.APIGatewayProxyRequest
		expectedBody  string
		expectedError error
	}{
		{
			name: "Regular GET Request with partition (and sort) key(s)",
			request: events.APIGatewayProxyRequest{
				RequestContext: events.APIGatewayProxyRequestContext{
					Authorizer: map[string]interface{}{
						"stringKey": tkn_str,
					},
				},
				Body: Helpers.MarshalKeys(partition, sort, values),
			},
			expectedBody:  Helpers.MarshalWrapper(values),
			expectedError: nil,
		},
		{
			// mock a request with a localhost SourceIP
			name: "Record not in database",
			request: events.APIGatewayProxyRequest{
				RequestContext: events.APIGatewayProxyRequestContext{
					Authorizer: map[string]interface{}{
						"stringKey": tkn_str,
					},
				},
				Body: Helpers.MarshalWrapper(wrong_vals),
			},
			expectedBody:  "",
			expectedError: ErrRecordNotFound,
		},
		{
			// mock a request with a localhost SourceIP
			name: "Use incorrect key schema",
			request: events.APIGatewayProxyRequest{
				RequestContext: events.APIGatewayProxyRequestContext{
					Authorizer: map[string]interface{}{
						"stringKey": tkn_str,
					},
				},
				Body: Helpers.MarshalWrapper(map[string]interface{}{
					"wrong_key": "wrong!",
				}),
			},
			expectedBody:  "",
			expectedError: ErrKeyNotFound,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			response, err := handler(testCase.request)
			if err != testCase.expectedError {
				t.Errorf("Expected error %v, but got %v", testCase.expectedError, err)
			}

			if response.Body != testCase.expectedBody {
				t.Errorf("Expected response %v, but got %v", testCase.expectedBody, response.Body)
			}

			if response.StatusCode != 200 {
				t.Errorf("Expected status code 200, but got %v", response.StatusCode)
			}
		})
	}
	onShutdown(nil)
}
