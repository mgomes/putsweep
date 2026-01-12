package putio

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	lib "github.com/putdotio/go-putio"
)

type Client struct {
	client *lib.Client
}

func NewClient(c *lib.Client) *Client {
	return &Client{client: c}
}

type FileInfo struct {
	ID   int64
	Name string
	Size uint64
}

func (c *Client) GetFileInfo(ctx context.Context, fileID int64) (*FileInfo, error) {
	file, err := c.client.Files.Get(ctx, fileID)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	if file.IsDir() {
		return nil, fmt.Errorf("file ID %d is a directory, not a file", fileID)
	}

	return &FileInfo{
		ID:   file.ID,
		Name: file.Name,
		Size: uint64(file.Size),
	}, nil
}

func (c *Client) GetDownloadURL(ctx context.Context, fileID int64) (string, error) {
	downloadURL, err := c.client.Files.URL(ctx, fileID, false)
	if err != nil {
		return "", fmt.Errorf("failed to get download URL: %w", err)
	}
	return downloadURL, nil
}

func ParsePutioURL(urlStr string) (int64, error) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return 0, fmt.Errorf("invalid URL: %w", err)
	}

	if !strings.HasSuffix(u.Host, "put.io") {
		return 0, fmt.Errorf("not a put.io URL: %s", u.Host)
	}

	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 || parts[0] != "files" {
		return 0, fmt.Errorf("invalid put.io file URL format")
	}

	fileID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid file ID: %w", err)
	}

	return fileID, nil
}
