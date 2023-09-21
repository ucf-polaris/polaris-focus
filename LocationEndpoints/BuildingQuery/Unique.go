package main

import (
	"context"
	"math"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
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

func pointInRadius(radius float64, lat float64, long float64, myLat float64, myLong float64) bool {
	return math.Sqrt(math.Pow(myLat-lat, 2)+math.Pow(myLong-long, 2)) <= radius
}

func filterByRadius(M []map[string]interface{}, radius float64, lat float64, long float64) []map[string]interface{} {
	ret := []map[string]interface{}{}
	for _, e := range M {
		myLat, _ := e["BuildingLat"].(float64)
		myLong, _ := e["BuildingLong"].(float64)

		if pointInRadius(radius, lat, long, myLat, myLong) {
			ret = append(ret, e)
		}
	}

	return ret
}
