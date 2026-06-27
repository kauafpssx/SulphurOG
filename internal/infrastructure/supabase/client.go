package supabase

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/sulphurog/sulphurog/internal/domain"
)

type Client struct {
	url    string
	key    string
	client *http.Client
}

func NewClient(url, key string) *Client {
	return &Client{
		url:    strings.TrimSuffix(url, "/"),
		key:    key,
		client: &http.Client{},
	}
}

func (c *Client) Upload(ctx context.Context, bucket string, path string, data []byte) error {
	url := fmt.Sprintf("%s/storage/v1/object/%s/%s", c.url, bucket, path)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("apikey", c.key)
	req.Header.Set("Authorization", "Bearer "+c.key)
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("x-upsert", "true")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("upload failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		bodyLower := strings.ToLower(string(body))
		if resp.StatusCode == 413 || strings.Contains(bodyLower, "exceeded") ||
			strings.Contains(bodyLower, "storageagelimitexceeded") ||
			strings.Contains(bodyLower, "bucket size limit") ||
			strings.Contains(bodyLower, "storage limit") {
			return domain.ErrStorageFull
		}
		return fmt.Errorf("upload error %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (c *Client) UploadReader(ctx context.Context, bucket string, path string, reader io.Reader, size int64) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}
	return c.Upload(ctx, bucket, path, data)
}

func (c *Client) UploadFile(ctx context.Context, localPath, remotePath, bucket string) error {
	data, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}
	return c.Upload(ctx, bucket, remotePath, data)
}
