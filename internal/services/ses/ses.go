// Package ses provides email notification services via AWS SES
package ses

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/ses/types"
	"go.uber.org/zap"

	appConfig "loan-eligibility-engine/internal/config"
	"loan-eligibility-engine/internal/models"
	"loan-eligibility-engine/internal/utils"
)

// Service handles SES email operations
type Service struct {
	client      *ses.Client
	fromEmail   string
	templateDir string
}

// EmailParams represents parameters for sending an email
type EmailParams struct {
	To        string
	Subject   string
	HTMLBody  string
	TextBody  string
	ReplyTo   string
	CC        []string
	BCC       []string
	ConfigSet string
}

// MatchNotificationParams contains data for match notification email
type MatchNotificationParams struct {
	UserName     string
	UserEmail    string
	MatchCount   int
	TopMatches   []MatchInfo
	DashboardURL string
}

// MatchInfo contains info about a single match for email
type MatchInfo struct {
	ProductName      string
	Provider         string
	InterestRateMin  float64
	InterestRateMax  float64
	MaxLoanAmount    float64
	EligibilityScore float64
}

// SendEmailResult contains the result of sending an email
type SendEmailResult struct {
	MessageID string
	SentAt    time.Time
}

// NewService creates a new SES service
func NewService(ctx context.Context) (*Service, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	appCfg, err := appConfig.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load app config: %w", err)
	}

	client := ses.NewFromConfig(cfg)

	return &Service{
		client:    client,
		fromEmail: appCfg.SESSenderEmail,
	}, nil
}

// SendEmail sends a basic email
func (s *Service) SendEmail(ctx context.Context, params EmailParams) (*SendEmailResult, error) {
	input := &ses.SendEmailInput{
		Source: aws.String(s.fromEmail),
		Destination: &types.Destination{
			ToAddresses: []string{params.To},
		},
		Message: &types.Message{
			Subject: &types.Content{
				Data:    aws.String(params.Subject),
				Charset: aws.String("UTF-8"),
			},
			Body: &types.Body{},
		},
	}

	// Add HTML body if provided
	if params.HTMLBody != "" {
		input.Message.Body.Html = &types.Content{
			Data:    aws.String(params.HTMLBody),
			Charset: aws.String("UTF-8"),
		}
	}

	// Add text body if provided
	if params.TextBody != "" {
		input.Message.Body.Text = &types.Content{
			Data:    aws.String(params.TextBody),
			Charset: aws.String("UTF-8"),
		}
	}

	// Add CC addresses
	if len(params.CC) > 0 {
		input.Destination.CcAddresses = params.CC
	}

	// Add BCC addresses
	if len(params.BCC) > 0 {
		input.Destination.BccAddresses = params.BCC
	}

	// Add reply-to
	if params.ReplyTo != "" {
		input.ReplyToAddresses = []string{params.ReplyTo}
	}

	// Add config set if specified
	if params.ConfigSet != "" {
		input.ConfigurationSetName = aws.String(params.ConfigSet)
	}

	result, err := s.client.SendEmail(ctx, input)
	if err != nil {
		utils.Logger.Error("Failed to send email",
			zap.String("to", params.To),
			zap.String("subject", params.Subject),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to send email: %w", err)
	}

	utils.Logger.Info("Email sent successfully",
		zap.String("to", params.To),
		zap.String("subject", params.Subject),
		zap.String("messageId", *result.MessageId),
	)

	return &SendEmailResult{
		MessageID: *result.MessageId,
		SentAt:    time.Now(),
	}, nil
}

// SendMatchNotification sends a loan match notification email
func (s *Service) SendMatchNotification(ctx context.Context, params MatchNotificationParams) (*SendEmailResult, error) {
	htmlBody, err := s.renderMatchNotificationHTML(params)
	if err != nil {
		return nil, fmt.Errorf("failed to render email template: %w", err)
	}

	textBody := s.renderMatchNotificationText(params)

	subject := fmt.Sprintf("🎉 Great news, %s! You have %d new loan matches", params.UserName, params.MatchCount)

	return s.SendEmail(ctx, EmailParams{
		To:       params.UserEmail,
		Subject:  subject,
		HTMLBody: htmlBody,
		TextBody: textBody,
	})
}

// SendBatchMatchNotifications sends match notifications to multiple users
func (s *Service) SendBatchMatchNotifications(ctx context.Context, notifications []MatchNotificationParams) ([]SendEmailResult, []error) {
	results := make([]SendEmailResult, 0, len(notifications))
	errors := make([]error, 0)

	for _, notif := range notifications {
		result, err := s.SendMatchNotification(ctx, notif)
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to send to %s: %w", notif.UserEmail, err))
			continue
		}
		results = append(results, *result)
	}

	utils.Logger.Info("Batch notifications sent",
		zap.Int("total", len(notifications)),
		zap.Int("success", len(results)),
		zap.Int("failed", len(errors)),
	)

	return results, errors
}

// BuildMatchNotificationParams creates notification params from match data
func BuildMatchNotificationParams(user *models.User, matches []models.Match, products map[int64]*models.LoanProduct, dashboardURL string) MatchNotificationParams {
	topMatches := make([]MatchInfo, 0, len(matches))

	for _, match := range matches {
		product, ok := products[match.ProductID]
		if !ok {
			continue
		}

		topMatches = append(topMatches, MatchInfo{
			ProductName:      product.ProductName,
			Provider:         product.ProviderName,
			InterestRateMin:  product.InterestRateMin,
			InterestRateMax:  product.InterestRateMax,
			MaxLoanAmount:    product.LoanAmountMax,
			EligibilityScore: match.MatchScore,
		})
	}

	return MatchNotificationParams{
		UserName:     user.UserID, // Using UserID as name since User model doesn't have Name field
		UserEmail:    user.Email,
		MatchCount:   len(matches),
		TopMatches:   topMatches,
		DashboardURL: dashboardURL,
	}
}

// renderMatchNotificationHTML renders the HTML email template
func (s *Service) renderMatchNotificationHTML(params MatchNotificationParams) (string, error) {
	tmpl := `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <style>
        body { font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; line-height: 1.6; color: #333; max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; padding: 30px; border-radius: 10px 10px 0 0; text-align: center; }
        .header h1 { margin: 0; font-size: 24px; }
        .content { background: #f9f9f9; padding: 30px; border-radius: 0 0 10px 10px; }
        .match-card { background: white; border-radius: 8px; padding: 20px; margin: 15px 0; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .match-card h3 { margin: 0 0 10px 0; color: #667eea; }
        .match-card .provider { color: #666; font-size: 14px; margin-bottom: 10px; }
        .match-card .details { display: flex; justify-content: space-between; flex-wrap: wrap; }
        .match-card .detail-item { margin: 5px 0; }
        .match-card .detail-label { font-size: 12px; color: #999; }
        .match-card .detail-value { font-weight: bold; color: #333; }
        .score-badge { display: inline-block; background: #28a745; color: white; padding: 5px 12px; border-radius: 20px; font-weight: bold; }
        .cta-button { display: inline-block; background: #667eea; color: white; padding: 15px 30px; text-decoration: none; border-radius: 8px; font-weight: bold; margin-top: 20px; }
        .cta-button:hover { background: #5a6fd6; }
        .footer { text-align: center; margin-top: 30px; color: #999; font-size: 12px; }
    </style>
</head>
<body>
    <div class="header">
        <h1>🎉 New Loan Matches Found!</h1>
        <p>Hi {{.UserName}}, we found {{.MatchCount}} loan products for you</p>
    </div>
    <div class="content">
        <p>Based on your profile, we've identified the following loan products that match your eligibility:</p>
        
        {{range .TopMatches}}
        <div class="match-card">
            <h3>{{.ProductName}}</h3>
            <p class="provider">by {{.Provider}}</p>
            <div class="details">
                <div class="detail-item">
                    <div class="detail-label">Interest Rate</div>
                    <div class="detail-value">{{printf "%.2f" .InterestRateMin}}% - {{printf "%.2f" .InterestRateMax}}%</div>
                </div>
                <div class="detail-item">
                    <div class="detail-label">Max Amount</div>
                    <div class="detail-value">₹{{printf "%.0f" .MaxLoanAmount}}</div>
                </div>
                <div class="detail-item">
                    <div class="detail-label">Eligibility Score</div>
                    <div class="detail-value"><span class="score-badge">{{printf "%.0f" .EligibilityScore}}%</span></div>
                </div>
            </div>
        </div>
        {{end}}
        
        {{if .DashboardURL}}
        <div style="text-align: center;">
            <a href="{{.DashboardURL}}" class="cta-button">View All Matches</a>
        </div>
        {{end}}
    </div>
    <div class="footer">
        <p>This email was sent by Loan Eligibility Engine</p>
        <p>You received this because you uploaded your profile for loan matching.</p>
    </div>
</body>
</html>`

	t, err := template.New("match_notification").Parse(tmpl)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, params); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// renderMatchNotificationText renders plain text version
func (s *Service) renderMatchNotificationText(params MatchNotificationParams) string {
	var buf bytes.Buffer

	buf.WriteString(fmt.Sprintf("Hi %s,\n\n", params.UserName))
	buf.WriteString(fmt.Sprintf("Great news! We found %d loan products that match your eligibility.\n\n", params.MatchCount))
	buf.WriteString("Here are your top matches:\n\n")

	for i, match := range params.TopMatches {
		buf.WriteString(fmt.Sprintf("%d. %s by %s\n", i+1, match.ProductName, match.Provider))
		buf.WriteString(fmt.Sprintf("   Interest Rate: %.2f%% - %.2f%%\n", match.InterestRateMin, match.InterestRateMax))
		buf.WriteString(fmt.Sprintf("   Max Amount: ₹%.0f\n", match.MaxLoanAmount))
		buf.WriteString(fmt.Sprintf("   Eligibility Score: %.0f%%\n\n", match.EligibilityScore))
	}

	if params.DashboardURL != "" {
		buf.WriteString(fmt.Sprintf("View all matches: %s\n\n", params.DashboardURL))
	}

	buf.WriteString("Best regards,\nLoan Eligibility Engine Team\n")

	return buf.String()
}

// VerifyEmailAddress verifies an email address for sending
func (s *Service) VerifyEmailAddress(ctx context.Context, email string) error {
	input := &ses.VerifyEmailAddressInput{
		EmailAddress: aws.String(email),
	}

	_, err := s.client.VerifyEmailAddress(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to verify email: %w", err)
	}

	utils.Logger.Info("Email verification initiated", zap.String("email", email))
	return nil
}

// GetSendQuota returns the current SES sending quota
func (s *Service) GetSendQuota(ctx context.Context) (*ses.GetSendQuotaOutput, error) {
	result, err := s.client.GetSendQuota(ctx, &ses.GetSendQuotaInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to get send quota: %w", err)
	}
	return result, nil
}
