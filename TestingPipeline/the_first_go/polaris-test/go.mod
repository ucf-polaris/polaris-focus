require (
	github.com/aws/aws-lambda-go v1.36.1
	github.com/aws/aws-sdk-go v1.45.6
	github.com/aws/aws-sdk-go-v2 v1.21.0
	github.com/aws/aws-sdk-go-v2/config v1.18.39
	github.com/aws/aws-sdk-go-v2/credentials v1.13.37
	github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue v1.10.39
	github.com/aws/aws-sdk-go-v2/service/dynamodb v1.21.5
	github.com/google/uuid v1.3.1
)

replace gopkg.in/yaml.v2 => gopkg.in/yaml.v2 v2.2.8

module polaris-test

go 1.16
