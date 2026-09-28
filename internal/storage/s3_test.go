package storage

import (
	"bytes"
	"context"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
)

// newTestS3 runs an in-process S3 implementation and returns a backend pointed at it. It
// exercises the real SDK request path (signing, path-style addressing, range headers)
// against a server that speaks the protocol, which is what a real bucket will do.
func newTestS3(t *testing.T, prefix string) *S3 {
	t.Helper()
	fake := gofakes3.New(s3mem.New())
	srv := httptest.NewServer(fake.Server())
	t.Cleanup(srv.Close)

	st, err := NewS3(context.Background(), S3Config{
		Bucket:          "nebula-test",
		Region:          "us-east-1",
		Endpoint:        srv.URL,
		Prefix:          prefix,
		AccessKeyID:     "test",
		SecretAccessKey: "test",
		ForcePathStyle:  true,
	})
	if err == nil {
		return st
	}
	// The bucket does not exist yet: create it with the same client settings and retry,
	// which also proves NewS3's reachability check reports the missing bucket clearly.
	if !strings.Contains(err.Error(), "not reachable") {
		t.Fatalf("unexpected NewS3 error: %v", err)
	}
	client := s3.New(s3.Options{
		BaseEndpoint: aws.String(srv.URL),
		Region:       "us-east-1",
		UsePathStyle: true,
		Credentials: aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
			return aws.Credentials{AccessKeyID: "test", SecretAccessKey: "test"}, nil
		}),
	})
	if _, err := client.CreateBucket(context.Background(), &s3.CreateBucketInput{Bucket: aws.String("nebula-test")}); err != nil {
		t.Fatalf("create bucket: %v", err)
	}
	st, err = NewS3(context.Background(), S3Config{
		Bucket: "nebula-test", Region: "us-east-1", Endpoint: srv.URL, Prefix: prefix,
		AccessKeyID: "test", SecretAccessKey: "test", ForcePathStyle: true,
	})
	if err != nil {
		t.Fatalf("NewS3 after bucket creation: %v", err)
	}
	return st
}

func TestS3RoundTrip(t *testing.T) {
	st := newTestS3(t, "strafe")
	ctx := context.Background()
	body := []byte("hello, bucket - this is an attachment body")

	n, err := st.Put(ctx, "attachments/1/hello.txt", bytes.NewReader(body), int64(len(body)), "text/plain")
	if err != nil {
		t.Fatalf("put: %v", err)
	}
	if n != int64(len(body)) {
		t.Fatalf("put reported %d bytes, want %d", n, len(body))
	}

	info, err := st.Stat(ctx, "attachments/1/hello.txt")
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Size != int64(len(body)) {
		t.Fatalf("stat size %d, want %d", info.Size, len(body))
	}

	rc, _, err := st.Open(ctx, "attachments/1/hello.txt", nil)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	got, _ := io.ReadAll(rc)
	_ = rc.Close()
	if !bytes.Equal(got, body) {
		t.Fatalf("open returned %q, want %q", got, body)
	}

	// A byte range, as a media player seeking would ask for.
	rc, whole, err := st.Open(ctx, "attachments/1/hello.txt", &ByteRange{Start: 7, End: 12})
	if err != nil {
		t.Fatalf("open range: %v", err)
	}
	got, _ = io.ReadAll(rc)
	_ = rc.Close()
	if string(got) != "bucket" {
		t.Fatalf("range read %q, want %q", got, "bucket")
	}
	if whole.Size != int64(len(body)) {
		t.Fatalf("range read reported whole size %d, want %d", whole.Size, len(body))
	}

	if err := st.Delete(ctx, "attachments/1/hello.txt"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := st.Stat(ctx, "attachments/1/hello.txt"); err != ErrNotFound {
		t.Fatalf("stat after delete: got %v, want ErrNotFound", err)
	}
	if err := st.Delete(ctx, "attachments/1/hello.txt"); err != ErrNotFound {
		t.Fatalf("second delete: got %v, want ErrNotFound", err)
	}
}

func TestS3MissingObjectAndBadKeys(t *testing.T) {
	st := newTestS3(t, "")
	ctx := context.Background()
	if _, err := st.Stat(ctx, "avatars/nope.png"); err != ErrNotFound {
		t.Fatalf("stat missing: got %v, want ErrNotFound", err)
	}
	if _, _, err := st.Open(ctx, "avatars/nope.png", nil); err != ErrNotFound {
		t.Fatalf("open missing: got %v, want ErrNotFound", err)
	}
	// The same traversal rules as the filesystem backend: a bucket has no directories to
	// escape, but a key like "../x" must still never reach it.
	for _, k := range []string{"../etc/passwd", "a/../../b", "", "/", "a//b", "a\\b"} {
		if _, err := st.Stat(ctx, k); err != ErrInvalidKey {
			t.Errorf("stat %q: got %v, want ErrInvalidKey", k, err)
		}
	}
}

// The prefix keeps one bucket shareable: everything this instance writes lands under it.
func TestS3PrefixIsApplied(t *testing.T) {
	st := newTestS3(t, "instances/a")
	k, err := st.objectKey("avatars/x.png")
	if err != nil {
		t.Fatal(err)
	}
	if k != "instances/a/avatars/x.png" {
		t.Fatalf("object key %q, want %q", k, "instances/a/avatars/x.png")
	}
}
