package metafiles

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/google/cel-go/cel"
	"gopkg.in/yaml.v3"
)

var ErrParseTagsFile = errors.New("failed to parse tags file")

func errAtPath(err error, path []string) error {
	if len(path) == 0 {
		return err
	}
	return fmt.Errorf("%w at path '%s'", err, strings.Join(path, "."))
}

func parseTagsFile(baseDir string, bytes []byte) (TagsTransformer, error) {
	var raw yaml.Node
	if err := yaml.Unmarshal(bytes, &raw); err != nil {
		return nil, fmt.Errorf("%w: tags file contains invalid YAML: %w", ErrParseTagsFile, err)
	}
	switch raw.Kind {
	case yaml.MappingNode:
	case yaml.DocumentNode:
		if len(raw.Content) == 0 {
			return nil, fmt.Errorf("%w: tags file is empty", ErrParseTagsFile)
		}
		raw = *raw.Content[0]
	default:
		return nil, fmt.Errorf("%w: tags file must be a mapping or document", ErrParseTagsFile)
	}
	var errs []error
	if tags, err := parseSimpleMap(baseDir, raw, nil); err == nil {
		return tags, nil
	} else {
		errs = append(errs, err)
	}
	if transforms, err := parsePatternMap(baseDir, raw, nil); err == nil {
		return transforms, nil
	} else {
		errs = append(errs, err)
	}
	return nil, fmt.Errorf("%w: %w", ErrParseTagsFile, errors.Join(errs...))
}

var keyRegex = regexp.MustCompile(`^[a-zA-Z0-9_ \-.:©]+$`)

func parseSimpleMap(baseDir string, raw yaml.Node, docPath []string) (TagsTransformer, error) {
	if raw.Kind != yaml.MappingNode {
		return nil, errAtPath(errors.New("entry must be a mapping"), docPath)
	}
	var transforms TagsTransformers
	for i := 0; i < len(raw.Content); i += 2 {
		keyNode := raw.Content[i]
		if transform, err := parseKeyValueNode(baseDir, *keyNode, *raw.Content[i+1], append(docPath, keyNode.Value)); err == nil {
			transforms = append(transforms, transform)
		} else {
			return nil, err
		}
	}
	return transforms, nil
}

func parsePatternMap(baseDir string, raw yaml.Node, docPath []string) (TagsTransformer, error) {
	if raw.Kind != yaml.MappingNode {
		return nil, errAtPath(errors.New("tags file entry must be a mapping"), docPath)
	}
	var transformers TagsTransformers
	for i := 0; i < len(raw.Content); i += 2 {
		keyNode := raw.Content[i]
		valueNode := raw.Content[i+1]
		if keyNode.Kind != yaml.ScalarNode {
			return nil, errAtPath(errors.New("key must be a scalar"), docPath)
		}
		if keyNode.Tag != "!!str" {
			return nil, errAtPath(errors.New("key must be a string"), docPath)
		}
		if !doublestar.ValidatePattern(keyNode.Value) {
			return nil, errAtPath(errors.New("key must be a valid glob pattern"), append(docPath, keyNode.Value))
		}
		transformer, err := parseSimpleMap(baseDir, *valueNode, append(docPath, keyNode.Value))
		if err != nil {
			return nil, err
		}
		transformers = append(transformers, PathMatchTagsTransformer{
			BaseDir:     baseDir,
			Pattern:     keyNode.Value,
			Transformer: transformer,
		})
	}
	return transformers, nil
}

func parseKeyValueNode(baseDir string, keyNode yaml.Node, valueNode yaml.Node, docPath []string) (TagsTransformer, error) {
	if keyNode.Kind != yaml.ScalarNode {
		return nil, errAtPath(errors.New("tag key must be a scalar"), docPath)
	}
	if keyNode.Tag != "!!str" {
		return nil, errAtPath(errors.New("tag key must be a string"), docPath)
	}
	if !keyRegex.MatchString(keyNode.Value) {
		return nil, errAtPath(errors.New("tag key must match pattern"), append(docPath, keyNode.Value))
	}
	if expr, err := parseTagExpression(baseDir, keyNode.Value, valueNode, docPath); err == nil {
		return expr, nil
	} else if errors.Is(err, ErrParseTagExpression) {
		return nil, err
	}
	values, err := parseSimpleTagValue(valueNode, append(docPath, keyNode.Value))
	if err != nil {
		return nil, err
	}
	return TagKeyValues{
		Key:    keyNode.Value,
		Values: values,
	}, nil
}

func parseSimpleTagValue(raw yaml.Node, docPath []string) ([]string, error) {
	switch raw.Kind {
	case yaml.ScalarNode:
		switch raw.Tag {
		case "!!null":
			return nil, nil
		case "!!bool":
			value := "0"
			if strings.ToLower(raw.Value) == "true" {
				value = "1"
			}
			return []string{value}, nil
		case "!!str", "!!int", "!!float":
			return []string{raw.Value}, nil
		default:
			return nil, errAtPath(errors.New("invalid tag value"), docPath)
		}
	case yaml.SequenceNode:
		values := make([]string, 0, len(raw.Content))
		for i, valueNode := range raw.Content {
			value, err := parseSimpleTagValue(*valueNode, append(docPath, fmt.Sprintf("[%d]", i)))
			if err != nil {
				return nil, err
			}
			values = append(values, value...)
		}
		return values, nil
	default:
		return nil, errAtPath(errors.New("invalid tag value"), docPath)
	}
}

var ErrParseTagExpression = errors.New("failed to parse tag expression")

func parseTagExpression(baseDir string, tagKey string, raw yaml.Node, docPath []string) (TagExpression, error) {
	if raw.Kind != yaml.MappingNode {
		return TagExpression{}, errAtPath(errors.New("tag expression must be a mapping"), docPath)
	}
	if len(raw.Content) != 2 {
		return TagExpression{}, errAtPath(errors.New("tag expression must have exactly one key"), docPath)
	}
	keyNode := raw.Content[0]
	if keyNode.Kind != yaml.ScalarNode || keyNode.Tag != "!!str" || keyNode.Value != "$" {
		return TagExpression{}, errAtPath(errors.New("tag expression key must be '$'"), docPath)
	}
	valueNode := raw.Content[1]
	if valueNode.Kind != yaml.ScalarNode || valueNode.Tag != "!!str" {
		return TagExpression{}, errAtPath(fmt.Errorf("%w: value must be a string", ErrParseTagExpression), docPath)
	}
	ast, issues := celEnv.Compile(valueNode.Value)
	if issues != nil && issues.Err() != nil {
		return TagExpression{}, errAtPath(fmt.Errorf("%w: expression could not be compiled: %w", ErrParseTagExpression, issues.Err()), docPath)
	}
	prg, err := celEnv.Program(ast, cel.EvalOptions(cel.OptOptimize))
	if err != nil {
		return TagExpression{}, errAtPath(fmt.Errorf("%w: program could not be constructed: %w", ErrParseTagExpression, err), docPath)
	}
	return TagExpression{
		BaseDir: baseDir,
		Key:     tagKey,
		Program: prg,
	}, nil
}
