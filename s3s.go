package s3s

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type App struct {
	s3client *s3.Client
}

func NewApp(ctx context.Context, region string) (*App, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, err
	}

	s3Client := s3.NewFromConfig(cfg)
	return &App{s3client: s3Client}, nil
}

func GetS3Bucket(ctx context.Context, app *App) ([]string, error) {
	input := &s3.ListBucketsInput{}
	output, err := app.s3client.ListBuckets(ctx, input)
	if err != nil {
		return nil, err
	}

	var s3keys = make([]string, len(output.Buckets))
	for i, content := range output.Buckets {
		s3keys[i] = *content.Name
	}

	return s3keys, nil
}

func GetS3Dir(ctx context.Context, app *App, bucket string, prefix string) ([]string, error) {
	input := &s3.ListObjectsV2Input{
		Bucket:    aws.String(bucket),
		Prefix:    aws.String(prefix),
		Delimiter: aws.String("/"),
	}
	pagenator := s3.NewListObjectsV2Paginator(app.s3client, input)

	var s3Keys []string
	for pagenator.HasMorePages() {
		output, err := pagenator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		pageKeys := make([]string, len(output.CommonPrefixes))
		for i := range output.CommonPrefixes {
			pageKeys[i] = *output.CommonPrefixes[i].Prefix
		}

		s3Keys = append(s3Keys, pageKeys...)
	}

	return s3Keys, nil
}

func GetS3Keys(ctx context.Context, app *App, bucket string, prefix string) ([]string, error) {
	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	}
	pagenator := s3.NewListObjectsV2Paginator(app.s3client, input)

	var s3Keys []string
	for pagenator.HasMorePages() {
		output, err := pagenator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		pageKeys := make([]string, output.KeyCount)
		for i := range output.Contents {
			pageKeys[i] = *output.Contents[i].Key
		}

		s3Keys = append(s3Keys, pageKeys...)
	}

	return s3Keys, nil
}

func S3Select(ctx context.Context, app *App, bucket string, key string, query string) error {
	compressionType := suggestCompressionType(key)

	params := &s3.SelectObjectContentInput{
		Bucket:          aws.String(bucket),
		Key:             aws.String(key),
		ExpressionType:  types.ExpressionTypeSql,
		Expression:      aws.String(query),
		RequestProgress: &types.RequestProgress{},
		InputSerialization: &types.InputSerialization{
			CompressionType: compressionType,
			JSON: &types.JSONInput{
				Type: types.JSONTypeLines,
			},
		},
		OutputSerialization: &types.OutputSerialization{
			JSON: &types.JSONOutput{},
		},
	}

	resp, err := app.s3client.SelectObjectContent(ctx, params)
	if err != nil {
		return err
	}
	stream := resp.GetStream()
	defer stream.Close()

	for event := range stream.Events() {
		v, ok := event.(*types.SelectObjectContentEventStreamMemberRecords)
		if ok {
			value := string(v.Value.Payload)
			fmt.Print(value)
		}
	}

	if err := stream.Err(); err != nil {
		return err
	}

	return nil
}

func suggestCompressionType(key string) types.CompressionType {
	switch {
	case strings.HasSuffix(key, ".gz"):
		return types.CompressionTypeGzip
	case strings.HasSuffix(key, ".bz2"):
		return types.CompressionTypeBzip2
	default:
		return types.CompressionTypeNone
	}
}
