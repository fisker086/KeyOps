package registry

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecr"
)

func (s *Service) listTagsECR(ctx context.Context, m map[string]string, appName string) ([]string, error) {
	region := strings.TrimSpace(m["ecr_region"])
	accessKey := strings.TrimSpace(m["ecr_access_key_id"])
	secretKey := strings.TrimSpace(m["ecr_secret_access_key"])
	if region == "" {
		return nil, fmt.Errorf("ecr_region is required in registry settings")
	}
	repoName := appName
	cfg := &aws.Config{Region: aws.String(region)}
	if accessKey != "" && secretKey != "" {
		cfg.Credentials = credentials.NewStaticCredentials(accessKey, secretKey, "")
	}
	sess, err := session.NewSession(cfg)
	if err != nil {
		return nil, err
	}
	svc := ecr.New(sess)
	var tags []string
	err = svc.ListImagesPagesWithContext(ctx, &ecr.ListImagesInput{
		RepositoryName: aws.String(repoName),
		Filter:         &ecr.ListImagesFilter{TagStatus: aws.String(ecr.TagStatusTagged)},
	}, func(out *ecr.ListImagesOutput, last bool) bool {
		for _, img := range out.ImageIds {
			if img.ImageTag != nil && *img.ImageTag != "" {
				tags = append(tags, *img.ImageTag)
			}
		}
		return true
	})
	if err != nil {
		return nil, err
	}
	return tags, nil
}
