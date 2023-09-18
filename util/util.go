package util

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sfn"
)

func ToTagMap(wdTags map[string]interface{}) map[string]string {
	tags := map[string]string{}
	for k, v := range wdTags {
		vs, ok := v.(string)
		if ok {
			tags[k] = vs
		}
	}
	return tags
}

func ToSFNTags(tags map[string]string) []*sfn.Tag {
	sfnTags := []*sfn.Tag{}
	for k, v := range tags {
		sfnTags = append(sfnTags, &sfn.Tag{
			Key:   aws.String(k),
			Value: aws.String(v),
		})
	}
	return sfnTags
}
