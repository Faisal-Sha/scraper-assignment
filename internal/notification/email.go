package notification

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"html/template"
	"net/smtp"
	"os"
	"strconv"

	"scraper/internal/models"
	"scraper/internal/proto"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type EmailService struct {
	db *gorm.DB
}

func NewEmailService(db *gorm.DB) *EmailService {
	return &EmailService{db: db}
}

func (s *NotificationServer) SendNotification(ctx context.Context, in *proto.NotificationRequest) (*proto.NotificationResponse, error) {
	logrus.WithFields(logrus.Fields{
		"user_id":    in.UserId,
		"product_id": in.ProductId,
	}).Info("Received notification request")

	if s.emailService == nil {
		s.emailService = NewEmailService(s.db)
	}

	userID, err := strconv.ParseUint(in.UserId, 10, 32)
	if err != nil {
		logrus.WithError(err).Error("Error parsing user ID")
		return &proto.NotificationResponse{Success: false}, nil
	}

	password := os.Getenv("EMAIL_APP_PASSWORD")
	if password == "" {
		logrus.Error("EMAIL_APP_PASSWORD not set")
		return nil, fmt.Errorf("email password not configured")
	}

	var oldPrice, newPrice float64
	_, err = fmt.Sscanf(in.Message, "Price dropped from %f to %f for", &oldPrice, &newPrice)
	if err != nil {
		logrus.WithError(err).Error("Error parsing price info")
		_, err = s.emailService.SendPriceDropNotification(uint(userID), uint(in.ProductId), 0, 0)
	} else {
		_, err = s.emailService.SendPriceDropNotification(uint(userID), uint(in.ProductId), oldPrice, newPrice)
	}

	if err != nil {
		logrus.WithError(err).Error("Error sending email notification")
	}

	return &proto.NotificationResponse{Success: true}, nil
}

func (es *EmailService) SendMail(toEmail string, htmlContent, subject string) error {
	logrus.WithFields(logrus.Fields{
		"to":      toEmail,
		"subject": subject,
	}).Info("Attempting to send email")

	senderMail := os.Getenv("EMAIL_SENDER")
	password := os.Getenv("EMAIL_APP_PASSWORD")
	smtpHost := os.Getenv("SMTP_HOST")
	smtpPort := os.Getenv("SMTP_PORT")

	if senderMail == "" {
		senderMail = "your-mailtrap-username"
	}
	if smtpHost == "" {
		smtpHost = "sandbox.smtp.mailtrap.io"
	}
	if smtpPort == "" {
		smtpPort = "2525"
	}

	headers := make(map[string]string)
	headers["From"] = senderMail
	headers["To"] = toEmail
	headers["Subject"] = subject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "text/html; charset=\"utf-8\""

	config := &tls.Config{ServerName: smtpHost}
	auth := smtp.PlainAuth("", senderMail, password, smtpHost)

	client, err := smtp.Dial(smtpHost + ":" + smtpPort)
	if err != nil {
		logrus.WithError(err).Error("Error dialing SMTP server")
		return err
	}
	defer client.Close()

	if err = client.StartTLS(config); err != nil {
		logrus.WithError(err).Error("Error starting TLS")
		return err
	}

	if err = client.Auth(auth); err != nil {
		logrus.WithError(err).Error("Error authenticating")
		return err
	}

	if err = client.Mail(senderMail); err != nil {
		logrus.WithError(err).Error("Error setting sender")
		return err
	}

	if err = client.Rcpt(toEmail); err != nil {
		logrus.WithError(err).Error("Error setting recipient")
		return err
	}

	w, err := client.Data()
	if err != nil {
		logrus.WithError(err).Error("Error creating data writer")
		return err
	}

	var msg bytes.Buffer
	for k, v := range headers {
		msg.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	msg.WriteString("\r\n")
	msg.WriteString(htmlContent)

	_, err = w.Write(msg.Bytes())
	if err != nil {
		logrus.WithError(err).Error("Error writing email content")
		return err
	}

	err = w.Close()
	if err != nil {
		logrus.WithError(err).Error("Error closing data writer")
		return err
	}

	logrus.WithField("to", toEmail).Info("Email sent successfully")
	return nil
}

func (es *EmailService) SendPriceDropNotification(userID uint, productID uint, oldPrice, newPrice float64) (bool, error) {
	if es.db == nil {
		logrus.Error("Database connection is nil")
		return false, fmt.Errorf("database connection is nil")
	}

	var user models.User
	if err := es.db.First(&user, userID).Error; err != nil {
		logrus.WithError(err).Error("Failed to find user")
		return false, fmt.Errorf("failed to find user: %w", err)
	}

	var product models.Product
	if err := es.db.First(&product, productID).Error; err != nil {
		logrus.WithError(err).Error("Failed to find product")
		return false, fmt.Errorf("failed to find product: %w", err)
	}

	name := product.Name
	var priceInfo map[string]interface{}
	if err := json.Unmarshal(product.PriceInfo, &priceInfo); err != nil {
		logrus.WithError(err).Error("Failed to unmarshal price info")
		return false, fmt.Errorf("failed to unmarshal price info: %w", err)
	}

	currency := "AED"
	if curr, ok := priceInfo["currency"].(string); ok {
		currency = curr
	}

	tmpl := `
	<html>
	<body style="font-family: Arial, sans-serif; color: #333; line-height: 1.6;">
		<div style="max-width: 600px; margin: 0 auto; padding: 20px; border: 1px solid #eee; border-radius: 10px;">
			<h2 style="color: #e91e63; margin-bottom: 20px;">Price Drop Alert!</h2>
			<p>Hi <b>{{.UserName}}</b>,</p>
			<p>Good news! A product you've favorited has dropped in price:</p>
			<div style="background-color: #f9f9f9; padding: 15px; border-radius: 5px; margin: 20px 0;">
				<h3 style="margin-top: 0; color: #333;">{{.ProductName}}</h3>
				<p><b>Price dropped from:</b> <span style="text-decoration: line-through;">{{.OldPrice}} {{.Currency}}</span></p>
				<p><b>New price:</b> <span style="color:Nimble, sans-serif; color: #e91e63; font-weight: bold; font-size: 1.2em;">{{.NewPrice}} {{.Currency}}</span></p>
				<p><b>You save:</b> <span style="color: #4caf50;">{{.Savings}} {{.Currency}} ({{.SavingsPercent}}%)</span></p>
			</div>
			<p>Don't miss out on this great deal!</p>
			<a href="http://localhost:8080/products/{{.ProductID}}" style="display: inline-block; background-color: #e91e63; color: white; padding: 10px 20px; text-decoration: none; border-radius: 5px; margin-top: 15px;">View Product</a>
			<p style="margin-top: 30px; font-size: 0.9em; color: #777;">
				This notification was sent because you've favorited this product.
				<br>Happy Shopping!
			</p>
		</div>
	</body>
	</html>`

	t, err := template.New("priceDropEmail").Parse(tmpl)
	if err != nil {
		logrus.WithError(err).Error("Failed to parse email template")
		return false, fmt.Errorf("failed to parse email template: %w", err)
	}

	savings := oldPrice - newPrice
	savingsPercent := (savings / oldPrice) * 100

	var buf bytes.Buffer
	data := struct {
		UserName       string
		ProductName    string
		OldPrice       float64
		NewPrice       float64
		Currency       string
		Savings        float64
		SavingsPercent float64
		ProductID      uint
	}{
		UserName:       user.Name,
		ProductName:    name,
		OldPrice:       oldPrice,
		NewPrice:       newPrice,
		Currency:       currency,
		Savings:        savings,
		SavingsPercent: float64(int(savingsPercent*100)) / 100,
		ProductID:      productID,
	}

	err = t.Execute(&buf, data)
	if err != nil {
		logrus.WithError(err).Error("Failed to execute email template")
		return false, fmt.Errorf("failed to execute email template: %w", err)
	}

	htmlContent := buf.String()
	subject := fmt.Sprintf("Price Drop Alert! %s is now cheaper", name)
	err = es.SendMail(user.Email, htmlContent, subject)
	if err != nil {
		logrus.WithError(err).Error("Failed to send email")
		return false, err
	}

	return true, nil
}