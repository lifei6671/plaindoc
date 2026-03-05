package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// BuildImageHostingObjectReadURLInput 定义对象读取链接构建参数。
type BuildImageHostingObjectReadURLInput struct {
	Provider  ImageHostingProvider
	ObjectKey string
	ObjectURL string
	FileName  string
	Purpose   DocumentAttachmentLinkPurpose
}

// BuildObjectReadURL 按 provider 配置生成对象读取链接（公开直链或预签名 URL）。
func (s *ImageHostingService) BuildObjectReadURL(
	ctx context.Context,
	config ImageHostingConfig,
	input BuildImageHostingObjectReadURLInput,
) (string, error) {
	provider := normalizeImageHostingProvider(string(input.Provider))
	if provider == "" || provider == ImageHostingProviderLocal {
		return "", errors.New("non-local storage provider is required")
	}

	objectKey := normalizeObjectStorageKey(input.ObjectKey)
	if config.DownloadStrategy(provider) == ImageHostingDownloadStrategyPublic {
		publicURL := buildPublicObjectReadURL(config, provider, objectKey, input.ObjectURL)
		if contentDisposition := buildObjectReadContentDisposition(input.Purpose, input.FileName); contentDisposition != "" {
			publicURL = appendObjectReadContentDisposition(publicURL, contentDisposition)
		}
		return publicURL, nil
	}
	if objectKey == "" {
		return "", errors.New("object key is empty")
	}

	switch provider {
	case ImageHostingProviderCloudflareR2:
		return buildCloudflareR2SignedObjectReadURL(ctx, config, input, objectKey)
	case ImageHostingProviderAliyunOSS:
		return buildAliyunOSSSignedObjectReadURL(config, input, objectKey)
	default:
		return "", fmt.Errorf("unsupported storage provider: %s", provider)
	}
}

func buildPublicObjectReadURL(
	config ImageHostingConfig,
	provider ImageHostingProvider,
	objectKey string,
	objectURL string,
) string {
	normalizedObjectURL := strings.TrimSpace(objectURL)
	if normalizedObjectURL != "" {
		return normalizedObjectURL
	}
	switch provider {
	case ImageHostingProviderCloudflareR2:
		return resolveObjectStoragePublicURL(config.CloudflareR2.PublicBaseURL, objectKey)
	case ImageHostingProviderAliyunOSS:
		return resolveObjectStoragePublicURL(config.AliyunOSS.PublicBaseURL, objectKey)
	default:
		return ""
	}
}

func appendObjectReadContentDisposition(rawURL string, contentDisposition string) string {
	normalizedURL := strings.TrimSpace(rawURL)
	if normalizedURL == "" {
		return ""
	}
	dispositionValue := strings.TrimSpace(contentDisposition)
	if dispositionValue == "" {
		return normalizedURL
	}
	parsedURL, err := url.Parse(normalizedURL)
	if err != nil {
		return normalizedURL
	}
	queryValues := parsedURL.Query()
	queryValues.Set("response-content-disposition", dispositionValue)
	parsedURL.RawQuery = queryValues.Encode()
	return parsedURL.String()
}

func buildCloudflareR2SignedObjectReadURL(
	ctx context.Context,
	config ImageHostingConfig,
	input BuildImageHostingObjectReadURLInput,
	objectKey string,
) (string, error) {
	accountID := strings.TrimSpace(config.CloudflareR2.AccountID)
	bucket := strings.TrimSpace(config.CloudflareR2.Bucket)
	accessKeyID := strings.TrimSpace(config.CloudflareR2.AccessKeyID)
	secretAccessKey := strings.TrimSpace(config.CloudflareR2.SecretAccessKey)
	if accountID == "" || bucket == "" || accessKeyID == "" || secretAccessKey == "" {
		return "", errors.New("cloudflare r2 config is incomplete for signed url")
	}

	endpoint := resolveCloudflareR2Endpoint(accountID)
	if endpoint == "" {
		return "", errors.New("cloudflare r2 endpoint is empty")
	}

	awsConfig := aws.Config{
		Region: "auto",
		Credentials: aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(
			accessKeyID,
			secretAccessKey,
			"",
		)),
		EndpointResolverWithOptions: aws.EndpointResolverWithOptionsFunc(
			func(service string, region string, _ ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{
					URL:               endpoint,
					SigningRegion:     "auto",
					HostnameImmutable: true,
				}, nil
			},
		),
	}
	client := s3.NewFromConfig(awsConfig, func(options *s3.Options) {
		options.UsePathStyle = true
	})
	presignClient := s3.NewPresignClient(client)
	requestInput := &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(objectKey),
	}
	if contentDisposition := buildObjectReadContentDisposition(input.Purpose, input.FileName); contentDisposition != "" {
		requestInput.ResponseContentDisposition = aws.String(contentDisposition)
	}

	ttl := config.SignedURLTTL(ImageHostingProviderCloudflareR2)
	presignedRequest, err := presignClient.PresignGetObject(
		ctx,
		requestInput,
		func(options *s3.PresignOptions) {
			options.Expires = ttl
		},
	)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(presignedRequest.URL), nil
}

func buildAliyunOSSSignedObjectReadURL(
	config ImageHostingConfig,
	input BuildImageHostingObjectReadURLInput,
	objectKey string,
) (string, error) {
	bucket := strings.TrimSpace(config.AliyunOSS.Bucket)
	accessKeyID := strings.TrimSpace(config.AliyunOSS.AccessKeyID)
	accessKeySecret := strings.TrimSpace(config.AliyunOSS.AccessKeySecret)
	if bucket == "" || accessKeyID == "" || accessKeySecret == "" {
		return "", errors.New("aliyun oss config is incomplete for signed url")
	}

	endpoint := resolveAliyunOSSEndpoint(config.AliyunOSS.Endpoint, config.AliyunOSS.Region)
	if endpoint == "" {
		return "", errors.New("aliyun oss endpoint is empty")
	}

	client, err := oss.New(endpoint, accessKeyID, accessKeySecret)
	if err != nil {
		return "", err
	}
	bucketClient, err := client.Bucket(bucket)
	if err != nil {
		return "", err
	}

	options := make([]oss.Option, 0, 1)
	if contentDisposition := buildObjectReadContentDisposition(input.Purpose, input.FileName); contentDisposition != "" {
		options = append(options, oss.ResponseContentDisposition(contentDisposition))
	}
	ttl := config.SignedURLTTL(ImageHostingProviderAliyunOSS)
	signedURL, err := bucketClient.SignURL(objectKey, oss.HTTPGet, int64(ttl.Seconds()), options...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(signedURL), nil
}

func resolveCloudflareR2Endpoint(accountID string) string {
	normalizedAccountID := strings.TrimSpace(accountID)
	if normalizedAccountID == "" {
		return ""
	}
	if strings.HasPrefix(normalizedAccountID, "https://") || strings.HasPrefix(normalizedAccountID, "http://") {
		return strings.TrimRight(normalizedAccountID, "/")
	}
	return "https://" + normalizedAccountID + ".r2.cloudflarestorage.com"
}

func resolveAliyunOSSEndpoint(endpoint string, region string) string {
	normalizedEndpoint := strings.TrimSpace(endpoint)
	if normalizedEndpoint != "" {
		if strings.HasPrefix(normalizedEndpoint, "https://") || strings.HasPrefix(normalizedEndpoint, "http://") {
			return strings.TrimRight(normalizedEndpoint, "/")
		}
		return "https://" + strings.TrimRight(normalizedEndpoint, "/")
	}
	normalizedRegion := strings.TrimSpace(region)
	if normalizedRegion == "" {
		return ""
	}
	return "https://oss-" + normalizedRegion + ".aliyuncs.com"
}

func resolveObjectStoragePublicURL(baseURL string, objectKey string) string {
	normalizedBaseURL := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	normalizedObjectKey := normalizeObjectStorageKey(objectKey)
	if normalizedBaseURL == "" || normalizedObjectKey == "" {
		return ""
	}
	return normalizedBaseURL + "/" + normalizedObjectKey
}

func normalizeObjectStorageKey(objectKey string) string {
	normalizedObjectKey := strings.TrimSpace(strings.TrimPrefix(objectKey, "/"))
	if normalizedObjectKey == "" {
		return ""
	}
	cleanObjectKey := path.Clean(normalizedObjectKey)
	if cleanObjectKey == "." || cleanObjectKey == "/" || strings.HasPrefix(cleanObjectKey, "../") {
		return ""
	}
	return cleanObjectKey
}

func buildObjectReadContentDisposition(purpose DocumentAttachmentLinkPurpose, fileName string) string {
	normalizedFileName := strings.TrimSpace(fileName)
	if normalizedFileName == "" {
		return ""
	}

	dispositionType := "attachment"
	if purpose == DocumentAttachmentLinkPurposePreview {
		dispositionType = "inline"
	}
	return fmt.Sprintf("%s; filename*=UTF-8''%s", dispositionType, url.PathEscape(normalizedFileName))
}
