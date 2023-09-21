package Template

import (
	"context"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

var client *dynamodb.Client
var handler func(r events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error)
var filepath string = ""

func OnShutdown(err error) {
	_, _ = client.DeleteTable(context.TODO(), &dynamodb.DeleteTableInput{
		TableName: aws.String("THENEWTABLE")})
	if err != nil {
		panic(err)
	}

}

func SetVariables(c *dynamodb.Client, h func(r events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error), fn string) {
	client = c
	handler = h
	filepath = fn
}

func TestHandler(t *testing.T) {
	test_case, err := ImportCases(filepath)
	if err != nil {
		OnShutdown(err)
	}

	//create table
	if err = HelperGenerateTable(client, test_case.Schema); err != nil {
		OnShutdown(err)
	}

	testCases := CreateTestCases(test_case.TestCases)

	for _, testCase := range testCases {
		//add pieces to empty database before hand
		err := BatchAddToTable(client, testCase.AddToDatabase)
		if err != nil {
			panic(err)
		}

		//run the test
		t.Run(testCase.Name, func(t *testing.T) {
			response, err := handler(testCase.Request)

			//run get request to compare
			if err != testCase.ExpectedError {
				t.Errorf("Expected error %v, but got %v", testCase.ExpectedError, err)
			}

			if err != testCase.ExpectedError {
				t.Errorf("Expected error %v, but got %v", testCase.ExpectedError, err)
			}

			if response.Body != testCase.ExpectedBody {
				t.Errorf("Expected response %v, but got %v", testCase.ExpectedBody, response.Body)
			}

			if response.StatusCode != 200 {
				t.Errorf("Expected status code 200, but got %v", response.StatusCode)
			}

			errs := CompareTable(client, "THENEWTABLE", testCase.ExpectedInDatabase)
			if errs != nil {
				t.Errorf(errs.Error())
			}
		})

		//reset to empty table
		ResetTable(client, test_case.Schema)
	}
	OnShutdown(nil)
}
