package main

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ses"
)

func main() {
	// Create a new session in the us-west-2 region.
	// Replace us-west-2 with the AWS Region you're using for Amazon SES.
	sess, err := session.NewSessionWithOptions(session.Options{
		Profile: "email",
		Config: aws.Config{
			Region: aws.String("us-east-2"),
		},
	})

	Recipient := "kaedenle@gmail.com"
	Sender := "ucfarexperiences@gmail.com"
	CharSet := "UTF-8"
	TextBody := "This email was sent with Amazon SES using the AWS SDK for Go."
	Subject := "testing"

	// Create an SES session.
	svc := ses.New(sess)

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
		// Uncomment to use a configuration set
		//ConfigurationSetName: aws.String(ConfigurationSet),
	}
	result, err := svc.SendEmail(input)
	if err != nil {
		panic(err)
	}
	fmt.Println("Email Sent to address: " + Recipient)
	fmt.Println(result)
}
