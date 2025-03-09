package metafiles

import (
	"fmt"
	"os"

	"github.com/navidrome/navidrome/log"
	"path/filepath"
)

func ReadFiles(baseDir string, filePaths []string) TagsTransformer {
	var tags TagsTransformers
	for _, filePath := range filePaths {
		if transform, err := readFile(baseDir, filePath); err != nil {
			log.Error(err)
			return TagKeyValues{
				Key: "comment",
				Values: []string{
					err.Error(),
				},
			}
		} else {
			tags = append(tags, transform)
		}
	}
	return tags
}

func readFile(baseDir, filePath string) (TagsTransformer, error) {
	fullPath := filepath.Join(baseDir, filePath)
	bytes, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf(`could not read tags file "%s": %w`, fullPath, err)
	}
	basePath := filepath.Dir(filePath)
	tags, err := parseTagsFile(basePath, bytes)
	if err != nil {
		return nil, fmt.Errorf(`could not parse tags file "%s": %w`, fullPath, err)
	}
	return tags, nil
}
