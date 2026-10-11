// Package storage talks to the family's private object-storage bucket:
// LocalStack locally, Cloudflare R2 or Backblaze B2 in production, all
// reached through the same S3-compatible API (SPEC 6, "Photos and
// attachments").
//
// Large files never pass through Go. core-api only ever mints presigned
// PUT/GET URLs and, once a client reports an upload done, HEADs the real
// object to verify what it was actually given rather than trusting the
// client's say-so.
package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"

	"github.com/olehmushka/mykomora/core-api/internal/config"
)

// presignExpiry bounds both PresignPut and PresignGet: short enough that a
// leaked URL (a browser history entry, a proxy log) stops working quickly,
// long enough for a slow mobile upload to finish.
const presignExpiry = 10 * time.Minute

// ErrObjectNotFound is returned by HeadObject when the key does not exist —
// either it was never uploaded, or something else already removed it.
var ErrObjectNotFound = errors.New("object not found")

// Object is one key the bucket holds, as reported by ListObjects.
type Object struct {
	Key          string
	LastModified time.Time
}

// Client is the narrow slice of S3 operations this milestone needs.
type Client interface {
	// PresignPut mints a URL + the headers the PUT request must carry for
	// the signature to validate. The caller never makes this request
	// itself — it hands the URL to the browser.
	PresignPut(ctx context.Context, key, contentType string, contentLength int64) (url string, headers map[string]string, expiresAt time.Time, err error)
	// PresignGet mints a short-lived URL to read a private object.
	PresignGet(ctx context.Context, key string) (url string, err error)
	// HeadObject reports what was actually uploaded, independent of
	// whatever a client claims it uploaded.
	HeadObject(ctx context.Context, key string) (contentType string, size int64, err error)
	DeleteObject(ctx context.Context, key string) error
	// ListObjects returns every key the bucket holds. Used only by the
	// cleanup sweeper (cleanup.go); nothing else needs the whole bucket.
	ListObjects(ctx context.Context) ([]Object, error)
}

// awsClient is split across two *s3.Client values because presigned URLs
// and core-api's own requests are not reached the same way. Presigning
// never makes a network call — it only has to build a URL the browser can
// resolve, which is config.S3PublicEndpoint (http://localhost:4566
// locally; S3Endpoint's Compose-internal "s3" hostname means nothing to a
// browser). HeadObject/DeleteObject/ListObjects are real calls, made
// against S3Endpoint from inside the Compose network.
type awsClient struct {
	bucket    string
	ops       *s3.Client
	presigner *s3.PresignClient
}

// NewClient builds the S3-compatible client from config.
func NewClient(cfg config.Config) Client {
	creds := credentials.NewStaticCredentialsProvider(cfg.S3AccessKeyID, cfg.S3SecretAccessKey, "")

	ops := s3.New(s3.Options{
		Region:       cfg.S3Region,
		Credentials:  creds,
		BaseEndpoint: aws.String(cfg.S3Endpoint),
		UsePathStyle: true,
	})

	public := s3.New(s3.Options{
		Region:       cfg.S3Region,
		Credentials:  creds,
		BaseEndpoint: aws.String(cfg.S3PublicEndpoint),
		UsePathStyle: true,
	})

	return &awsClient{bucket: cfg.S3Bucket, ops: ops, presigner: s3.NewPresignClient(public)}
}

func (c *awsClient) PresignPut(
	ctx context.Context, key, contentType string, contentLength int64,
) (url string, headers map[string]string, expiresAt time.Time, err error) {
	req, err := c.presigner.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(c.bucket),
		Key:           aws.String(key),
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(contentLength),
	}, s3.WithPresignExpires(presignExpiry))
	if err != nil {
		return "", nil, time.Time{}, fmt.Errorf("presign put: %w", err)
	}

	signed := make(map[string]string, len(req.SignedHeader))
	for name, values := range req.SignedHeader {
		if len(values) > 0 {
			signed[name] = values[0]
		}
	}

	return req.URL, signed, time.Now().Add(presignExpiry), nil
}

func (c *awsClient) PresignGet(ctx context.Context, key string) (string, error) {
	req, err := c.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(presignExpiry))
	if err != nil {
		return "", fmt.Errorf("presign get: %w", err)
	}

	return req.URL, nil
}

func (c *awsClient) HeadObject(ctx context.Context, key string) (contentType string, size int64, err error) {
	out, err := c.ops.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(c.bucket), Key: aws.String(key)})
	if err != nil {
		if isNotFound(err) {
			return "", 0, ErrObjectNotFound
		}

		return "", 0, fmt.Errorf("head object: %w", err)
	}

	if out.ContentType != nil {
		contentType = *out.ContentType
	}

	if out.ContentLength != nil {
		size = *out.ContentLength
	}

	return contentType, size, nil
}

func (c *awsClient) DeleteObject(ctx context.Context, key string) error {
	if _, err := c.ops.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(c.bucket), Key: aws.String(key)}); err != nil {
		return fmt.Errorf("delete object: %w", err)
	}

	return nil
}

func (c *awsClient) ListObjects(ctx context.Context) ([]Object, error) {
	out := []Object{}

	paginator := s3.NewListObjectsV2Paginator(c.ops, &s3.ListObjectsV2Input{Bucket: aws.String(c.bucket)})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("list objects: %w", err)
		}

		for _, obj := range page.Contents {
			if obj.Key == nil {
				continue
			}

			var lastModified time.Time
			if obj.LastModified != nil {
				lastModified = *obj.LastModified
			}

			out = append(out, Object{Key: *obj.Key, LastModified: lastModified})
		}
	}

	return out, nil
}

// isNotFound reports whether err is S3's way of saying the key doesn't
// exist — either the typed NotFound error, or (what LocalStack and some
// S3-compatible providers return for a bare HEAD) a generic API error
// carrying a 404 status.
func isNotFound(err error) bool {
	var notFound *types.NotFound
	if errors.As(err, &notFound) {
		return true
	}

	var apiErr smithy.APIError

	return errors.As(err, &apiErr) && apiErr.ErrorCode() == "NotFound"
}
