package integrations

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/smtp"
	"strconv"
	"strings"

	"github.com/PrabhaharanNM/jenkins-mcp-agent/go-binary/internal/models"
)

// SendEmail sends an HTML build failure report via SMTP to the responsible team manager.
func SendEmail(req *models.AnalysisRequest, analysis *models.ClaudeAnalysis, teamMgr *models.TeamManager, htmlReport string, buildCtx *models.BuildContext) error {
	if req.Email.SmtpHost == "" {
		log.Println("[EMAIL] SMTP host not configured, skipping email")
		return nil
	}

	if teamMgr == nil || teamMgr.Email == "" {
		log.Println("[EMAIL] No recipient email address, skipping email")
		return nil
	}

	addr := req.Email.SmtpHost + ":" + strconv.Itoa(req.Email.SmtpPort)

	// Build the subject line
	subject := fmt.Sprintf("Build Failed: %s #%d - %s", buildCtx.JobName, buildCtx.BuildNumber, analysis.Category)

	// Build the From header
	from := req.Email.FromAddress
	fromHeader := from
	if req.Email.FromName != "" {
		fromHeader = fmt.Sprintf("%s <%s>", req.Email.FromName, from)
	}

	// Build MIME message
	var msg strings.Builder
	msg.WriteString(fmt.Sprintf("From: %s\r\n", fromHeader))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", teamMgr.Email))
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(htmlReport)

	messageBytes := []byte(msg.String())
	recipients := []string{teamMgr.Email}

	auth := smtp.PlainAuth("", req.Email.Username, req.Email.Password, req.Email.SmtpHost)

	if req.Email.EnableSsl {
		return sendWithTLS(addr, req.Email.SmtpHost, auth, from, recipients, messageBytes)
	}

	// Try STARTTLS first; if it fails (e.g. MailHog doesn't support it),
	// fall back to plain SMTP without any TLS.
	if err := sendWithStartTLS(addr, req.Email.SmtpHost, auth, from, recipients, messageBytes); err != nil {
		log.Printf("[EMAIL] STARTTLS failed (%v), falling back to plain SMTP", err)
		return sendPlain(addr, from, recipients, messageBytes)
	}
	return nil
}

// sendPlain connects using plain SMTP without any TLS (e.g., MailHog on port 1025).
func sendPlain(addr, from string, to []string, msg []byte) error {
	client, err := smtp.Dial(addr)
	if err != nil {
		log.Printf("[EMAIL] plain SMTP dial failed: %v", err)
		return fmt.Errorf("plain smtp dial failed: %w", err)
	}
	defer client.Close()

	// No AUTH for plain SMTP (MailHog doesn't require it)
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("smtp MAIL FROM failed: %w", err)
	}
	for _, recipient := range to {
		if err := client.Rcpt(recipient); err != nil {
			return fmt.Errorf("smtp RCPT TO failed for %s: %w", recipient, err)
		}
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp DATA failed: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("smtp write failed: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp data close failed: %w", err)
	}

	log.Printf("[EMAIL] Email sent successfully (plain SMTP) to %v", to)
	return client.Quit()
}

// sendWithTLS connects using implicit TLS (e.g., port 465).
func sendWithTLS(addr, host string, auth smtp.Auth, from string, to []string, msg []byte) error {
	tlsConfig := &tls.Config{
		ServerName: host,
	}

	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		log.Printf("[EMAIL] TLS dial failed: %v", err)
		return fmt.Errorf("tls dial failed: %w", err)
	}

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		log.Printf("[EMAIL] SMTP client creation failed: %v", err)
		return fmt.Errorf("smtp client creation failed: %w", err)
	}
	defer client.Close()

	if err := client.Auth(auth); err != nil {
		log.Printf("[EMAIL] SMTP auth failed: %v", err)
		return fmt.Errorf("smtp auth failed: %w", err)
	}

	if err := client.Mail(from); err != nil {
		return fmt.Errorf("smtp MAIL FROM failed: %w", err)
	}
	for _, recipient := range to {
		if err := client.Rcpt(recipient); err != nil {
			return fmt.Errorf("smtp RCPT TO failed for %s: %w", recipient, err)
		}
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp DATA failed: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("smtp write failed: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp data close failed: %w", err)
	}

	log.Printf("[EMAIL] Email sent successfully to %v", to)
	return client.Quit()
}

// sendWithStartTLS connects using STARTTLS (e.g., port 587).
func sendWithStartTLS(addr, host string, auth smtp.Auth, from string, to []string, msg []byte) error {
	client, err := smtp.Dial(addr)
	if err != nil {
		log.Printf("[EMAIL] SMTP dial failed: %v", err)
		return fmt.Errorf("smtp dial failed: %w", err)
	}
	defer client.Close()

	tlsConfig := &tls.Config{
		ServerName: host,
	}
	if err := client.StartTLS(tlsConfig); err != nil {
		log.Printf("[EMAIL] STARTTLS failed: %v", err)
		return fmt.Errorf("starttls failed: %w", err)
	}

	if err := client.Auth(auth); err != nil {
		log.Printf("[EMAIL] SMTP auth failed: %v", err)
		return fmt.Errorf("smtp auth failed: %w", err)
	}

	if err := client.Mail(from); err != nil {
		return fmt.Errorf("smtp MAIL FROM failed: %w", err)
	}
	for _, recipient := range to {
		if err := client.Rcpt(recipient); err != nil {
			return fmt.Errorf("smtp RCPT TO failed for %s: %w", recipient, err)
		}
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp DATA failed: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("smtp write failed: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp data close failed: %w", err)
	}

	log.Printf("[EMAIL] Email sent successfully to %v", to)
	return client.Quit()
}
