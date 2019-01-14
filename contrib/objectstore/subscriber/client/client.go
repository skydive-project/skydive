package client

import (
	"bytes"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

// Client allows uploading objects to an object storage service
type Client interface {
	// WriteObject stores a single object
	WriteObject(bucket, objectKey, data, contentType, contentEncoding string, metadata map[string]*string) error
	// ReadObject reads a single object
	ReadObject(bucket, objectKey string) ([]byte, error)
	// ListObjects lists objects withing a bucket
	ListObjects(bucket, prefix string) ([]*string, error)
}

// S3Client allows uploading objects to an S3-compatible object storage service
type S3Client struct {
	s3Client *s3.S3
}

// WriteObject stores a single object
func (s *S3Client) WriteObject(bucket, objectKey, data, contentType, contentEncoding string, metadata map[string]*string) error {
	_, err := s.s3Client.PutObject(&s3.PutObjectInput{
		Body:            strings.NewReader(data),
		Bucket:          aws.String(bucket),
		ContentType:     aws.String(contentType),
		ContentEncoding: aws.String(contentEncoding),
		Key:             aws.String(objectKey),
		Metadata:        metadata,
	})

	return err
}

// ReadObject reads a single object
func (s *S3Client) ReadObject(bucket, objectKey string) ([]byte, error) {
	getObjectOutput, err := s.s3Client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(objectKey),
	})

	if err != nil {
		return nil, err
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(getObjectOutput.Body)
	return buf.Bytes(), nil
}

// ListObjects lists objects withing a bucket
func (s *S3Client) ListObjects(bucket, prefix string) ([]*string, error) {
	params := &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	}
	objectKeys := make([]*string, 0)
	fn := func(page *s3.ListObjectsV2Output, lastPage bool) bool {
		for _, object := range page.Contents {
			objectKeys = append(objectKeys, object.Key)
		}
		return true
	}
	err := s.s3Client.ListObjectsV2Pages(params, fn)
	if err != nil {
		return nil, err
	}

	return objectKeys, nil
}

// New creates a new S3-compatible object storage client
func New(endpoint, region, accessKey, secretKey string) Client {
	s3Client := s3.New(session.New(&aws.Config{
		S3ForcePathStyle: aws.Bool(true),
		Endpoint:         aws.String(endpoint),
		Credentials:      credentials.NewStaticCredentials(accessKey, secretKey, ""),
		Region:           aws.String(region),
	}))
	return &S3Client{
		s3Client: s3Client,
	}
}
