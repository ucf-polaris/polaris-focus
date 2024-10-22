package test_cases

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"testing"

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
	GSI        []string          `json:"global_secondary_index,omitempty"`
	GSIName    string            `json:"global_secondary_index_name,omitempty"`
	Attributes map[string]string `json:"attributes"`
}

type JsonCases struct {
	Name                 string                   `json:"name"`
	Request              map[string]interface{}   `json:"request"`
	ExpectedResponse     string                   `json:"expected_response"`
	ExpectedResponseBody map[string]interface{}   `json:"expected_response_body"`
	IgnoreInBody         []string                 `json:"ignore_in_body"`
	Add                  []map[string]interface{} `json:"ADD"`
	Get                  []map[string]interface{} `json:"GET"`
	IgnoreInGet          []string                 `json:"ignore_in_get"`
	HandleToken          bool                     `json:"handle_token"`
	ConvertSet           []string                 `json:"convert_to_set"`
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
	BodyIsJson         bool
	ExpectedError      error
	AddToDatabase      []map[string]interface{}
	ExpectedInDatabase []map[string]interface{}
	IgnoreFields       []string
	IgnoreJsonFields   []string
	ConvertToSet       []string
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
		body_flag := false
		if element.ExpectedResponse == "" {
			response = MarshalWrapper(element.ExpectedResponseBody)
			body_flag = true
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
			BodyIsJson:         body_flag,
			ExpectedError:      nil,
			AddToDatabase:      element.Add,
			ExpectedInDatabase: element.Get,
			IgnoreFields:       element.IgnoreInGet,
			IgnoreJsonFields:   element.IgnoreInBody,
			ConvertToSet:       element.ConvertSet,
		}

		ret = append(ret, temp)
	}
	return ret
}

// create KeySchema that links to GenerateTable's KeySchema
func makeKeySchema(keys []string) []types.KeySchemaElement {
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

	return keySchema
}

// create Global Secondary Index that links to GenerateTables Global Secondary Index
func makeGSI(GSI []string, name string) []types.GlobalSecondaryIndex {
	//make key schema for GSI
	keys := makeKeySchema(GSI)

	ret := []types.GlobalSecondaryIndex{
		{
			IndexName: aws.String(name),
			KeySchema: keys,
			ProvisionedThroughput: &types.ProvisionedThroughput{
				ReadCapacityUnits:  aws.Int64(10),
				WriteCapacityUnits: aws.Int64(10),
			},
			Projection: &types.Projection{ProjectionType: types.ProjectionTypeAll},
		},
	}

	return ret
}

// create AttributeDefinition that links to GenerateTable's AtributeDefinitions
func makeAttributeSchema(attributes map[string]string) []types.AttributeDefinition {
	dataTypes := map[string]types.ScalarAttributeType{
		"S": types.ScalarAttributeTypeS,
		"B": types.ScalarAttributeTypeB,
		"N": types.ScalarAttributeTypeN,
	}

	defSchema := []types.AttributeDefinition{}

	for key, val := range attributes {
		record := types.AttributeDefinition{
			AttributeName: aws.String(key),
			AttributeType: dataTypes[val],
		}
		defSchema = append(defSchema, record)
	}

	return defSchema
}

func HelperGenerateTable(client *dynamodb.Client, schema Schem) error {
	a := &dynamodb.ListTablesInput{}
	result, err := client.ListTables(context.TODO(), a)
	if err != nil {
		return err
	}

	//if table doesn't exist, create one
	if len(result.TableNames) == 0 {

		err = GenerateTable(client, schema)
		if err != nil {
			return err
		}
	}

	return nil
}

// Creates table
func GenerateTable(client *dynamodb.Client, schema Schem) error {
	keySchema := makeKeySchema(schema.Keys)
	attributeSchema := makeAttributeSchema(schema.Attributes)

	input := &dynamodb.CreateTableInput{
		AttributeDefinitions: attributeSchema,
		KeySchema:            keySchema,
		TableName:            aws.String("THENEWTABLE"),
		ProvisionedThroughput: &types.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(10),
			WriteCapacityUnits: aws.Int64(10),
		},
	}

	if len(schema.GSI) != 0 {
		GSI := makeGSI(schema.GSI, schema.GSIName)
		input.GlobalSecondaryIndexes = GSI
	}

	_, err := client.CreateTable(context.Background(), input)

	if err != nil {
		return err
	}

	return nil

}

// Adds random data to table with partition and sort key defined
func BatchAddToTable(client *dynamodb.Client, values []map[string]interface{}, convert []string) error {
	for _, element := range values {
		data, _ := attributevalue.MarshalMap(element)

		//convert to string set
		ListToStringSet(convert, data)

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

// imports filename to run
func ImportConfigs() (Configs, error) {
	jsonFile, err := os.Open("test-cases/configs.json")
	if err != nil {
		return Configs{}, err
	}

	byteFile, _ := ioutil.ReadAll(jsonFile)

	//get json file output
	output := Configs{}

	err = json.Unmarshal(byteFile, &output)
	log.Println(output)
	if err != nil {
		return Configs{}, err
	}

	defer jsonFile.Close()
	return output, nil
}

// compare expected body with obtained body, but account "ignore in"
func CompareBodies(expected map[string]interface{}, obtained map[string]interface{}, ignore []string) (bool, string, string) {
	expected_copy := deepCopyMap(expected)
	obtained_copy := deepCopyMap(obtained)

	for _, e := range ignore {
		delete(expected_copy, e)
		delete(obtained_copy, e)
	}

	return MarshalWrapper(expected_copy) == MarshalWrapper(obtained_copy), MarshalWrapper(expected_copy), MarshalWrapper(obtained_copy)
}

// helper to deep copy a map
func deepCopyMap(M map[string]interface{}) map[string]interface{} {
	ret := make(map[string]interface{})
	for k, v := range M {
		ret[k] = v
	}

	return ret
}

// transforms all fields provided into string set from lists
func ListToStringSet(fields []string, M map[string]types.AttributeValue) {
	//go through fields
	for _, element := range fields {
		//if of type AV list
		if val, ok := M[element].(*types.AttributeValueMemberL); ok {

			temp := []string{}
			err := attributevalue.Unmarshal(val, &temp)

			if err != nil {
				panic(err)
			}

			//delete key if empty (ADD will reappend)
			if len(temp) != 0 {
				M[element] = &types.AttributeValueMemberSS{Value: temp}
			} else {
				delete(M, element)
			}
		}
	}
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

	M["tokens"] = tokens

	return tokens
}

func ignoreSchem(vals map[string]interface{}, ignore []string) {
	for _, e := range ignore {
		delete(vals, e)
	}
}

func CompareTable(client *dynamodb.Client, table string, expected []map[string]interface{}, ignore []string, t *testing.T) error {
	output, err := client.Scan(context.TODO(), &dynamodb.ScanInput{
		TableName: aws.String(table),
	})

	if err != nil {
		return err
	}

	count := 0

	//if there's nothing just return
	if len(expected) == 0 {
		return nil
	}
	fmt.Println()

	t.Log("___GET COMPARE STARTED___")

	//double for loop comparing contents of two lists
	for _, ee := range expected {
		copy_map := output.Items

		flag := false
		//subtract one element each time you've found a successful match
		for io, eo := range copy_map {
			new_map := make(map[string]interface{})
			attributevalue.UnmarshalMap(eo, &new_map)

			ignoreSchem(new_map, ignore)
			//log.Println(strconv.Itoa(io+1) + ": " + MarshalWrapper(new_map))

			if MarshalWrapper(new_map) == MarshalWrapper(ee) {
				flag = true
				output.Items = append(output.Items[:io], output.Items[io+1:]...)
				t.Log(MarshalWrapper(new_map) + " found")
				count++
				break
			}
		}
		if !flag {
			t.Log(MarshalWrapper(ee) + " not found")
			return errors.New(MarshalWrapper(ee))
		}
	}

	if count == len(expected) {
		return nil
	}
	return errors.New("element")
}

func ResetTable(client *dynamodb.Client, schema Schem) {
	_, _ = client.DeleteTable(context.TODO(), &dynamodb.DeleteTableInput{
		TableName: aws.String("THENEWTABLE")})

	GenerateTable(client, schema)
}
