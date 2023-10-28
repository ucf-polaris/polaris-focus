package DatabaseFuncs

import (
	"context"
	"log"
	"polaris-api/pkg/Helpers"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go/aws"
)

func produceQueryResult(page *dynamodb.ScanPaginator) ([]map[string]interface{}, error) {
	p := []map[string]interface{}{}

	for page.HasMorePages() {
		out, err := page.NextPage(context.TODO())
		if err != nil {
			return nil, err
		}

		temp := []map[string]interface{}{}
		err = attributevalue.UnmarshalListOfMaps(out.Items, &temp)
		if err != nil {
			return nil, err
		}

		p = append(p, temp...)
	}

	return p, nil
}

func ScanDatabase(table string, client *dynamodb.Client) ([]map[string]interface{}, error) {
	scanInput := &dynamodb.ScanInput{
		// table name is a global variable
		TableName: &table,
	}

	paginator := dynamodb.NewScanPaginator(client, scanInput)
	ret, err := produceQueryResult(paginator)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func addColonToField(mapping map[string]interface{}) map[string]interface{} {
	ret := make(map[string]interface{})
	for k, v := range mapping {
		ret[":"+k] = v
	}

	return ret
}

func UpdateDatabase(conditional, query, table string, client *dynamodb.Client, keys, item_definition map[string]interface{}, isSet []string) (map[string]interface{}, error) {
	key_input, err := attributevalue.MarshalMap(keys)
	if err != nil {
		return nil, err
	}

	item_definition = addColonToField(item_definition)
	items, err := attributevalue.MarshalMap(item_definition)
	if err != nil {
		return nil, err
	}

	err = Helpers.ListToStringSet(
		isSet,
		items,
		true,
	)
	if err != nil {
		return nil, err
	}

	log.Println(items)

	updateInput := &dynamodb.UpdateItemInput{
		// table name is a global variable
		TableName: &table,
		// Partitiion key for user table is EventID
		Key: key_input,
		// "SET" update expression to update the item in the table.
		UpdateExpression:          aws.String(query),
		ExpressionAttributeValues: items,
		ReturnValues:              types.ReturnValueUpdatedNew,
	}

	if conditional != "" {
		updateInput.ConditionExpression = aws.String(conditional)
	}

	retValues, err := client.UpdateItem(context.Background(), updateInput)
	if err != nil {
		return nil, err
	}

	ret := map[string]interface{}{}
	attributevalue.UnmarshalMap(retValues.Attributes, &ret)

	return ret, nil
}
