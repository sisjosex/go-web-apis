package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// region is fixed: R2 takes "auto", SeaweedFS ignores it. Setting it is what keeps presigning local —
// without one, minio-go asks the store for the bucket's location before signing.
const region = "auto"

// Client is the process's one connection pool to the store (keep-alive to R2); every bucket shares it.
type Client struct {
	core *minio.Core
}

// NewClient connects to an S3 endpoint given as a URL ("https://<account>.r2.cloudflarestorage.com",
// "http://127.0.0.1:8333"). Nothing is dialled until the first call.
func NewClient(endpoint, accessKey, secretKey string) (*Client, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Host == "" {
		return nil, fmt.Errorf("storage: endpoint %q is not a URL", endpoint)
	}
	core, err := minio.NewCore(parsed.Host, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: parsed.Scheme == "https",
		Region: region,
	})
	if err != nil {
		return nil, fmt.Errorf("storage: %w", err)
	}
	return &Client{core: core}, nil
}

// Bucket is the store for one bucket.
func (c *Client) Bucket(name string) ObjectStore {
	return &s3Store{core: c.core, bucket: name}
}

// EmptyTestBucket deletes every object in a test bucket, so `make db-reset` leaves the bucket as
// empty as the database it resets. Only a bucket named *-test is ever emptied: the guard is here, at
// the one place that deletes in bulk, not in the caller.
func (c *Client) EmptyTestBucket(ctx context.Context, bucket string) error {
	if !strings.HasSuffix(bucket, "-test") {
		return fmt.Errorf("storage: refusing to empty %q, not a -test bucket", bucket)
	}
	objects := c.core.Client.ListObjects(ctx, bucket, minio.ListObjectsOptions{Recursive: true})
	for result := range c.core.RemoveObjects(ctx, bucket, objects, minio.RemoveObjectsOptions{}) {
		if result.Err != nil {
			return fmt.Errorf("storage: empty %s: %w", bucket, result.Err)
		}
	}
	return nil
}

type s3Store struct {
	core   *minio.Core
	bucket string
}

func (s *s3Store) Put(ctx context.Context, key string, body io.Reader, size int64, opts PutOptions) error {
	_, err := s.core.Client.PutObject(ctx, s.bucket, key, body, size, minio.PutObjectOptions{
		ContentType:  opts.ContentType,
		CacheControl: opts.CacheControl,
	})
	return mapErr(err)
}

func (s *s3Store) GetRange(ctx context.Context, key string, offset, length int64) ([]byte, int64, error) {
	opts := minio.GetObjectOptions{}
	if err := opts.SetRange(offset, offset+length-1); err != nil {
		return nil, 0, err
	}
	body, info, header, err := s.core.GetObject(ctx, s.bucket, key, opts)
	if err != nil {
		return nil, 0, mapErr(err)
	}
	defer func() { _ = body.Close() }()

	data, err := io.ReadAll(io.LimitReader(body, length))
	if err != nil {
		return nil, 0, err
	}
	// "bytes 0-511/48213": the total follows the slash. A store that ignores the range answers 200
	// with the whole object, and then its length is the total.
	total := info.Size
	if _, after, ok := strings.Cut(header.Get("Content-Range"), "/"); ok {
		if parsed, err := strconv.ParseInt(after, 10, 64); err == nil {
			total = parsed
		}
	}
	return data, total, nil
}

func (s *s3Store) Copy(ctx context.Context, src, dst string) error {
	_, err := s.core.Client.CopyObject(ctx,
		minio.CopyDestOptions{Bucket: s.bucket, Object: dst},
		minio.CopySrcOptions{Bucket: s.bucket, Object: src})
	return mapErr(err)
}

func (s *s3Store) Delete(ctx context.Context, key string) error {
	return mapErr(s.core.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{}))
}

func (s *s3Store) PresignPut(ctx context.Context, key, contentType string, size int64, ttl time.Duration) (string, http.Header, error) {
	headers := http.Header{}
	headers.Set("Content-Type", contentType)
	headers.Set("Content-Length", strconv.FormatInt(size, 10))
	signed, err := s.core.PresignHeader(ctx, http.MethodPut, s.bucket, key, ttl, url.Values{}, headers)
	if err != nil {
		return "", nil, err
	}
	// The browser sets Content-Length itself and refuses to be told it; Content-Type is the one the
	// client has to send.
	return signed.String(), http.Header{"Content-Type": {contentType}}, nil
}

func (s *s3Store) PresignGet(ctx context.Context, key, filename, contentType string, ttl time.Duration) (string, error) {
	params := url.Values{}
	params.Set("response-content-disposition", `attachment; filename="`+filename+`"`)
	params.Set("response-content-type", contentType)
	signed, err := s.core.PresignedGetObject(ctx, s.bucket, key, ttl, params)
	if err != nil {
		return "", err
	}
	return signed.String(), nil
}

func mapErr(err error) error {
	if err == nil {
		return nil
	}
	if code := minio.ToErrorResponse(err).Code; code == "NoSuchKey" || code == "NotFound" {
		return fmt.Errorf("%w: %v", ErrNotFound, err)
	}
	var resp minio.ErrorResponse
	if errors.As(err, &resp) && resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("%w: %v", ErrNotFound, err)
	}
	return err
}
