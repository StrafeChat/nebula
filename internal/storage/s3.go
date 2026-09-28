package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

// S3Config is everything needed to reach one bucket on any S3-compatible service.
type S3Config struct {
	Bucket string
	Region string
	// Endpoint overrides the AWS endpoint for compatible services (MinIO, R2, B2, Ceph).
	// Empty means real AWS S3.
	Endpoint string
	// Prefix is prepended to every key, so one bucket can be shared between instances or
	// with other data ("strafe/").
	Prefix string
	// AccessKeyID/SecretAccessKey are static credentials. Both empty means the SDK's usual
	// chain: environment, shared config, instance/role credentials.
	AccessKeyID     string
	SecretAccessKey string
	// ForcePathStyle addresses the bucket as host/bucket/key instead of bucket.host/key.
	// Required by MinIO and most self-hosted services; harmful on AWS.
	ForcePathStyle bool
}

// S3 stores objects in a bucket.
type S3 struct {
	client   *s3.Client
	uploader *transfermanager.Client
	bucket   string
	prefix   string
}

// NewS3 connects and verifies the bucket is reachable, so a typo in the configuration is
// a startup error with the real cause rather than a 500 on the first upload.
func NewS3(ctx context.Context, cfg S3Config) (*S3, error) {
	if cfg.Bucket == "" {
		return nil, errors.New("s3: bucket is required")
	}
	var opts []func(*awsconfig.LoadOptions) error
	if cfg.Region != "" {
		opts = append(opts, awsconfig.WithRegion(cfg.Region))
	}
	if cfg.AccessKeyID != "" || cfg.SecretAccessKey != "" {
		opts = append(opts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		))
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("s3: load config: %w", err)
	}
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
		o.UsePathStyle = cfg.ForcePathStyle
		// Uploads are streamed straight from the request body, which cannot be rewound. The
		// SDK's default of checksumming every upload needs either a seekable body or a
		// trailing checksum over TLS - so a plain-http MinIO on a LAN would refuse every
		// PUT. Checksums stay on where the service requires them.
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		o.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
	})

	prefix := strings.Trim(cfg.Prefix, "/")
	if prefix != "" {
		prefix += "/"
	}
	st := &S3{client: client, uploader: transfermanager.New(client), bucket: cfg.Bucket, prefix: prefix}

	probe, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if _, err := client.HeadBucket(probe, &s3.HeadBucketInput{Bucket: aws.String(cfg.Bucket)}); err != nil {
		return nil, fmt.Errorf("s3: bucket %q not reachable: %w", cfg.Bucket, err)
	}
	return st, nil
}

func (s *S3) Name() string { return "s3" }

func (s *S3) objectKey(key string) (string, error) {
	key, err := CleanKey(key)
	if err != nil {
		return "", err
	}
	return s.prefix + key, nil
}

func (s *S3) Stat(ctx context.Context, key string) (ObjectInfo, error) {
	k, err := s.objectKey(key)
	if err != nil {
		return ObjectInfo{}, err
	}
	out, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(k)})
	if err != nil {
		return ObjectInfo{}, mapS3Error(err)
	}
	return ObjectInfo{
		Size:        aws.ToInt64(out.ContentLength),
		ModTime:     aws.ToTime(out.LastModified),
		ContentType: aws.ToString(out.ContentType),
		ETag:        aws.ToString(out.ETag),
	}, nil
}

func (s *S3) Open(ctx context.Context, key string, rng *ByteRange) (io.ReadCloser, ObjectInfo, error) {
	k, err := s.objectKey(key)
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	in := &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(k)}
	if rng != nil {
		in.Range = aws.String(fmt.Sprintf("bytes=%d-%d", rng.Start, rng.End))
	}
	out, err := s.client.GetObject(ctx, in)
	if err != nil {
		return nil, ObjectInfo{}, mapS3Error(err)
	}
	info := ObjectInfo{
		Size:        aws.ToInt64(out.ContentLength),
		ModTime:     aws.ToTime(out.LastModified),
		ContentType: aws.ToString(out.ContentType),
		ETag:        aws.ToString(out.ETag),
	}
	// For a ranged read ContentLength is the range's length; the caller wants the whole
	// object's size to build Content-Range, which S3 reports in that same header.
	if rng != nil {
		if total, ok := totalFromContentRange(aws.ToString(out.ContentRange)); ok {
			info.Size = total
		}
	}
	return out.Body, info, nil
}

// totalFromContentRange parses "bytes 0-99/1234" -> 1234.
func totalFromContentRange(h string) (int64, bool) {
	i := strings.LastIndexByte(h, '/')
	if i < 0 {
		return 0, false
	}
	var total int64
	if _, err := fmt.Sscanf(h[i+1:], "%d", &total); err != nil {
		return 0, false
	}
	return total, true
}

func (s *S3) Put(ctx context.Context, key string, body io.Reader, _ int64, contentType string) (int64, error) {
	k, err := s.objectKey(key)
	if err != nil {
		return 0, err
	}
	in := &transfermanager.UploadObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(k)}
	if contentType != "" {
		in.ContentType = aws.String(contentType)
	}
	// The body is the HTTP request stream, which cannot be rewound - and a plain PutObject
	// needs to hash the whole payload for the request signature before sending it, which
	// means seeking back to the start. The transfer manager reads the stream in parts and
	// signs each one as it goes, so it works on any reader and, as a bonus, uses multipart
	// upload for anything bigger than one part.
	counter := &countingReader{r: body}
	in.Body = counter
	if _, err := s.uploader.UploadObject(ctx, in); err != nil {
		return 0, fmt.Errorf("s3: put %q: %w", k, err)
	}
	return counter.n, nil
}

type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

func (s *S3) Delete(ctx context.Context, key string) error {
	k, err := s.objectKey(key)
	if err != nil {
		return err
	}
	// S3's DeleteObject is idempotent and says nothing about whether the key existed; the
	// HTTP layer wants a 404 for a missing object, so check first.
	if _, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(k)}); err != nil {
		return mapS3Error(err)
	}
	if _, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(k)}); err != nil {
		return fmt.Errorf("s3: delete %q: %w", k, err)
	}
	return nil
}

// mapS3Error turns the SDK's "no such key" family into ErrNotFound and leaves the rest.
func mapS3Error(err error) error {
	var nsk *types.NoSuchKey
	if errors.As(err, &nsk) {
		return ErrNotFound
	}
	var nf *types.NotFound
	if errors.As(err, &nf) {
		return ErrNotFound
	}
	var api smithy.APIError
	if errors.As(err, &api) {
		switch api.ErrorCode() {
		case "NoSuchKey", "NotFound", "404":
			return ErrNotFound
		}
	}
	return err
}
