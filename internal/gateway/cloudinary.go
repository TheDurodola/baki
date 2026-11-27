package gateway

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/bolaji/baki/internal/config"
	"github.com/google/uuid"
)

type CloudinaryGateway struct {
	cfg        *config.Config
	httpClient *http.Client
}

func NewCloudinaryGateway(cfg *config.Config) *CloudinaryGateway {
	return &CloudinaryGateway{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type cloudinaryUploadResponse struct {
	PublicID  string `json:"public_id"`
	SecureURL string `json:"secure_url"`
	Error     *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// UploadImage uploads an image reader to Cloudinary and returns the secure URL
func (g *CloudinaryGateway) UploadImage(ctx context.Context, fileReader io.Reader, filename string) (string, error) {
	// Fallback mock for development environment when Cloudinary keys are not provided
	if g.cfg.CloudinaryCloudName == "" || g.cfg.CloudinaryAPIKey == "" || g.cfg.CloudinaryAPISecret == "" {
		mockURL := fmt.Sprintf("https://res.cloudinary.com/demo/image/upload/%s/%s_%s",
			g.cfg.CloudinaryUploadFolder, uuid.New().String(), filename)
		return mockURL, nil
	}

	timestamp := time.Now().Unix()
	folder := g.cfg.CloudinaryUploadFolder

	// Cloudinary signature: SHA-1 of sorted parameters + api_secret
	// e.g. folder=baki/profiles&timestamp=1234567890<api_secret>
	signaturePayload := fmt.Sprintf("folder=%s&timestamp=%d%s", folder, timestamp, g.cfg.CloudinaryAPISecret)
	hasher := sha1.New()
	hasher.Write([]byte(signaturePayload))
	signature := hex.EncodeToString(hasher.Sum(nil))

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add file field
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return "", fmt.Errorf("create form file: %w", err)
	}
	if _, err := io.Copy(part, fileReader); err != nil {
		return "", fmt.Errorf("copy file bytes: %w", err)
	}

	// Add required parameters
	_ = writer.WriteField("api_key", g.cfg.CloudinaryAPIKey)
	_ = writer.WriteField("timestamp", fmt.Sprintf("%d", timestamp))
	_ = writer.WriteField("signature", signature)
	_ = writer.WriteField("folder", folder)

	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("close multipart writer: %w", err)
	}

	apiURL := fmt.Sprintf("https://api.cloudinary.com/v1_1/%s/image/upload", g.cfg.CloudinaryCloudName)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, body)
	if err != nil {
		return "", fmt.Errorf("create upload request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("cloudinary upload request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read cloudinary response: %w", err)
	}

	var res cloudinaryUploadResponse
	if err := json.Unmarshal(respBody, &res); err != nil {
		return "", fmt.Errorf("decode cloudinary response: %w", err)
	}

	if res.Error != nil && res.Error.Message != "" {
		return "", fmt.Errorf("cloudinary error: %s", res.Error.Message)
	}

	if res.SecureURL == "" {
		return "", fmt.Errorf("cloudinary did not return a secure_url")
	}

	return res.SecureURL, nil
}
