package service

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

// MailMessage 定义邮件消息体。
type MailMessage struct {
	To       []string
	Subject  string
	TextBody string
}

// MailSender 邮件发送器能力抽象。
type MailSender interface {
	Send(ctx context.Context, config EmailConfig, message MailMessage) error
}

type smtpMailSender struct{}

// NewSMTPMailSender 创建 SMTP 邮件发送器。
func NewSMTPMailSender() MailSender {
	return &smtpMailSender{}
}

func (s *smtpMailSender) Send(ctx context.Context, config EmailConfig, message MailMessage) error {
	if !config.Enabled {
		return fmt.Errorf("%w: email config disabled", ErrPasswordResetEmailDisabled)
	}
	fromAddress := strings.TrimSpace(config.FromEmail)
	if fromAddress == "" {
		return fmt.Errorf("%w: from email is empty", ErrPasswordResetEmailDisabled)
	}
	if strings.TrimSpace(config.SMTP.Host) == "" || config.SMTP.Port <= 0 {
		return fmt.Errorf("%w: smtp host/port invalid", ErrPasswordResetEmailDisabled)
	}

	recipients := normalizeMailRecipients(message.To)
	if len(recipients) == 0 {
		return fmt.Errorf("%w: recipients are empty", ErrPasswordResetEmailSendFailed)
	}

	fromMailbox := mail.Address{
		Name:    strings.TrimSpace(config.FromName),
		Address: fromAddress,
	}
	replyTo := strings.TrimSpace(config.ReplyTo)
	mailText := buildTextMailPayload(fromMailbox, replyTo, recipients, message)

	smtpHost := strings.TrimSpace(config.SMTP.Host)
	serverAddr := net.JoinHostPort(smtpHost, fmt.Sprintf("%d", config.SMTP.Port))
	connectTimeout := time.Duration(config.SMTP.ConnectTimeoutMS) * time.Millisecond
	if connectTimeout <= 0 {
		connectTimeout = 3 * time.Second
	}
	sendTimeout := time.Duration(config.SMTP.SendTimeoutMS) * time.Millisecond
	if sendTimeout <= 0 {
		sendTimeout = 5 * time.Second
	}

	dialer := &net.Dialer{Timeout: connectTimeout}
	security := strings.ToLower(strings.TrimSpace(config.SMTP.Security))
	if security == "" {
		security = "starttls"
	}

	var conn net.Conn
	var err error
	switch security {
	case "tls":
		conn, err = tls.DialWithDialer(dialer, "tcp", serverAddr, &tls.Config{
			ServerName: smtpHost,
			MinVersion: tls.VersionTLS12,
		})
	default:
		conn, err = dialer.DialContext(ctx, "tcp", serverAddr)
	}
	if err != nil {
		return fmt.Errorf("%w: dial smtp failed: %v", ErrPasswordResetEmailSendFailed, err)
	}
	defer func() {
		_ = conn.Close()
	}()

	_ = conn.SetDeadline(time.Now().Add(sendTimeout))
	client, err := smtp.NewClient(conn, smtpHost)
	if err != nil {
		return fmt.Errorf("%w: create smtp client failed: %v", ErrPasswordResetEmailSendFailed, err)
	}
	defer func() {
		_ = client.Close()
	}()

	if security == "starttls" {
		ok, _ := client.Extension("STARTTLS")
		if !ok {
			return fmt.Errorf("%w: smtp server does not support STARTTLS", ErrPasswordResetEmailSendFailed)
		}
		if err := client.StartTLS(&tls.Config{
			ServerName: smtpHost,
			MinVersion: tls.VersionTLS12,
		}); err != nil {
			return fmt.Errorf("%w: starttls failed: %v", ErrPasswordResetEmailSendFailed, err)
		}
	}

	username := strings.TrimSpace(config.SMTP.Username)
	if username != "" {
		auth := smtp.PlainAuth("", username, config.SMTP.PasswordCiphertext, smtpHost)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("%w: smtp auth failed: %v", ErrPasswordResetEmailSendFailed, err)
		}
	}

	if err := client.Mail(fromAddress); err != nil {
		return fmt.Errorf("%w: smtp MAIL FROM failed: %v", ErrPasswordResetEmailSendFailed, err)
	}
	for _, recipient := range recipients {
		if err := client.Rcpt(recipient); err != nil {
			return fmt.Errorf("%w: smtp RCPT TO failed: %v", ErrPasswordResetEmailSendFailed, err)
		}
	}

	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("%w: smtp DATA failed: %v", ErrPasswordResetEmailSendFailed, err)
	}
	if _, err := writer.Write([]byte(mailText)); err != nil {
		_ = writer.Close()
		return fmt.Errorf("%w: write mail body failed: %v", ErrPasswordResetEmailSendFailed, err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("%w: finalize mail body failed: %v", ErrPasswordResetEmailSendFailed, err)
	}

	if err := client.Quit(); err != nil {
		return fmt.Errorf("%w: smtp quit failed: %v", ErrPasswordResetEmailSendFailed, err)
	}
	return nil
}

func normalizeMailRecipients(raw []string) []string {
	if len(raw) == 0 {
		return []string{}
	}
	result := make([]string, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))
	for _, item := range raw {
		normalized := strings.ToLower(strings.TrimSpace(item))
		if normalized == "" || !strings.Contains(normalized, "@") {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result
}

func buildTextMailPayload(
	from mail.Address,
	replyTo string,
	recipients []string,
	message MailMessage,
) string {
	subject := strings.TrimSpace(message.Subject)
	if subject == "" {
		subject = "PlainDoc Notification"
	}
	body := strings.TrimSpace(message.TextBody)
	if body == "" {
		body = "PlainDoc Notification"
	}

	var payload strings.Builder
	writer := bufio.NewWriter(&payload)
	_, _ = writer.WriteString(fmt.Sprintf("From: %s\r\n", from.String()))
	_, _ = writer.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(recipients, ", ")))
	if trimmedReplyTo := strings.TrimSpace(replyTo); trimmedReplyTo != "" {
		_, _ = writer.WriteString(fmt.Sprintf("Reply-To: %s\r\n", trimmedReplyTo))
	}
	_, _ = writer.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	_, _ = writer.WriteString("MIME-Version: 1.0\r\n")
	_, _ = writer.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	_, _ = writer.WriteString("Content-Transfer-Encoding: 8bit\r\n")
	_, _ = writer.WriteString("\r\n")
	_, _ = writer.WriteString(body)
	_, _ = writer.WriteString("\r\n")
	_ = writer.Flush()
	return payload.String()
}
