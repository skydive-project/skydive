/*
 * Copyright (C) 2019 IBM, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy ofthe License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specificlanguage governing permissions and
 * limitations under the License.
 *
 */

package subscriber

import (
	"bytes"
	"strings"

	"github.com/IBM/ibm-cos-sdk-go/aws"
	"github.com/IBM/ibm-cos-sdk-go/aws/credentials"
	"github.com/IBM/ibm-cos-sdk-go/aws/credentials/ibmiam"
	"github.com/IBM/ibm-cos-sdk-go/aws/session"
	"github.com/IBM/ibm-cos-sdk-go/service/s3"
	"github.com/skydive-project/skydive/logging"
)

// objectStoreClient allows uploading objects to an object storage service
type objectStoreClient interface {
	// WriteObject stores a single object
	WriteObject(bucket, objectKey, data, contentType, contentEncoding string, metadata map[string]*string) error
	// ReadObject reads a single object
	ReadObject(bucket, objectKey string) ([]byte, error)
	// ReadObjectMetadata returns an object metadata
	ReadObjectMetadata(bucket, objectKey string) (map[string]*string, error)
	// ListObjects lists objects within a bucket
	ListObjects(bucket, prefix string) ([]*string, error)
	// DeleteObject deletes a single object
	DeleteObject(bucket, objectKey string) error
}

// s3Client allows uploading objects to an S3-compatible object storage service
type s3Client struct {
	s3c *s3.S3
}

// WriteObject stores a single object
func (s *s3Client) WriteObject(bucket, objectKey, data, contentType, contentEncoding string, metadata map[string]*string) error {
	_, err := s.s3c.PutObject(&s3.PutObjectInput{
		Body:            strings.NewReader(data),
		Bucket:          aws.String(bucket),
		ContentType:     aws.String(contentType),
		ContentEncoding: aws.String(contentEncoding),
		Key:             aws.String(objectKey),
		Metadata:        metadata,
	})

	logging.GetLogger().Infof("WriteObject %s result: %v", objectKey, err)

	return err
}

// ReadObject reads a single object
func (s *s3Client) ReadObject(bucket, objectKey string) ([]byte, error) {
	getObjectOutput, err := s.s3c.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(objectKey),
	})

	logging.GetLogger().Infof("ReadObject %s result: %v", objectKey, err)

	if err != nil {
		return nil, err
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(getObjectOutput.Body)
	return buf.Bytes(), nil
}

// ReadObjectMetadata returns an object metadata
func (s *s3Client) ReadObjectMetadata(bucket, objectKey string) (map[string]*string, error) {
	headObjectOutput, err := s.s3c.HeadObject(&s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(objectKey),
	})

	logging.GetLogger().Infof("ReadObjectMetadata %s result: %v", objectKey, err)

	if err != nil {
		return nil, err
	}

	return headObjectOutput.Metadata, nil
}

// DeleteObject deletes a single object
func (s *s3Client) DeleteObject(bucket, objectKey string) error {
	_, err := s.s3c.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(objectKey),
	})

	logging.GetLogger().Infof("DeleteObject %s result: %v", objectKey, err)

	return err
}

// ListObjects lists objects within a bucket
func (s *s3Client) ListObjects(bucket, prefix string) ([]*string, error) {
	params := &s3.ListObjectsInput{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	}
	objectKeys := make([]*string, 0)
	fn := func(page *s3.ListObjectsOutput, lastPage bool) bool {
		for _, object := range page.Contents {
			objectKeys = append(objectKeys, object.Key)
		}
		return true
	}
	err := s.s3c.ListObjectsPages(params, fn)
	logging.GetLogger().Infof("ListObjects result: %d, %v", len(objectKeys), err)
	if err != nil {
		return nil, err
	}

	return objectKeys, nil
}

// newClient creates a new S3-compatible object storage client
func newClient(endpoint, region, accessKey, secretKey, apiKey, iamEndpoint string) objectStoreClient {
	var sdkCreds *credentials.Credentials
	if apiKey != "" {
		sdkCreds = ibmiam.NewStaticCredentials(aws.NewConfig(), iamEndpoint, apiKey, "")
	} else {
		sdkCreds = credentials.NewStaticCredentials(accessKey, secretKey, "")
	}

	conf := aws.NewConfig().
		WithEndpoint(endpoint).
		WithCredentials(sdkCreds).
		WithS3ForcePathStyle(true).
		WithRegion(region)

	sess := session.Must(session.NewSession())
	s3c := s3.New(sess, conf)

	return &s3Client{
		s3c: s3c,
	}
}
