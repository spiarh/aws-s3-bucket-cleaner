package main

import (
	"flag"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/pkg/errors"
)

const (
	appName                      = "aws-s3-bucket-cleaner"
	defaultMaxArge time.Duration = 90 * 24 * time.Hour
)

type Clock interface {
	Since(t time.Time) time.Duration
}

type RealClock struct{}

func (RealClock) Since(t time.Time) time.Duration { return time.Since(t) }

var _ Clock = RealClock{}

type Client struct {
	logger zerolog.Logger
	clock  Clock
	s3iface.S3API
}

func New(clock Clock, s3Client s3iface.S3API) *Client {
	return &Client{
		logger: log.With().Str("name", appName).Logger(),
		clock:  clock,
		S3API:  s3Client,
	}
}

func newS3Session(region string) (*session.Session, error) {
	sess, err := session.NewSession(&aws.Config{Region: &region})
	if err != nil {
		return nil, err
	}

	if _, err := sess.Config.Credentials.Get(); err != nil {
		return nil, errors.Wrapf(err, "failed to get AWS credentials")
	}

	return sess, nil
}

func (c *Client) cleanUpObjects(bucket string, maxAge time.Duration, dryRun bool) (int, error) {
	listInput := &s3.ListObjectsV2Input{Bucket: aws.String(bucket)}
	objectsToDelete := []*s3.ObjectIdentifier{}
	fn := func(page *s3.ListObjectsV2Output, lastPage bool) bool {
		for _, object := range page.Contents {
			age := c.clock.Since(*object.LastModified)
			if age > maxAge {
				c.logger.Info().
					Str("bucket-name", *object.Key).
					Str("bucket-age", object.LastModified.String()).
					Msg("object marked for deletion")
				objectsToDelete = append(objectsToDelete, &s3.ObjectIdentifier{Key: object.Key})
			}
		}

		return lastPage
	}

	if err := c.ListObjectsV2Pages(listInput, fn); err != nil {
		return 0, errors.Wrapf(err, "failed to list objects")
	}

	if len(objectsToDelete) == 0 {
		log.Info().Msg("0 bucket found for deletion")
		return 0, nil
	}

	if dryRun {
		c.logger.Info().Msg("dry-run mode enabled, no object will be deleted")
		return 0, nil
	}

	deleteInput := &s3.DeleteObjectsInput{
		Bucket: aws.String(bucket),
		Delete: &s3.Delete{Objects: objectsToDelete},
	}

	c.logger.Info().Msgf("%d object(s) marked for deletion", len(deleteInput.Delete.Objects))
	c.logger.Info().Msg("deleting objects")
	deletedObjects, err := c.DeleteObjects(deleteInput)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to delete objects")
	}

	var deletedObjectsCount int
	if deletedObjects != nil {
		deletedObjectsCount = len(deletedObjects.Deleted)
	}
	c.logger.Info().Msgf("%d object(s) deleted", deletedObjectsCount)

	return deletedObjectsCount, nil
}

func main() {
	var exitCode = 0
	var (
		maxAge         time.Duration
		bucket, region string
		dryRun         bool
	)

	// Use pretty output if we are in a terminal, json otherwise.
	if isTTYAllocated() {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})
	}

	flag.DurationVar(&maxAge, "max-age", defaultMaxArge, "Delete objects older than the max age, 90d by default")
	flag.StringVar(&bucket, "bucket", os.Getenv("BUCKET_NAME"), "Bucket name, default to BUCKET_NAME environment variable")
	flag.StringVar(&region, "region", os.Getenv("BUCKET_REGION"), "Bucket region, default to BUCKET_REGION environment variable")
	flag.BoolVar(&dryRun, "dry-run", false, "Do not actually delete any object")
	flag.Parse()

	logger := log.With().Str("name", appName).Logger()
	logger.Info().Msg("started")

	if bucket == "" {
		logger.Fatal().Msg("bucket name not set from cli or environment variable BUCKET_NAME")
	}
	if region == "" {
		logger.Fatal().Msg("bucket region not set from cli or environment variable BUCKET_REGION")
	}

	s3Session, err := newS3Session(region)
	if err != nil {
		logger.Fatal().Err(err).Msg("unable to create AWS session")
	}

	client := New(RealClock{}, s3.New(s3Session))
	_, err = client.cleanUpObjects(bucket, maxAge, dryRun)
	if err != nil {
		logger.Error().Err(err).Msg("unable to clean-up objects")
		exitCode = 1
	}

	logger.Info().Msg("finished")
	os.Exit(exitCode)
}

// isTTYAllocated returns true when a TTY is allocated.
func isTTYAllocated() bool {
	if fileInfo, _ := os.Stdout.Stat(); (fileInfo.Mode() & os.ModeCharDevice) != 0 {
		return true
	}
	return false
}
