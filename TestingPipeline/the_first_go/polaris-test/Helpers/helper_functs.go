package Helpers

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"math/rand"
	"os"
	"reflect"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// Produce Random Data that links back to AddToTable
func ProduceRandomData(schema map[string]string) (map[string]types.AttributeValue, map[string]interface{}, error) {
	values := make(map[string]interface{})

	rand.Seed(time.Now().UnixNano())

	for k, v := range schema {
		switch v {
		case "N":
			values[k] = rand.Intn(10000)
		case "BOOL":
			values[k] = true
		case "S":
			values[k] = strconv.Itoa(rand.Int())
		case "L":
			values[k] = []string{"this", "is", "a", "mighty", "test"}
		case "M":
			values[k] = map[string]int{
				"This":      1,
				"That":      2,
				"OverWhere": 3,
			}
		default:
			return nil, nil, errors.New("invalid datatype")
		}
	}

	item, err := attributevalue.MarshalMap(values)
	if err != nil {
		return nil, nil, err
	}

	return item, values, nil
}

// create KeySchema that links to GenerateTable's KeySchema
func makeKeySchema(partition string, sort string) []types.KeySchemaElement {
	keySchema := []types.KeySchemaElement{
		{
			AttributeName: aws.String(partition),
			KeyType:       types.KeyTypeHash,
		},
	}

	if sort != "" {
		sortKey := types.KeySchemaElement{
			AttributeName: aws.String(sort),
			KeyType:       types.KeyTypeRange,
		}
		keySchema = append(keySchema, sortKey)
	}

	return keySchema
}

// create AttributeDefinition that links to GenerateTable's AtributeDefinitions
func makeAttributeSchema(partition string, sort string, schema map[string]string) []types.AttributeDefinition {
	dataTypes := map[string]types.ScalarAttributeType{
		"S": types.ScalarAttributeTypeS,
		"B": types.ScalarAttributeTypeB,
		"N": types.ScalarAttributeTypeN,
	}

	defSchema := []types.AttributeDefinition{
		{
			AttributeName: aws.String(partition),
			AttributeType: dataTypes[schema[partition]],
		},
	}

	if sort != "" {
		defElement := types.AttributeDefinition{
			AttributeName: aws.String(sort),
			AttributeType: dataTypes[schema[sort]],
		}
		defSchema = append(defSchema, defElement)
	}

	return defSchema
}

func HelperGenerateTable(client *dynamodb.Client, partition string, sort string, schema map[string]string) error {
	a := &dynamodb.ListTablesInput{}
	result, _ := client.ListTables(context.TODO(), a)

	//if table doesn't exist, create one
	if len(result.TableNames) == 0 {
		err := GenerateTable(client, partition, sort, schema)
		if err != nil {
			return err
		}
	}

	return nil
}

// Creates table
func GenerateTable(client *dynamodb.Client, partition string, sort string, schema map[string]string) error {
	keySchema := makeKeySchema(partition, sort)
	attributeSchema := makeAttributeSchema(partition, sort, schema)

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
func AddToTable(client *dynamodb.Client, schema map[string]string) (map[string]interface{}, error) {
	data, output, err := ProduceRandomData(schema)

	if err != nil {
		return nil, err
	}

	_, err = client.PutItem(context.TODO(), &dynamodb.PutItemInput{
		TableName: aws.String("THENEWTABLE"),
		Item:      data,
	})

	if err != nil {
		return nil, err
	}

	return output, nil
}

// Extracts keys, links back to ImportConfigs
func extractKeys(full map[string]interface{}) (string, string, error) {
	keys, ok := full["keys"].([]interface{})
	if !ok {
		return "", "", errors.New("missing/invalid keys field")
	}

	if len(keys) == 1 {
		return keys[0].(string), "", nil
	}
	return keys[0].(string), keys[1].(string), nil
}

// Extracts schema, links back to ImportConfigs
func extractSchema(full map[string]interface{}) (map[string]string, error) {
	ret := make(map[string]string)

	schem, ok := full["schema"]
	if !ok {
		return nil, errors.New("something went wrong when extracting schema")
	} else {
		// some weird voodoo magic
		js, _ := json.Marshal(schem)
		json.Unmarshal(js, &ret)
	}
	return ret, nil
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

// imports schema and keys from config file
func ImportConfigs() (map[string]string, string, string, error) {
	jsonFile, err := os.Open("Helpers/configs.json")
	if err != nil {
		return nil, "", "", err
	}

	byteFile, _ := ioutil.ReadAll(jsonFile)

	//get json file output
	output := make(map[string]interface{})
	err = json.Unmarshal(byteFile, &output)
	if err != nil {
		return nil, "", "", err
	}

	//extract keys
	partition, sort, err := extractKeys(output)
	if err != nil {
		return nil, "", "", err
	}

	//extract schema
	schema, err := extractSchema(output)
	if err != nil {
		return nil, "", "", err
	}

	defer jsonFile.Close()

	return schema, partition, sort, nil
}

// helper function that marshals the keys for return
func MarshalKeys(partition string, sort string, values map[string]interface{}) string {
	new_map := map[string]interface{}{
		partition: values[partition],
	}

	if sort != "" {
		new_map[sort] = values[sort]
	}

	js, _ := json.Marshal(new_map)

	return string(js)
}

// helper function that marshals stuff in-line
func MarshalWrapper(M map[string]interface{}) string {
	js, _ := json.Marshal(M)
	return string(js)
}

// produces map with token fields already packed in
func ProduceToken(M map[string]interface{}) (map[string]interface{}, map[string]interface{}) {
	tokens := map[string]interface{}{
		"token":        "tkn",
		"refreshToken": "rfsTkn",
	}
	M["token"] = tokens["token"]
	M["refreshToken"] = tokens["refreshToken"]

	return M, tokens
}

// produce incorrect keys that (probably) don't exist in the database (FOR GET)
func ProduceIncorrectKeys(partition string, sort string, schema map[string]string, correct map[string]interface{}) map[string]interface{} {
	//prepare key schema
	tmp := map[string]string{
		partition: schema[partition],
	}
	if sort != "" {
		tmp[sort] = schema[sort]
	}

	MAX_COUNT := 10
	count := 0
	_, wrong_vals, _ := ProduceRandomData(tmp)
	//if (somehow) wrong_vals and correct are equal, roll again
	for reflect.DeepEqual(wrong_vals, correct) && count != MAX_COUNT {
		_, wrong_vals, _ = ProduceRandomData(tmp)
		count++
	}

	return wrong_vals
}

func IsInTable(client *dynamodb.Client, partition string, sort string, values map[string]interface{}) (bool, string, string) {
	js := make(map[string]interface{})
	json.Unmarshal([]byte(MarshalKeys(partition, sort, values)), &js)

	av, _ := attributevalue.MarshalMap(js)

	output, err := client.GetItem(context.TODO(), &dynamodb.GetItemInput{
		TableName: aws.String("THENEWTABLE"),
		Key:       av,
	})

	if err != nil {
		return false, "", err.Error()
	}

	//get output
	attributevalue.UnmarshalMap(output.Item, &js)
	gotten := MarshalWrapper(js)
	expected := MarshalWrapper(values)

	return gotten == expected, expected, gotten
}
