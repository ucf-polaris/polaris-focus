package main

import (
	"context"
	helpers "polaris-api/TestingPipeline/the_first_go/polaris-test/Helpers"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	dyn2 "github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

func onShutdown(err error) {
	_, _ = client.DeleteTable(context.TODO(), &dyn2.DeleteTableInput{
		TableName: aws.String("THENEWTABLE")})
	if err != nil {
		panic(err)
	}

}

func TestHandler(t *testing.T) {
	//get configs
	configs, err := helpers.GetFile()
	if err != nil {
		onShutdown(err)
	}
	test_case, err := helpers.ImportCases(configs)
	if err != nil {
		onShutdown(err)
	}

	//create table
	if err = helpers.HelperGenerateTable(client, test_case.Schema); err != nil {
		onShutdown(err)
	}

	testCases := helpers.CreateTestCases(test_case.TestCases)

	for _, testCase := range testCases {
		//add pieces to empty database before hand
		err := helpers.BatchAddToTable(client, testCase.AddToDatabase)
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

			errs := helpers.CompareTable(client, "THENEWTABLE", testCase.ExpectedInDatabase)
			if errs != nil {
				t.Errorf(errs.Error())
			}
		})

		//reset to empty table
		helpers.ResetTable(client, test_case.Schema)
	}
	onShutdown(nil)
}
