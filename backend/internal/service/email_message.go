package service

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"net/textproto"
	"strings"
	"time"
)

// smtpMessage 保存 SMTP 信封地址与完整 MIME 报文。
type smtpMessage struct {
	envelopeFrom string
	envelopeTo   string
	messageID    string
	data         []byte
}

// EmailAttachment 是待写入 MIME 邮件的内存附件。
type EmailAttachment struct {
	Name        string
	ContentType string
	Data        []byte
}

// buildSMTPMessage 构造符合邮件协议的 HTML MIME 报文。
func buildSMTPMessage(config *SMTPConfig, to, subject, body string) (smtpMessage, error) {
	return buildSMTPMessageWithAttachments(config, to, subject, body, nil)
}

// buildSMTPMessageWithAttachments 构造含受控附件的 multipart MIME 报文。
func buildSMTPMessageWithAttachments(config *SMTPConfig, to, subject, body string, attachments []EmailAttachment) (smtpMessage, error) {
	if config == nil {
		return smtpMessage{}, errors.New("missing SMTP configuration")
	}

	fromAddress, err := parseSMTPAddress(config.From, "from")
	if err != nil {
		return smtpMessage{}, err
	}
	recipientAddress, err := parseSMTPAddress(to, "recipient")
	if err != nil {
		return smtpMessage{}, err
	}
	messageID, err := generateEmailMessageID(fromAddress.Address, config.Host)
	if err != nil {
		return smtpMessage{}, fmt.Errorf("generate message ID: %w", err)
	}

	fromName := sanitizeEmailHeader(config.FromName)
	if strings.TrimSpace(fromName) == "" {
		fromName = fromAddress.Name
	}
	fromHeader := (&mail.Address{
		Name:    fromName,
		Address: fromAddress.Address,
	}).String()
	toHeader := (&mail.Address{
		Name:    recipientAddress.Name,
		Address: recipientAddress.Address,
	}).String()
	subjectHeader := mime.QEncoding.Encode("UTF-8", sanitizeEmailHeader(subject))

	var message bytes.Buffer
	fmt.Fprintf(&message, "From: %s\r\n", fromHeader)
	fmt.Fprintf(&message, "To: %s\r\n", toHeader)
	fmt.Fprintf(&message, "Date: %s\r\n", time.Now().UTC().Format(time.RFC1123Z))
	fmt.Fprintf(&message, "Message-ID: %s\r\n", messageID)
	fmt.Fprintf(&message, "Subject: %s\r\n", subjectHeader)
	fmt.Fprint(&message, "MIME-Version: 1.0\r\n")
	if len(attachments) == 0 {
		fmt.Fprint(&message, "Content-Type: text/html; charset=UTF-8\r\n"+
			"Content-Transfer-Encoding: quoted-printable\r\n\r\n")
		bodyWriter := quotedprintable.NewWriter(&message)
		if _, err := bodyWriter.Write([]byte(body)); err != nil {
			return smtpMessage{}, fmt.Errorf("encode email body: %w", err)
		}
		if err := bodyWriter.Close(); err != nil {
			return smtpMessage{}, fmt.Errorf("close email body encoder: %w", err)
		}
	} else if err := writeMultipartEmail(&message, body, attachments); err != nil {
		return smtpMessage{}, err
	}

	return smtpMessage{
		envelopeFrom: fromAddress.Address,
		envelopeTo:   recipientAddress.Address,
		messageID:    messageID,
		data:         message.Bytes(),
	}, nil
}

// writeMultipartEmail 写入 HTML 正文与经过编码的附件，避免附件内容影响邮件头。
func writeMultipartEmail(message *bytes.Buffer, body string, attachments []EmailAttachment) error {
	writer := multipart.NewWriter(message)
	fmt.Fprintf(message, "Content-Type: multipart/mixed; boundary=%q\r\n\r\n", writer.Boundary())
	bodyHeader := textproto.MIMEHeader{}
	bodyHeader.Set("Content-Type", "text/html; charset=UTF-8")
	bodyHeader.Set("Content-Transfer-Encoding", "quoted-printable")
	bodyPart, err := writer.CreatePart(bodyHeader)
	if err != nil {
		return fmt.Errorf("create html email part: %w", err)
	}
	bodyWriter := quotedprintable.NewWriter(bodyPart)
	if _, err := bodyWriter.Write([]byte(body)); err != nil {
		return fmt.Errorf("encode html email part: %w", err)
	}
	if err := bodyWriter.Close(); err != nil {
		return fmt.Errorf("close html email encoder: %w", err)
	}
	for _, attachment := range attachments {
		name := sanitizeEmailAttachmentName(attachment.Name)
		if name == "" || len(attachment.Data) == 0 {
			return errors.New("email attachment name and content are required")
		}
		contentType := strings.TrimSpace(attachment.ContentType)
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		disposition := mime.FormatMediaType("attachment", map[string]string{"filename": name})
		header := textproto.MIMEHeader{}
		header.Set("Content-Type", contentType)
		header.Set("Content-Disposition", disposition)
		header.Set("Content-Transfer-Encoding", "base64")
		part, err := writer.CreatePart(header)
		if err != nil {
			return fmt.Errorf("create email attachment part: %w", err)
		}
		encoded := base64.NewEncoder(base64.StdEncoding, part)
		if _, err := encoded.Write(attachment.Data); err != nil {
			return fmt.Errorf("encode email attachment: %w", err)
		}
		if err := encoded.Close(); err != nil {
			return fmt.Errorf("close email attachment encoder: %w", err)
		}
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("close multipart email: %w", err)
	}
	return nil
}

func sanitizeEmailAttachmentName(name string) string {
	name = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(name, "\r", ""), "\n", ""))
	if len(name) > 200 {
		name = name[:200]
	}
	return name
}

// parseSMTPAddress 校验单个 SMTP 地址并拒绝换行注入。
func parseSMTPAddress(value, field string) (*mail.Address, error) {
	if strings.ContainsAny(value, "\r\n") {
		return nil, fmt.Errorf("invalid SMTP %s address: contains a line break", field)
	}

	cleaned := strings.TrimSpace(value)
	address, err := mail.ParseAddress(cleaned)
	if err != nil || strings.TrimSpace(address.Address) == "" {
		if err == nil {
			err = errors.New("address is empty")
		}
		return nil, fmt.Errorf("invalid SMTP %s address: %w", field, err)
	}
	return address, nil
}

// generateEmailMessageID 生成随机且带发送域名的 Message-ID。
func generateEmailMessageID(fromAddress, smtpHost string) (string, error) {
	randomID := make([]byte, 16)
	if _, err := rand.Read(randomID); err != nil {
		return "", err
	}

	domain := strings.TrimSpace(sanitizeEmailHeader(smtpHost))
	if at := strings.LastIndexByte(fromAddress, '@'); at >= 0 && at < len(fromAddress)-1 {
		domain = fromAddress[at+1:]
	}
	domain = strings.Trim(domain, "[]<>")
	if domain == "" {
		domain = "localhost"
	}

	return fmt.Sprintf("<%s@%s>", hex.EncodeToString(randomID), domain), nil
}
