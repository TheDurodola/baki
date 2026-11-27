package gateway

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/TheDurodola/baki/internal/config"
)

type PaystackGateway struct {
	cfg        *config.Config
	httpClient *http.Client
}

func NewPaystackGateway(cfg *config.Config) *PaystackGateway {
	return &PaystackGateway{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type InitializeTransactionRequest struct {
	Email       string   `json:"email"`
	Amount      int64    `json:"amount"` // in minor units (kobo)
	Reference   string   `json:"reference"`
	CallbackURL string   `json:"callback_url,omitempty"`
	Channels    []string `json:"channels,omitempty"`
}

type InitializeTransactionResponse struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
	Data    struct {
		AuthorizationURL string `json:"authorization_url"`
		AccessCode       string `json:"access_code"`
		Reference        string `json:"reference"`
	} `json:"data"`
}

type VerifyTransactionResponse struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
	Data    struct {
		Status    string    `json:"status"` // "success", "failed", "abandoned"
		Reference string    `json:"reference"`
		Amount    int64     `json:"amount"`
		Currency  string    `json:"currency"`
		PaidAt    time.Time `json:"paid_at"`
		Customer  struct {
			Email string `json:"email"`
		} `json:"customer"`
	} `json:"data"`
}

func (g *PaystackGateway) Initialize(ctx context.Context, req InitializeTransactionRequest) (*InitializeTransactionResponse, error) {
	if g.cfg.PaystackSecretKey == "" {
		// Mock response for local development if keys are not configured
		return &InitializeTransactionResponse{
			Status:  true,
			Message: "Mock checkout URL (Test Mode)",
			Data: struct {
				AuthorizationURL string `json:"authorization_url"`
				AccessCode       string `json:"access_code"`
				Reference        string `json:"reference"`
			}{
				AuthorizationURL: fmt.Sprintf("https://checkout.paystack.com/%s", req.Reference),
				AccessCode:       "mock_access_code",
				Reference:        req.Reference,
			},
		}, nil
	}

	url := "https://api.paystack.co/transaction/initialize"
	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create http request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+g.cfg.PaystackSecretKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := g.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("paystack initialize request: %w", err)
	}
	defer resp.Body.Close()

	var result InitializeTransactionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode paystack response: %w", err)
	}

	if !result.Status {
		return nil, fmt.Errorf("paystack error: %s", result.Message)
	}

	return &result, nil
}

func (g *PaystackGateway) Verify(ctx context.Context, reference string) (*VerifyTransactionResponse, error) {
	if g.cfg.PaystackSecretKey == "" {
		// Return mock success for local development
		return &VerifyTransactionResponse{
			Status:  true,
			Message: "Verification successful (Mock)",
			Data: struct {
				Status    string    `json:"status"`
				Reference string    `json:"reference"`
				Amount    int64     `json:"amount"`
				Currency  string    `json:"currency"`
				PaidAt    time.Time `json:"paid_at"`
				Customer  struct {
					Email string `json:"email"`
				} `json:"customer"`
			}{
				Status:    "success",
				Reference: reference,
				PaidAt:    time.Now().UTC(),
			},
		}, nil
	}

	url := fmt.Sprintf("https://api.paystack.co/transaction/verify/%s", reference)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create http request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+g.cfg.PaystackSecretKey)

	resp, err := g.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("paystack verify request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	var result VerifyTransactionResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode paystack response: %w", err)
	}

	if !result.Status {
		return nil, fmt.Errorf("paystack verify error: %s", result.Message)
	}

	return &result, nil
}

// VerifyWebhookSignature verifies that the webhook request came from Paystack using HMAC-SHA512
func (g *PaystackGateway) VerifyWebhookSignature(payload []byte, signatureHeader string) bool {
	if g.cfg.PaystackWebhookKey == "" {
		return true // If no webhook secret configured in dev, accept
	}

	mac := hmac.New(sha512.New, []byte(g.cfg.PaystackWebhookKey))
	mac.Write(payload)
	expectedSignature := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expectedSignature), []byte(signatureHeader))
}
