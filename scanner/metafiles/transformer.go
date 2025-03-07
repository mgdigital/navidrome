package metafiles

import (
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/navidrome/navidrome/model"
)

type TagsTransformer interface {
	Transform(filePath string, tags model.RawTags) model.RawTags
}

type TagsTransformers []TagsTransformer

func (l TagsTransformers) Transform(filePath string, tags model.RawTags) model.RawTags {
	for _, transform := range l {
		tags = transform.Transform(filePath, tags)
	}
	return tags
}

type PathMatchTagsTransformer struct {
	BasePath    string
	Pattern     string
	Transformer TagsTransformer
}

func (t PathMatchTagsTransformer) Transform(filePath string, tags model.RawTags) model.RawTags {
	if !doublestar.MatchUnvalidated(t.Pattern, filePath[len(t.BasePath)+1:]) {
		return tags
	}
	return t.Transformer.Transform(filePath, tags)
}

type TagKeyValues struct {
	Key    string
	Values []string
}

func (t TagKeyValues) Transform(_ string, tags model.RawTags) model.RawTags {
	newTags := make(model.RawTags)
	normKey := normalizeTagKey(t.Key)
	for k, v := range tags {
		if normalizeTagKey(k) != normKey {
			newTags[k] = v
		}
	}
	if t.Values != nil {
		newTags[t.Key] = t.Values
	}
	return newTags
}

func normalizeTagKey(key string) string {
	return strings.ToLower(key)
}
