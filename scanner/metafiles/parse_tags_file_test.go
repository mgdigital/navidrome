package metafiles

import (
	"testing"

	"github.com/navidrome/navidrome/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSimpleMapFile(t *testing.T) {
	t.Parallel()

	result, err := parseTagsFile("test", []byte(`
testkey: testvalue
testInt: 10
testList:
  - test1
  - 11
testNull: null
testFloat: 0.01
testQuoted: "quoted"
testBool: true
testBoolUpper: FALSE
testBoolMixed: True
`))
	require.NoError(t, err)
	assert.Equal(t, TagsTransformers{
		TagKeyValues{
			Key:    "testkey",
			Values: []string{"testvalue"},
		},
		TagKeyValues{
			Key:    "testInt",
			Values: []string{"10"},
		},
		TagKeyValues{
			Key:    "testList",
			Values: []string{"test1", "11"},
		},
		TagKeyValues{
			Key:    "testNull",
			Values: []string(nil),
		},
		TagKeyValues{
			Key:    "testFloat",
			Values: []string{"0.01"},
		},
		TagKeyValues{
			Key:    "testQuoted",
			Values: []string{"quoted"},
		},
		TagKeyValues{
			Key:    "testBool",
			Values: []string{"1"},
		},
		TagKeyValues{
			Key:    "testBoolUpper",
			Values: []string{"0"},
		},
		TagKeyValues{
			Key:    "testBoolMixed",
			Values: []string{"1"},
		},
	}, result)
}

func TestParsePatternMap(t *testing.T) {
	t.Parallel()

	result, err := parseTagsFile("test", []byte(`
"**/*.flac":
 album: test
 artists:
   - artist1
   - artist2
"CD1/*.flac":
 discnumber: 1
`))
	require.NoError(t, err)
	assert.Equal(t, TagsTransformers{
		PathMatchTagsTransformer{
			BaseDir: "test",
			Pattern: "**/*.flac",
			Transformer: TagsTransformers{
				TagKeyValues{
					Key:    "album",
					Values: []string{"test"},
				},
				TagKeyValues{
					Key:    "artists",
					Values: []string{"artist1", "artist2"},
				},
			},
		},
		PathMatchTagsTransformer{
			BaseDir: "test",
			Pattern: "CD1/*.flac",
			Transformer: TagsTransformers{
				TagKeyValues{
					Key:    "discnumber",
					Values: []string{"1"},
				},
			},
		},
	}, result)
}

func TestParseExpression(t *testing.T) {
	t.Parallel()

	result, err := parseTagsFile("test", []byte(`
testkey: testvalue
testExpr:
  $: values.map(v, v + "!")
testNonExistentKeyExpr:
  $: values.map(v, v + "!")
artist:
  $: tags["title"][0].split(" - ")[0].split(" & ")
title:
  $: value.split(" - ")[1]
discnumber:
  $: 1 + 1
float:
  $: 0.01 + 0.01
bool:
  $: "true"
`))
	require.NoError(t, err)

	transformed := result.Transform("test/subdir/file.flac", model.RawTags{
		"TESTEXPR": []string{"test1", "test2"},
		"TITLE":    []string{"artist1 & artist2 - title"},
	})
	assert.Equal(t, model.RawTags{
		"testkey":                []string{"testvalue"},
		"testExpr":               []string{"test1!", "test2!"},
		"testNonExistentKeyExpr": []string{},
		"artist":                 []string{"artist1", "artist2"},
		"title":                  []string{"title"},
		"discnumber":             []string{"2"},
		"float":                  []string{"0.02"},
		"bool":                   []string{"1"},
	}, transformed)
}

func TestParsePathBasedExpression(t *testing.T) {
	t.Parallel()

	result, err := parseTagsFile("/Base Dir", []byte(`
artist:
  $: relativePath.split("/")[0]
album:
  $: relativePath.split("/")[1]
`))
	require.NoError(t, err)

	transformed := result.Transform("/Base Dir/Artist Name/Album Name/tags.yml", model.RawTags{
		"TITLE": []string{"The Song Title"},
	})
	assert.Equal(t, model.RawTags{
		"artist": []string{"Artist Name"},
		"album":  []string{"Album Name"},
		"TITLE":  []string{"The Song Title"},
	}, transformed)
}
