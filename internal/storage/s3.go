package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/lazrossi/code-agent/internal/config"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// TraceEvent matches the agent trace format for deserialization.
type TraceEvent struct {
	Phase  string    `json:"phase"`
	At     time.Time `json:"at"`
	Detail string    `json:"detail,omitempty"`
}

// Client wraps a MinIO client for artifact storage.
type Client struct {
	mc     *minio.Client
	bucket string
}

// New creates a MinIO client and ensures the bucket exists.
func New(cfg config.S3Config) (*Client, error) {
	mc, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("minio client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	exists, err := mc.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("check bucket: %w", err)
	}
	if !exists {
		if err := mc.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, fmt.Errorf("create bucket: %w", err)
		}
	}

	return &Client{mc: mc, bucket: cfg.Bucket}, nil
}

// PutRunLog uploads agent output for a run.
func (c *Client) PutRunLog(ctx context.Context, runID int64, data []byte) error {
	key := fmt.Sprintf("runs/%d/output.log", runID)
	r := bytes.NewReader(data)
	_, err := c.mc.PutObject(ctx, c.bucket, key, r, int64(len(data)), minio.PutObjectOptions{
		ContentType: "text/plain",
	})
	return err
}

// PutRunTrace uploads structured trace events for a run.
func (c *Client) PutRunTrace(ctx context.Context, runID int64, data []byte) error {
	key := fmt.Sprintf("runs/%d/trace.json", runID)
	r := bytes.NewReader(data)
	_, err := c.mc.PutObject(ctx, c.bucket, key, r, int64(len(data)), minio.PutObjectOptions{
		ContentType: "application/json",
	})
	return err
}

// GetRunLog retrieves agent output for a run.
func (c *Client) GetRunLog(ctx context.Context, runID int64) ([]byte, error) {
	key := fmt.Sprintf("runs/%d/output.log", runID)
	obj, err := c.mc.GetObject(ctx, c.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer obj.Close()
	return io.ReadAll(obj)
}

// GetRunTrace retrieves structured trace events for a run.
func (c *Client) GetRunTrace(ctx context.Context, runID int64) ([]TraceEvent, error) {
	key := fmt.Sprintf("runs/%d/trace.json", runID)
	obj, err := c.mc.GetObject(ctx, c.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer obj.Close()

	data, err := io.ReadAll(obj)
	if err != nil {
		return nil, err
	}

	var events []TraceEvent
	if err := json.Unmarshal(data, &events); err != nil {
		return nil, fmt.Errorf("parse trace: %w", err)
	}
	return events, nil
}
