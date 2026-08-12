package mail

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"mime"
	"net"
	"net/smtp"
	"strings"
	"time"

	"github.com/nassabiq/go-chi-boilerplate/internal/config"
)

type Sender struct {
	host     string
	port     int
	user     string
	password string
	from     string
	name     string
	useTLS   bool
}

type Message struct {
	To      string
	Subject string
	Text    string
	HTML    string
}

func NewSender(cfg *config.Config) *Sender {
	return &Sender{
		host:     cfg.SMTP.Host,
		port:     cfg.SMTP.Port,
		user:     cfg.SMTP.User,
		password: cfg.SMTP.Password,
		from:     cfg.SMTP.From,
		name:     cfg.SMTP.Name,
		useTLS:   cfg.SMTP.UseTLS,
	}
}

func (s *Sender) Send(ctx context.Context, message Message) error {
	address := fmt.Sprintf("%s:%d", s.host, s.port)

	dialer := &net.Dialer{
		Timeout: 10 * time.Second,
	}

	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return fmt.Errorf("connect to smtp server: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, s.host)
	if err != nil {
		return fmt.Errorf("create smtp client: %w", err)
	}
	defer client.Close()

	if s.useTLS {
		if err := client.StartTLS(&tls.Config{
			ServerName: s.host,
			MinVersion: tls.VersionTLS12,
		}); err != nil {
			return fmt.Errorf("start smtp tls: %w", err)
		}
	}

	if s.user != "" {
		auth := smtp.PlainAuth("", s.user, s.password, s.host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("authenticate smtp: %w", err)
		}
	}

	if err := client.Mail(s.from); err != nil {
		return fmt.Errorf("set smtp sender: %w", err)
	}

	if err := client.Rcpt(message.To); err != nil {
		return fmt.Errorf("set smtp recipient: %w", err)
	}

	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("open smtp message body: %w", err)
	}

	body := buildMessage(s.name, s.from, message)

	if _, err := writer.Write(body); err != nil {
		_ = writer.Close()
		return fmt.Errorf("write smtp message: %w", err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("close smtp message: %w", err)
	}

	if err := client.Quit(); err != nil {
		return fmt.Errorf("quit smtp client: %w", err)
	}

	return nil
}

func buildMessage(fromName string, fromEmail string, message Message) []byte {
	var body bytes.Buffer

	from := fromEmail
	if fromName != "" {
		from = fmt.Sprintf("%s <%s>", mime.QEncoding.Encode("UTF-8", fromName), fromEmail)
	}

	fmt.Fprintf(&body, "From: %s\r\n", from)
	fmt.Fprintf(&body, "To: %s\r\n", message.To)
	fmt.Fprintf(&body, "Subject: %s\r\n", mime.QEncoding.Encode("UTF-8", message.Subject))

	if message.HTML != "" && message.Text != "" {
		boundary := "go-chi-mail-boundary"

		body.WriteString("MIME-Version: 1.0\r\n")
		fmt.Fprintf(&body, "Content-Type: multipart/alternative; boundary=%q\r\n", boundary)
		body.WriteString("\r\n")

		fmt.Fprintf(&body, "--%s\r\n", boundary)
		body.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")

		body.WriteString(message.Text)
		body.WriteString("\r\n")

		fmt.Fprintf(&body, "--%s\r\n", boundary)

		body.WriteString(
			"Content-Type: text/html; charset=UTF-8\r\n\r\n",
		)

		body.WriteString(message.HTML)
		body.WriteString("\r\n")

		fmt.Fprintf(&body, "--%s--\r\n", boundary)

		return body.Bytes()
	}

	contentType := "text/plain"

	content := message.Text

	if message.HTML != "" {
		contentType = "text/html"
		content = message.HTML
	}

	body.WriteString("MIME-Version: 1.0\r\n")
	fmt.Fprintf(&body, "Content-Type: %s; charset=UTF-8\r\n", contentType)
	body.WriteString("\r\n")

	body.WriteString(
		strings.TrimSpace(content),
	)

	return body.Bytes()
}
