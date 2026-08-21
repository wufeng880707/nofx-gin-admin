package oss

import (
	"context"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type S3ClientConfig struct {
	Domain          string
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	BucketName      string
	Prefix          string
}

var _ IClient = (*S3Client)(nil)

type S3Client struct {
	config S3ClientConfig
	client *s3.Client
}

func NewS3Client(config S3ClientConfig) (*S3Client, error) {
	awsConfig, err := awsconfig.LoadDefaultConfig(
		context.Background(),
		awsconfig.WithRegion(config.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(config.AccessKeyID, config.SecretAccessKey, "")),
	)
	if err != nil {
		return nil, err
	}

	return &S3Client{
		config: config,
		client: s3.NewFromConfig(awsConfig),
	}, nil
}

func (c *S3Client) PutObject(ctx context.Context, bucketName, objectName string, reader io.ReadSeeker, objectSize int64, options ...PutObjectOptions) (*PutObjectResult, error) {
	if bucketName == "" {
		bucketName = c.config.BucketName
	}

	var opt PutObjectOptions
	if len(options) > 0 {
		opt = options[0]
	}

	objectName = formatObjectName(c.config.Prefix, objectName)
	input := &s3.PutObjectInput{
		Bucket:             aws.String(bucketName),
		Key:                aws.String(objectName),
		Body:               reader,
		ContentType:        aws.String(opt.ContentType),
		ContentDisposition: aws.String("inline"),
		ACL:                types.ObjectCannedACLPublicRead,
	}

	if len(opt.UserMetadata) > 0 {
		input.Metadata = opt.UserMetadata
	}

	output, err := c.client.PutObject(ctx, input)
	if err != nil {
		return nil, err
	}

	return &PutObjectResult{
		URL:  c.config.Domain + "/" + objectName,
		Key:  aws.ToString(input.Key),
		ETag: aws.ToString(output.ETag),
		Size: objectSize,
	}, nil
}

func (c *S3Client) GetObject(ctx context.Context, bucketName, objectName string) (io.ReadCloser, error) {
	if bucketName == "" {
		bucketName = c.config.BucketName
	}

	objectName = formatObjectName(c.config.Prefix, objectName)
	input := &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectName),
	}

	output, err := c.client.GetObject(ctx, input)
	if err != nil {
		return nil, err
	}

	return output.Body, nil
}

func (c *S3Client) RemoveObject(ctx context.Context, bucketName, objectName string) error {
	if bucketName == "" {
		bucketName = c.config.BucketName
	}

	objectName = formatObjectName(c.config.Prefix, objectName)
	input := &s3.DeleteObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectName),
	}

	_, err := c.client.DeleteObject(ctx, input)
	return err
}

func (c *S3Client) RemoveObjectByURL(ctx context.Context, urlStr string) error {
	prefix := c.config.Domain + "/"
	if !strings.HasPrefix(urlStr, prefix) {
		return nil
	}

	objectName := strings.TrimPrefix(urlStr, prefix)
	input := &s3.DeleteObjectInput{
		Bucket: aws.String(c.config.BucketName),
		Key:    aws.String(objectName),
	}

	_, err := c.client.DeleteObject(ctx, input)
	return err
}

func (c *S3Client) StatObjectByURL(ctx context.Context, urlStr string) (*ObjectStat, error) {
	prefix := c.config.Domain + "/"
	if !strings.HasPrefix(urlStr, prefix) {
		return nil, nil
	}

	objectName := strings.TrimPrefix(urlStr, prefix)
	return c.StatObject(ctx, c.config.BucketName, objectName)
}

func (c *S3Client) StatObject(ctx context.Context, bucketName, objectName string) (*ObjectStat, error) {
	if bucketName == "" {
		bucketName = c.config.BucketName
	}

	objectName = formatObjectName(c.config.Prefix, objectName)
	input := &s3.HeadObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectName),
	}

	output, err := c.client.HeadObject(ctx, input)
	if err != nil {
		return nil, err
	}

	var metadata map[string]string
	if output.Metadata != nil {
		metadata = output.Metadata
	}

	return &ObjectStat{
		Key:          objectName,
		ETag:         aws.ToString(output.ETag),
		LastModified: aws.ToTime(output.LastModified),
		Size:         aws.ToInt64(output.ContentLength),
		ContentType:  aws.ToString(output.ContentType),
		UserMetadata: metadata,
	}, nil
}
