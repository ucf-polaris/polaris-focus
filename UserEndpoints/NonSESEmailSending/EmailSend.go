package main

import (
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/aws/aws-lambda-go/lambda"
	"gopkg.in/gomail.v2"
)

var password string

func init() {
	password = os.Getenv("APP_PASSWORD")
}

func main() {
	lambda.Start(handler)
	/*handler(
		map[string]interface{}{
			"email": "kaedenle@gmail.com",
			"code":  1.0,
		},
	)*/
}

func handler(request map[string]interface{}) error {

	var email string
	var code int
	var emailType int

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

	if val, ok := request["type"].(float64); ok {
		emailType = int(val)
	} else {
		emailType = 0
	}

	EmailTemplates := []struct {
		Body    string
		Subject string
	}{
		{
			Body:    "You've registered for a new UCF Polaris account. The code to activate your account is " + strconv.Itoa(code),
			Subject: "UCF Polaris Registration Code",
		},
		{
			Body:    "The code to reset your password is " + strconv.Itoa(code) + ". Once you enter this code into the app you'll be redirected to fields to reset your password",
			Subject: "UCF Polaris Password Recovery Code",
		},
	}

	template := EmailTemplates[emailType%len(EmailTemplates)]
	sender := "ucfarexperiences@gmail.com"

	m := gomail.NewMessage()

	// Set E-Mail sender
	m.SetHeader("From", sender)

	// Set E-Mail receivers
	m.SetHeader("To", email)

	// Set E-Mail subject
	m.SetHeader("Subject", template.Subject)

	// Set E-Mail body. You can set plain text or html with text/html
	m.SetBody("text/plain", template.Body)

	// Settings for SMTP server
	d := gomail.NewDialer("smtp.gmail.com", 587, sender, password)

	// This is only needed when SSL/TLS certificate is not valid on server.
	// In production this should be set to false.
	d.TLSConfig = &tls.Config{InsecureSkipVerify: true}

	// Now send E-Mail
	if err := d.DialAndSend(m); err != nil {
		fmt.Println(err)
		panic(err)
	}

	log.Println("Email Sent")

	return nil
}
