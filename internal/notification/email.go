// Package notification implements the notification service for sending price drop alerts to users
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

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"scraper/internal/models"
	"scraper/internal/proto"
)

// EmailService handles sending email notifications to users.
// It requires a database connection to look up user and product information.
type EmailService struct {
	db *gorm.DB // Database connection for user/product lookups
}

// NewEmailService creates a new email service instance.
//
// Parameters:
//   - db: Database connection for user/product lookups
//
// Returns:
//   - *EmailService: Configured email service
func NewEmailService(db *gorm.DB) *EmailService {
	return &EmailService{db: db}
}

// SendNotification implements the gRPC NotificationService interface.
// It handles incoming notification requests by:
// 1. Parsing the user ID and validating credentials
// 2. Extracting price information from the message
// 3. Sending an email notification about the price drop
//
// Parameters:
//   - ctx: Request context
//   - in: Notification request containing user ID, product ID, and message
//
// Returns:
//   - *proto.NotificationResponse: Success/failure response
//   - error: Any error that occurred during processing
func (s *NotificationServer) SendNotification(ctx context.Context, in *proto.NotificationRequest) (*proto.NotificationResponse, error) {
	// Log incoming request
	logrus.WithFields(logrus.Fields{
		"user_id":    in.UserId,
		"product_id": in.ProductId,
	}).Info("Received notification request")

	// Initialize email service if needed
	if s.emailService == nil {
		s.emailService = NewEmailService(s.db)
	}

	// Parse and validate user ID
	userID, err := strconv.ParseUint(in.UserId, 10, 32)
	if err != nil {
		logrus.WithError(err).Error("Error parsing user ID")
		return &proto.NotificationResponse{Success: false}, nil
	}

	// Check email credentials
	password := os.Getenv("EMAIL_APP_PASSWORD")
	if password == "" {
		logrus.Error("EMAIL_APP_PASSWORD not set")
		return nil, fmt.Errorf("email password not configured")
	}

	// Extract price information from message
	var oldPrice, newPrice float64
	_, err = fmt.Sscanf(in.Message, "Price dropped from %f to %f for", &oldPrice, &newPrice)
	if err != nil {
		logrus.WithError(err).Error("Error parsing price info")
		_, err = s.emailService.SendPriceDropNotification(uint(userID), uint(in.ProductId), 0, 0)
	} else {
		_, err = s.emailService.SendPriceDropNotification(uint(userID), uint(in.ProductId), oldPrice, newPrice)
	}

	// Log any email sending errors
	if err != nil {
		logrus.WithError(err).Error("Error sending email notification")
	}

	return &proto.NotificationResponse{Success: true}, nil
}

// SendMail sends an HTML email using the configured SMTP server.
// It supports TLS encryption and authentication.
//
// The function performs the following steps:
// 1. Gets SMTP configuration from environment variables
// 2. Sets up email headers and authentication
// 3. Establishes TLS connection to SMTP server
// 4. Sends the email with HTML content
//
// Environment Variables:
//   - EMAIL_SENDER: Sender email address (default: mailtrap username)
//   - EMAIL_APP_PASSWORD: SMTP password
//   - SMTP_HOST: SMTP server hostname (default: sandbox.smtp.mailtrap.io)
//   - SMTP_PORT: SMTP server port (default: 2525)
//
// Parameters:
//   - toEmail: Recipient's email address
//   - htmlContent: HTML content of the email
//   - subject: Email subject line
//
// Returns:
//   - error: Any error that occurred while sending the email
func (es *EmailService) SendMail(toEmail string, htmlContent, subject string) error {
	// Log attempt to send email
	logrus.WithFields(logrus.Fields{
		"to":      toEmail,
		"subject": subject,
	}).Info("Attempting to send email")

	// Get SMTP configuration from environment
	senderMail := os.Getenv("EMAIL_SENDER")
	password := os.Getenv("EMAIL_APP_PASSWORD")
	smtpHost := os.Getenv("SMTP_HOST")
	smtpPort := os.Getenv("SMTP_PORT")

	// Set default values if not configured
	if senderMail == "" {
		senderMail = "your-mailtrap-username"
	}
	if smtpHost == "" {
		smtpHost = "sandbox.smtp.mailtrap.io"
	}
	if smtpPort == "" {
		smtpPort = "2525"
	}

	// Setup email headers
	headers := make(map[string]string)
	headers["From"] = senderMail
	headers["To"] = toEmail
	headers["Subject"] = subject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "text/html; charset=\"utf-8\""

	// Configure TLS and authentication
	config := &tls.Config{ServerName: smtpHost}
	auth := smtp.PlainAuth("", senderMail, password, smtpHost)

	// Connect to SMTP server
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

// SendPriceDropNotification sends an email notification to a user when a product's price drops.
// The email includes:
// - Product name and price change details
// - Amount saved and savings percentage
// - Link to view the product
// - Styled HTML template with a professional layout
//
// The function performs these steps:
// 1. Validates database connection
// 2. Retrieves user and product information
// 3. Extracts price and currency information
// 4. Generates a styled HTML email using a template
// 5. Sends the email using the SendMail function
//
// Parameters:
//   - userID: ID of the user to notify
//   - productID: ID of the product with price drop
//   - oldPrice: Previous price of the product
//   - newPrice: New reduced price of the product
//
// Returns:
//   - bool: True if notification was sent successfully
//   - error: Any error that occurred during the process
func (es *EmailService) SendPriceDropNotification(userID uint, productID uint, oldPrice, newPrice float64) (bool, error) {
	// Validate database connection
	if es.db == nil {
		logrus.Error("Database connection is nil")
		return false, fmt.Errorf("database connection is nil")
	}

	// Retrieve user information
	var user models.User
	if err := es.db.First(&user, userID).Error; err != nil {
		logrus.WithError(err).Error("Failed to find user")
		return false, fmt.Errorf("failed to find user: %w", err)
	}

	// Retrieve product information
	var product models.Product
	if err := es.db.First(&product, productID).Error; err != nil {
		logrus.WithError(err).Error("Failed to find product")
		return false, fmt.Errorf("failed to find product: %w", err)
	}

	// Extract product name and price information
	name := product.Name
	var priceInfo map[string]interface{}
	if err := json.Unmarshal(product.PriceInfo, &priceInfo); err != nil {
		logrus.WithError(err).Error("Failed to unmarshal price info")
		return false, fmt.Errorf("failed to unmarshal price info: %w", err)
	}

	// Get currency from price info or use default
	currency := "AED"
	if curr, ok := priceInfo["currency"].(string); ok {
		currency = curr
	}

	// HTML email template with styling
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

	// Parse email template
	t, err := template.New("priceDropEmail").Parse(tmpl)
	if err != nil {
		logrus.WithError(err).Error("Failed to parse email template")
		return false, fmt.Errorf("failed to parse email template: %w", err)
	}

	// Calculate savings and percentage
	savings := oldPrice - newPrice
	savingsPercent := (savings / oldPrice) * 100

	// Prepare template data
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
		SavingsPercent: float64(int(savingsPercent*100)) / 100, // Round to 2 decimal places
		ProductID:      productID,
	}

	// Execute template with data
	err = t.Execute(&buf, data)
	if err != nil {
		logrus.WithError(err).Error("Failed to execute email template")
		return false, fmt.Errorf("failed to execute email template: %w", err)
	}

	// Prepare email content
	htmlContent := buf.String()
	subject := fmt.Sprintf("Price Drop Alert! %s is now cheaper", name)
	err = es.SendMail(user.Email, htmlContent, subject)
	if err != nil {
		logrus.WithError(err).Error("Failed to send email")
		return false, err
	}

	return true, nil
}