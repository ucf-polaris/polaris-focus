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

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go/aws"
)

func ProduceRandomData(schema map[string]string) (map[string]types.AttributeValue, map[string]interface{}, error) {

	item := make(map[string]types.AttributeValue)
	values := make(map[string]interface{})

	rand.Seed(time.Now().UnixNano())

	for k, v := range schema {
		switch v {
		case "N":
			val := rand.Intn(10000)
			item[k] = &types.AttributeValueMemberN{Value: strconv.Itoa(val)}
			values[k] = val
		case "BOOL":
			val := true
			item[k] = &types.AttributeValueMemberBOOL{Value: val}
			values[k] = val
		case "S":
			val := rand.Int()
			item[k] = &types.AttributeValueMemberS{Value: strconv.Itoa(val)}
			values[k] = strconv.Itoa(val)
		case "L":
			val := []string{"this", "is", "a", "mighty", "test"}
			av, err := attributevalue.MarshalList(val)
			if err != nil {
				return nil, nil, err
			}
			item[k] = &types.AttributeValueMemberL{Value: av}
			values[k] = val
		case "M":
			val := map[string]int{
				"This":      1,
				"That":      2,
				"OverWhere": 3,
			}
			av, err := attributevalue.MarshalMap(val)
			if err != nil {
				return nil, nil, err
			}
			item[k] = &types.AttributeValueMemberM{Value: av}
			values[k] = val
		default:
			return nil, nil, errors.New("invalid datatype")
		}
	}

	return item, values, nil
}

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

// change to clear table
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

func AddToTable(client *dynamodb.Client, partition string, sort string, schema map[string]string) (map[string]interface{}, error) {
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

func extractSchema(full map[string]interface{}) (map[string]string, error) {
	ret := make(map[string]string)

	schem, ok := full["schema"]
	if !ok {
		return nil, errors.New("something went wrong when extracting schema")
	} else {
		js, _ := json.Marshal(schem)
		json.Unmarshal(js, &ret)
	}
	return ret, nil
}

func importConfigs() (map[string]string, string, string, error) {
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

func MarshalWrapper(M map[string]interface{}) string {
	js, _ := json.Marshal(M)
	return string(js)
}

func ProduceToken(M map[string]interface{}) (map[string]interface{}, map[string]interface{}) {
	tokens := map[string]interface{}{
		"token":        "tkn",
		"refreshToken": "rfsTkn",
	}
	M["token"] = tokens["token"]
	M["refreshToken"] = tokens["refreshToken"]

	return M, tokens
}

func ProduceIncorrectKeys(partition string, sort string, schema map[string]string, correct map[string]interface{}) map[string]interface{} {
	//prepare key schema
	tmp := map[string]string{
		partition: schema[partition],
	}
	if sort != "" {
		tmp[sort] = schema[sort]
	}

	_, wrong_vals, _ := ProduceRandomData(tmp)
	//if (somehow) wrong_vals and correct are equal, roll again
	for reflect.DeepEqual(wrong_vals, correct) {
		_, wrong_vals, _ = ProduceRandomData(tmp)
	}

	return wrong_vals
}
