package registry

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecr/types"
)

func (s *Service) listTagsECR(ctx context.Context, m map[string]string, appName string) ([]string, error) {
	region := strings.TrimSpace(m["ecr_region"])
	accessKey := strings.TrimSpace(m["ecr_access_key_id"])
	secretKey := strings.TrimSpace(m["ecr_secret_access_key"])
	if region == "" {
		return nil, fmt.Errorf("ecr_region is required in registry settings")
	}

	optFns := []func(*config.LoadOptions) error{
		config.WithRegion(region),
	}
	if accessKey != "" && secretKey != "" {
		optFns = append(optFns, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		))
	}
	cfg, err := config.LoadDefaultConfig(ctx, optFns...)
	if err != nil {
		return nil, err
	}
	svc := ecr.NewFromConfig(cfg)

	paginator := ecr.NewListImagesPaginator(svc, &ecr.ListImagesInput{
		RepositoryName: aws.String(appName),
		Filter:         &types.ListImagesFilter{TagStatus: types.TagStatusTagged},
	})
	var tags []string
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, img := range out.ImageIds {
			if img.ImageTag != nil && *img.ImageTag != "" {
				tags = append(tags, *img.ImageTag)
			}
		}
	}
	return tags, nil
}
