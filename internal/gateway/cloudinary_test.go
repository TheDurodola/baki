package gateway_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/TheDurodola/baki/internal/config"
	"github.com/TheDurodola/baki/internal/gateway"
)

func TestCloudinaryGateway_UploadImage_MockMode(t *testing.T) {
	cfg := &config.Config{
		CloudinaryCloudName:    "", // Empty triggers dev/mock mode
		CloudinaryAPIKey:       "",
		CloudinaryAPISecret:    "",
		CloudinaryUploadFolder: "baki/profiles",
	}

	gw := gateway.NewCloudinaryGateway(cfg)
	dummyImageContent := []byte("fake-image-bytes-jpeg")
	reader := bytes.NewReader(dummyImageContent)

	url, err := gw.UploadImage(context.Background(), reader, "profile.jpg")
	if err != nil {
		t.Fatalf("unexpected error in mock upload: %v", err)
	}

	if !strings.HasPrefix(url, "https://res.cloudinary.com/demo/image/upload/baki/profiles/") {
		t.Errorf("expected URL to start with demo cloudinary upload folder, got: %s", url)
	}

	if !strings.HasSuffix(url, "profile.jpg") {
		t.Errorf("expected URL to end with filename, got: %s", url)
	}
}
