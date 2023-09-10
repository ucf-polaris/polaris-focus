package main

import (
	"errors"
	"log"
	"strconv"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ses"
)

var emailClient *ses.SES

func init() {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))

	// Create an SES session.
	emailClient = ses.New(sess)
}

func main() {
	lambda.Start(handler)
}

func handler(request map[string]interface{}) error {

	var email string
	var code int

	if val, ok := request["email"].(string); ok {
		email = val
	} else {
		return errors.New("email not found in body")
	}

	if val, ok := request["code"].(float64); ok {
		code = int(val)
	} else {
		return errors.New("code not found in body")
	}

	Recipient := email
	Sender := "ucfarexperiences@gmail.com"
	CharSet := "UTF-8"
	TextBody := "Your registration code for UCF Polaris is " + strconv.Itoa(code) + "."
	Subject := "UCF Polaris Registration Code"

	// Attempt to send the email.
	input := &ses.SendEmailInput{
		Destination: &ses.Destination{
			CcAddresses: []*string{},
			ToAddresses: []*string{
				aws.String(Recipient),
			},
		},
		Message: &ses.Message{
			Body: &ses.Body{
				Text: &ses.Content{
					Charset: aws.String(CharSet),
					Data:    aws.String(TextBody),
				},
			},
			Subject: &ses.Content{
				Charset: aws.String(CharSet),
				Data:    aws.String(Subject),
			},
		},
		Source: aws.String(Sender),
	}
	result, err := emailClient.SendEmail(input)
	if err != nil {
		return err
	}

	log.Println("Email Sent to address: " + Recipient)
	log.Println(result)

	return nil
}
