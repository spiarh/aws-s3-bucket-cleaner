package main

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
)

type FakeClock struct{}

func (f FakeClock) Since(t time.Time) time.Duration {
	return f.Now().Sub(t)
}

func (FakeClock) Now() time.Time {
	return time.Date(2022, time.January, 12, 0, 0, 0, 0, time.UTC)
}

var _ Clock = FakeClock{}

type mockS3Client struct {
	listObjects []*s3.Object
	s3iface.S3API
}

func (m *mockS3Client) ListObjectsV2Pages(input *s3.ListObjectsV2Input, fn func(*s3.ListObjectsV2Output, bool) bool) error {
	output := &s3.ListObjectsV2Output{Contents: m.listObjects}
	fn(output, true)
	return nil
}

func (m *mockS3Client) DeleteObjects(input *s3.DeleteObjectsInput) (*s3.DeleteObjectsOutput, error) {
	deletedObjects := []*s3.DeletedObject{}
	for _, obj := range input.Delete.Objects {
		deletedObjects = append(deletedObjects, &s3.DeletedObject{
			Key: obj.Key,
		})
	}

	return &s3.DeleteObjectsOutput{Deleted: deletedObjects}, nil
}

func TestCleanUpObjects(t *testing.T) {
	var tests = []struct {
		name                   string
		listObjects            []*s3.Object
		maxAge                 time.Duration
		expectedDeletedObjects int
	}{
		{
			name:                   "no bucket to delete",
			listObjects:            []*s3.Object{},
			maxAge:                 60 * time.Minute,
			expectedDeletedObjects: 0,
		},
		{
			name: "no old-enough bucket to delete",
			listObjects: []*s3.Object{
				{
					Key:          aws.String("60min-old"),
					LastModified: aws.Time(FakeClock{}.Now().Add(-60 * time.Minute)),
				},
			},
			maxAge:                 60 * time.Minute,
			expectedDeletedObjects: 0,
		},
		{
			name: "buckets to delete",
			listObjects: []*s3.Object{
				{
					Key:          aws.String("10min-old"),
					LastModified: aws.Time(FakeClock{}.Now().Add(-10 * time.Minute)),
				},
				{
					Key:          aws.String("30min-old"),
					LastModified: aws.Time(FakeClock{}.Now().Add(-30 * time.Minute)),
				},
				{
					Key:          aws.String("60min-old"),
					LastModified: aws.Time(FakeClock{}.Now().Add(-60 * time.Minute)),
				},
			},
			maxAge:                 20 * time.Minute,
			expectedDeletedObjects: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := New(FakeClock{}, &mockS3Client{listObjects: tt.listObjects})

			deleted, err := client.cleanUpObjects("", tt.maxAge, false)
			if err != nil {
				t.Error(err)
			}

			if deleted != tt.expectedDeletedObjects {
				t.Errorf("expected deleted objects: %d, got: %d", deleted, tt.expectedDeletedObjects)
			}
		})
	}
}
