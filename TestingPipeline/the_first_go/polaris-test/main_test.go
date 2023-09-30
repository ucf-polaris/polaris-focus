package main

import (
	"context"
	"encoding/json"
	"fmt"
	"polaris-api/TestingPipeline/the_first_go/polaris-test/Helpers"
	helpers "polaris-api/TestingPipeline/the_first_go/polaris-test/Helpers"
	"strings"
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
	cfgs, err := helpers.ImportConfigs()
	if err != nil {
		onShutdown(err)
	}

	test_case, err := helpers.ImportCases("Helpers/" + cfgs.FileName)
	if err != nil {
		onShutdown(err)
	}

	//create table
	err = helpers.HelperGenerateTable(client, test_case.Schema)
	if err != nil {
		onShutdown(err)
	}

	testCases := helpers.CreateTestCases(test_case.TestCases)

	for _, testCase := range testCases {
		//add pieces to empty database before hand
		err := helpers.BatchAddToTable(client, testCase.AddToDatabase, testCase.ConvertToSet)
		if err != nil {
			panic(err)
		}

		//run the test
		t.Run(testCase.Name, func(t *testing.T) {
			response, err := handler(testCase.Request)

			// Unintentional error output
			if err != testCase.ExpectedError {
				t.Errorf("Expected error %v, but got %v", testCase.ExpectedError, err)
			}

			// Handle ExpectedBody (non-json)
			if !testCase.BodyIsJson {
				//if 'ERROR' in obtained body and 'ERROR' in expected, pass
				if strings.Contains(testCase.ExpectedBody, "ERROR") {
					if !strings.Contains(response.Body, "ERROR") {
						t.Errorf("Expected response %v, but got %v", testCase.ExpectedBody, response.Body)
					} else {
						t.Log("Error Message: " + response.Body)
					}
					//if 'ERROR' not in expected, compare directly
				} else {
					if testCase.ExpectedBody != response.Body {
						t.Errorf("Expected response %v, but got %v", testCase.ExpectedBody, response.Body)
					}
				}
			}

			// Handle ExpectedBody(json)
			if testCase.BodyIsJson {
				expected := make(map[string]interface{})
				obtained := make(map[string]interface{})
				json.Unmarshal([]byte(testCase.ExpectedBody), &expected)
				err = json.Unmarshal([]byte(response.Body), &obtained)

				//response body isn't in json format
				if err != nil {
					t.Errorf("Expected json response but got %v", response.Body)
				}

				if ok, exp, obt := Helpers.CompareBodies(expected, obtained, testCase.IgnoreJsonFields); !ok {
					t.Errorf("Expected json response %v, but got %v", exp, obt)
				} else {
					t.Logf("Body returned " + response.Body)
				}
			}

			/*if response.StatusCode != 200 {
				t.Errorf("Expected status code 200, but got %v", response.StatusCode)
			}*/

			//run get to test against database
			errs := helpers.CompareTable(client, "THENEWTABLE", testCase.ExpectedInDatabase, testCase.IgnoreFields, t)
			if errs != nil {
				t.Errorf("Get Test failed expected in database" + errs.Error())
			}
			fmt.Println()
		})

		//reset to empty table
		helpers.ResetTable(client, test_case.Schema)
	}
	onShutdown(nil)
}
