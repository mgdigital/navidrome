package metafiles

import (
	"cmp"
	"slices"
	"strconv"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types/ref"
	"github.com/navidrome/navidrome/log"
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
	BaseDir     string
	Pattern     string
	Transformer TagsTransformer
}

func (t PathMatchTagsTransformer) Transform(filePath string, tags model.RawTags) model.RawTags {
	if !sanityCheckFilePath(t.BaseDir, filePath) {
		return tags
	}
	if !doublestar.MatchUnvalidated(t.Pattern, filePath[len(t.BaseDir)+1:]) {
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

type TagExpression struct {
	BaseDir string
	Key     string
	Program cel.Program
}

func (t TagExpression) Transform(filePath string, tags model.RawTags) model.RawTags {
	if !sanityCheckFilePath(t.BaseDir, filePath) {
		return tags
	}
	values := extractTagValues(t.Key, tags)
	value := ""
	if len(values) > 0 {
		value = values[0]
	}
	result, _, err := t.Program.Eval(map[string]any{
		"tags":         extractAllTagValues(tags),
		"key":          t.Key,
		"values":       values,
		"value":        value,
		"fullPath":     filePath,
		"relativePath": filePath[len(t.BaseDir)+1:],
		"baseDir":      t.BaseDir,
	})
	if err != nil {
		log.Error("error evaluating tag expression", "error", err)
		return tags
	}
	return TagKeyValues{
		Key:    t.Key,
		Values: resolveCelValue(result.Value()),
	}.Transform(filePath, tags)
}

func extractTagValues(key string, tags model.RawTags) []string {
	key = normalizeTagKey(key)
	var kvs []TagKeyValues
	for k, v := range tags {
		if normalizeTagKey(k) == key {
			kvs = append(kvs, TagKeyValues{
				Key:    k,
				Values: v,
			})
		}
	}
	slices.SortFunc(kvs, func(i, j TagKeyValues) int {
		return cmp.Compare(i.Key, j.Key)
	})
	var values []string
	for _, kv := range kvs {
		values = append(values, kv.Values...)
	}
	return values
}

func extractAllTagValues(tags model.RawTags) map[string][]string {
	var kvs []TagKeyValues
	for k, v := range tags {
		kvs = append(kvs, TagKeyValues{
			Key:    k,
			Values: v,
		})
	}
	slices.SortFunc(kvs, func(i, j TagKeyValues) int {
		return cmp.Compare(i.Key, j.Key)
	})
	keyValuesMap := make(map[string][]string)
	for _, kv := range kvs {
		normKey := normalizeTagKey(kv.Key)
		keyValuesMap[normKey] = append(keyValuesMap[normKey], kv.Values...)
		if normKey != kv.Key {
			keyValuesMap[kv.Key] = append(keyValuesMap[kv.Key], kv.Values...)
		}
	}
	return keyValuesMap
}

func resolveCelValue(value any) []string {
	switch typed := value.(type) {
	case ref.Val:
		return resolveCelValue(typed.Value())
	case []ref.Val:
		strValues := make([]string, 0, len(typed))
		for _, val := range typed {
			strValues = append(strValues, resolveCelValue(val.Value())...)
		}
		return strValues
	case []string:
		return typed
	case string:
		return []string{typed}
	case []any:
		strValues := make([]string, 0, len(typed))
		for _, val := range typed {
			strValues = append(strValues, resolveCelValue(val)...)
		}
		return strValues
	case int64:
		return []string{strconv.Itoa(int(typed))}
	case float64:
		return []string{strconv.FormatFloat(typed, 'f', -1, 64)}
	case bool:
		if typed {
			return []string{"1"}
		}
		return []string{"0"}
	default:
		if str, ok := value.(string); ok {
			return []string{str}
		}
		log.Error("error converting tag expression result to strValues", "value", value)
		return nil
	}
}

func sanityCheckFilePath(baseDir, filePath string) bool {
	if !strings.HasPrefix(filePath, baseDir+"/") {
		log.Error("file path does not start with base dir", "baseDir", baseDir, "filePath", filePath)
		return false
	}
	return true
}
