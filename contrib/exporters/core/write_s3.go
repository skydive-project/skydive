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

package core

import (
	"strings"

	"github.com/spf13/viper"

	"github.com/IBM/ibm-cos-sdk-go/aws"
	"github.com/IBM/ibm-cos-sdk-go/aws/credentials"
	"github.com/IBM/ibm-cos-sdk-go/aws/credentials/ibmiam"
	"github.com/IBM/ibm-cos-sdk-go/aws/session"
	"github.com/IBM/ibm-cos-sdk-go/service/s3"

	"github.com/skydive-project/skydive/logging"
)

type writeS3 struct {
	s3c *s3.S3
}

// Write stores a single file
func (s *writeS3) Write(dirname, filename, content, contentType, contentEncoding string,
	metadata map[string]*string) error {
	_, err := s.s3c.PutObject(&s3.PutObjectInput{
		Body:            strings.NewReader(content),
		Bucket:          aws.String(dirname),
		ContentType:     aws.String(contentType),
		ContentEncoding: aws.String(contentEncoding),
		Key:             aws.String(filename),
		Metadata:        metadata,
	})

	logging.GetLogger().Infof("Write %s result: %v", filename, err)

	return err
}

// NewWriteS3 creates a new S3-compatible object storage client
func NewWriteS3(cfg *viper.Viper) (interface{}, error) {
	endpoint := cfg.GetString(CfgRoot + "write.s3.endpoint")
	region := cfg.GetString(CfgRoot + "write.s3.region")
	accessKey := cfg.GetString(CfgRoot + "write.s3.access_key")
	secretKey := cfg.GetString(CfgRoot + "write.s3.secret_key")
	apiKey := cfg.GetString(CfgRoot + "write.s3.api_key")
	iamEndpoint := cfg.GetString(CfgRoot + "write.s3.iam_endpoint")

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

	return &writeS3{
		s3c: s3c,
	}, nil
}
