package email

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"net/smtp"
)

type Config struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
}

type Service struct {
	config Config
}

func NewService(config Config) *Service {
	return &Service{config: config}
}

func (s *Service) Send(
	to string,
	subject string,
	templatePath string,
	data any,
) error {
	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		return fmt.Errorf("parse email template: %w", err)
	}

	var body bytes.Buffer

	if err := tmpl.Execute(&body, data); err != nil {
		return fmt.Errorf("execute email template: %w", err)
	}

	message := fmt.Sprintf(
		"From: %s\r\n"+
			"To: %s\r\n"+
			"Subject: %s\r\n"+
			"MIME-Version: 1.0\r\n"+
			"Content-Type: text/html; charset=UTF-8\r\n"+
			"\r\n"+
			"%s",
		s.config.From,
		to,
		subject,
		body.String(),
	)

	auth := smtp.PlainAuth(
		"",
		s.config.Username,
		s.config.Password,
		s.config.Host,
	)

	return smtp.SendMail(
		s.config.Host+":"+s.config.Port,
		auth,
		s.config.From,
		[]string{to},
		[]byte(message),
	)
}

func (s *Service) SendAsync(
	to string,
	subject string,
	templatePath string,
	data any,
) {
	go func() {
		if err := s.Send(to, subject, templatePath, data); err != nil {
			log.Printf("email send failed: %v", err)
		}
	}()
}
