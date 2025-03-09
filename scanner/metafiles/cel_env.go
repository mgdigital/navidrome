package metafiles

import (
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/ext"
)

var celEnv = func() *cel.Env {
	env, err := newCelEnv()
	if err != nil {
		panic(err)
	}
	return env
}()

func newCelEnv() (*cel.Env, error) {
	return cel.NewCustomEnv(
		cel.StdLib(),
		cel.EagerlyValidateDeclarations(true),
		cel.ExtendedValidations(),
		ext.Strings(ext.StringsValidateFormatCalls(true)),
		cel.Variable("tags", cel.MapType(cel.StringType, cel.ListType(cel.StringType))),
		cel.Variable("key", cel.StringType),
		cel.Variable("values", cel.ListType(cel.StringType)),
		cel.Variable("value", cel.StringType),
		cel.Variable("fullPath", cel.StringType),
		cel.Variable("relativePath", cel.StringType),
		cel.Variable("baseDir", cel.StringType),
		cel.Variable("filename", cel.StringType),
	)
}
