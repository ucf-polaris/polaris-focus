package helpers

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type Schem struct {
	Keys       []string          `json:"keys"`
	Attributes map[string]string `json:"attributes"`
}

type JsonCases struct {
	Name                 string                   `json:"name"`
	Request              map[string]interface{}   `json:"request"`
	ExpectedResponse     string                   `json:"expected_response"`
	ExpectedResponseBody map[string]interface{}   `json:"expected_response_body"`
	Add                  []map[string]interface{} `json:"ADD"`
	Get                  []map[string]interface{} `json:"GET"`
	HandleToken          bool                     `json:"handle_token"`
}

type FileCases struct {
	Schema    Schem       `json:"schema"`
	TestCases []JsonCases `json:"test_cases"`
}

type Configs struct {
	FileName string `json:"filename"`
}

type TestCases struct {
	Name               string
	Request            events.APIGatewayProxyRequest
	ExpectedBody       string
	ExpectedError      error
	AddToDatabase      []map[string]interface{}
	ExpectedInDatabase []map[string]interface{}
}

func CreateTestCases(t []JsonCases) []TestCases {
	//for token handling
	ret := []TestCases{}

	for _, element := range t {
		//determine if token will be handled
		tkn_str := ""
		if element.HandleToken {
			tkn_str = MarshalWrapper(AppendToken(element.ExpectedResponseBody))
		}

		//determine value of ExpectedResponse
		response := element.ExpectedResponse
		if element.ExpectedResponse == "" {
			response = MarshalWrapper(element.ExpectedResponseBody)
		}

		temp := TestCases{
			Name: element.Name,
			Request: events.APIGatewayProxyRequest{
				RequestContext: events.APIGatewayProxyRequestContext{
					Authorizer: map[string]interface{}{
						"stringKey": tkn_str,
					},
				},
				Body: MarshalWrapper(element.Request),
			},
			ExpectedBody:       response,
			ExpectedError:      nil,
			AddToDatabase:      element.Add,
			ExpectedInDatabase: element.Get,
		}

		ret = append(ret, temp)
	}
	return ret
}

// create KeySchema that links to GenerateTable's KeySchema
func makeKeySchema(keys []string) ([]types.KeySchemaElement, string, string) {
	//error checking for empty list
	if len(keys) == 0 {
		panic(errors.New("empty keys array"))
	}

	partition := keys[0]
	keySchema := []types.KeySchemaElement{
		{
			AttributeName: aws.String(partition),
			KeyType:       types.KeyTypeHash,
		},
	}

	sort := ""
	if len(keys) > 1 {
		sort = keys[1]
		sortKey := types.KeySchemaElement{
			AttributeName: aws.String(sort),
			KeyType:       types.KeyTypeRange,
		}
		keySchema = append(keySchema, sortKey)
	}

	return keySchema, partition, sort
}

// create AttributeDefinition that links to GenerateTable's AtributeDefinitions
func makeAttributeSchema(partition string, sort string, attributes map[string]string) []types.AttributeDefinition {
	dataTypes := map[string]types.ScalarAttributeType{
		"S": types.ScalarAttributeTypeS,
		"B": types.ScalarAttributeTypeB,
		"N": types.ScalarAttributeTypeN,
	}

	if _, ok := attributes[partition]; !ok {
		panic(errors.New("missing partition in schema"))
	}

	defSchema := []types.AttributeDefinition{
		{
			AttributeName: aws.String(partition),
			AttributeType: dataTypes[attributes[partition]],
		},
	}

	if sort != "" {
		if _, ok := attributes[sort]; !ok {
			panic(errors.New("missing sort in schema"))
		}
		defElement := types.AttributeDefinition{
			AttributeName: aws.String(sort),
			AttributeType: dataTypes[attributes[sort]],
		}
		defSchema = append(defSchema, defElement)
	}

	return defSchema
}

func HelperGenerateTable(client *dynamodb.Client, schema Schem) error {
	a := &dynamodb.ListTablesInput{}
	result, _ := client.ListTables(context.TODO(), a)

	//if table doesn't exist, create one
	if len(result.TableNames) == 0 {

		err := GenerateTable(client, schema)
		if err != nil {
			return err
		}
	}

	return nil
}

// Creates table
func GenerateTable(client *dynamodb.Client, schema Schem) error {
	keySchema, partition, sort := makeKeySchema(schema.Keys)
	attributeSchema := makeAttributeSchema(partition, sort, schema.Attributes)

	_, err := client.CreateTable(context.Background(), &dynamodb.CreateTableInput{
		AttributeDefinitions: attributeSchema,
		KeySchema:            keySchema,
		TableName:            aws.String("THENEWTABLE"),
		ProvisionedThroughput: &types.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(10),
			WriteCapacityUnits: aws.Int64(10),
		},
	})

	if err != nil {
		return err
	}

	return nil

}

// Adds random data to table with partition and sort key defined
func BatchAddToTable(client *dynamodb.Client, values []map[string]interface{}) error {
	for _, element := range values {
		data, _ := attributevalue.MarshalMap(element)

		_, err := client.PutItem(context.TODO(), &dynamodb.PutItemInput{
			TableName: aws.String("THENEWTABLE"),
			Item:      data,
		})

		if err != nil {
			return err
		}
	}

	return nil
}

// Helper that creates the dynamoDB host
func ConstructDynamoHost() *dynamodb.Client {
	var err error
	var cfg aws.Config
	cfg, err = config.LoadDefaultConfig(context.TODO(),
		config.WithRegion("localhost"),
		config.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptionsFunc(
			func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{URL: "http://localhost:8000"}, nil
			})),
		config.WithCredentialsProvider(credentials.StaticCredentialsProvider{
			Value: aws.Credentials{
				AccessKeyID: "abcd", SecretAccessKey: "a1b2c3", SessionToken: "",
				Source: "Mock credentials used above for local instance",
			},
		}),
	)
	if err != nil {
		panic(err)
	}

	return dynamodb.NewFromConfig(cfg)
}

func GetFile() (string, error) {
	configs, err := os.Open("Helpers/configs.json")
	byteFile, _ := ioutil.ReadAll(configs)

	//get json file output
	output := Configs{}

	err = json.Unmarshal(byteFile, &output)
	if err != nil {
		return "", err
	}

	defer configs.Close()

	return "Helpers/" + output.FileName, nil
}

// imports schema and keys from test file
func ImportCases(filepath string) (FileCases, error) {
	jsonFile, err := os.Open(filepath)
	if err != nil {
		return FileCases{}, err
	}

	byteFile, _ := ioutil.ReadAll(jsonFile)

	//get json file output
	output := FileCases{}

	err = json.Unmarshal(byteFile, &output)
	if err != nil {
		return FileCases{}, err
	}

	defer jsonFile.Close()
	return output, nil
}

// helper function that marshals stuff in-line
func MarshalWrapper(M map[string]interface{}) string {
	js, _ := json.Marshal(M)
	return string(js)
}

// produces map with token fields already packed in
func AppendToken(M map[string]interface{}) map[string]interface{} {
	tokens := map[string]interface{}{
		"token":        "tkn",
		"refreshToken": "rfsTkn",
	}

	M["token"] = "tkn"
	M["refreshToken"] = "rfsTkn"

	return tokens
}

func CompareTable(client *dynamodb.Client, table string, expected []map[string]interface{}) error {
	output, err := client.Scan(context.TODO(), &dynamodb.ScanInput{
		TableName: aws.String(table),
	})

	if err != nil {
		return err
	}

	count := 0

	//double for loop comparing contents of two lists
	for _, ee := range expected {
		copy_map := output.Items

		flag := false
		//subtract one element each time you've found a successful match
		for io, eo := range copy_map {
			new_map := make(map[string]interface{})
			attributevalue.UnmarshalMap(eo, &new_map)

			if MarshalWrapper(new_map) == MarshalWrapper(ee) {
				flag = true
				output.Items = append(output.Items[:io], output.Items[io+1:]...)
				count++
				break
			}
		}
		if flag {
			return errors.New("element " + MarshalWrapper(ee) + " missing")
		}
	}

	if count == len(expected) {
		return nil
	}
	return errors.New("element missing")
}

func ResetTable(client *dynamodb.Client, schema Schem) {
	_, _ = client.DeleteTable(context.TODO(), &dynamodb.DeleteTableInput{
		TableName: aws.String("THENEWTABLE")})

	GenerateTable(client, schema)
}
