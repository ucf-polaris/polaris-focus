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

func TestADD(t *testing.T) {
	//get configs
	schema, partition, sort, err := Helpers.ImportConfigs()
	if err != nil {
		onShutdown(err)
	}

	//create table
	if err = Helpers.HelperGenerateTable(client, partition, sort, schema); err != nil {
		onShutdown(err)
	}

	//get data
	_, values, _ := Helpers.ProduceRandomData(schema)

	//setup json requests for test cases
	_, tokens := Helpers.ProduceToken(make(map[string]interface{}))
	tkn_str := Helpers.MarshalWrapper(tokens)

	testCases := []struct {
		name          string
		request       events.APIGatewayProxyRequest
		expectedBody  string
		expectedError error
	}{
		{
			name: "Regular ADD Request with partition (and sort) key(s)",
			request: events.APIGatewayProxyRequest{
				RequestContext: events.APIGatewayProxyRequestContext{
					Authorizer: map[string]interface{}{
						"stringKey": tkn_str,
					},
				},
				Body: Helpers.MarshalWrapper(values),
			},
			expectedBody:  tkn_str,
			expectedError: nil,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			response, err := handler(testCase.request)

			cond, expected_str, obtained_str := Helpers.IsInTable(client, partition, sort, values)

			if cond == false {
				t.Errorf("Expected item %q, but got %q", expected_str, obtained_str)
			}

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

func TestGET(t *testing.T) {
	//get configs
	schema, partition, sort, err := Helpers.ImportConfigs()
	if err != nil {
		onShutdown(err)
	}

	//create table
	if err = Helpers.HelperGenerateTable(client, partition, sort, schema); err != nil {
		onShutdown(err)
	}

	//add a value to table
	values, err := Helpers.AddToTable(client, schema)
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
