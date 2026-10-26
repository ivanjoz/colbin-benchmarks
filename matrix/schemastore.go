package matrix

import (
	"strings"

	"github.com/ivanjoz/colbin-benchmarks/datasets/schemastore"
	"github.com/ivanjoz/colbin-benchmarks/datasets/schemastore/schemastoregen"
	"github.com/ivanjoz/colbin-benchmarks/formats"
)

// schemastoreCases is two cases per SchemaStore document: typed, as its struct,
// and dynamic, as encoding/json's map[string]any or []any.
func schemastoreCases() []Case {
	cases := []Case{
		typedDocument("circleciblank", schemastoregen.CircleCIBlank),
		typedDocument("circlecimatrix", schemastoregen.CircleCIMatrix),
		typedDocument("commitlint", schemastoregen.CommitLint),
		typedDocument("commitlintbasic", schemastoregen.CommitLintBasic),
		typedDocument("epr", schemastoregen.EPR),
		typedDocument("eslintrc", schemastoregen.ESLintRC),
		typedDocument("esmrc", schemastoregen.ESMRC),
		typedDocument("geojson", schemastoregen.GeoJSON),
		typedDocument("githubfundingblank", schemastoregen.GitHubFundingBlank),
		typedDocument("githubworkflow", schemastoregen.GitHubWorkflow),
		typedDocument("gruntcontribclean", schemastoregen.GruntContribClean),
		typedDocument("imageoptimizerwebjob", schemastoregen.ImageOptimizerWebJob),
		typedDocument("jsonereversesort", schemastoregen.JSONEReverseSort),
		typedDocument("jsonesort", schemastoregen.JSONESort),
		typedDocument("jsonfeed", schemastoregen.JSONFeed),
		typedDocument("jsonresume", schemastoregen.JSONResume),
		typedDocument("netcoreproject", schemastoregen.NetCoreProject),
		typedDocument("nightwatch", schemastoregen.Nightwatch),
		typedDocument("openweathermap", schemastoregen.OpenWeatherMap),
		typedDocument("openweatherroadrisk", schemastoregen.OpenWeatherRoadRisk),
		typedDocument("packagejson", schemastoregen.PackageJSON),
		typedDocument("packagejsonlintrc", schemastoregen.PackageJSONLintRC),
		typedDocument("sapcloudsdkpipeline", schemastoregen.SAPCloudSDKPipeline),
		typedDocument("travisnotifications", schemastoregen.TravisNotifications),
		typedDocument("tslintbasic", schemastoregen.TSLintBasic),
		typedDocument("tslintextend", schemastoregen.TSLintExtend),
		typedDocument("tslintmulti", schemastoregen.TSLintMulti),
	}
	for _, document := range schemastore.Documents {
		if document == "openweatherroadrisk" { // the one document whose root is an array
			cases = append(cases, dynamicDocument[[]any](document))
		} else {
			cases = append(cases, dynamicDocument[map[string]any](document))
		}
	}
	return cases
}

func typedDocument[T any](document string, adapters func() formats.Adapters[T]) Case {
	return schemastoreCase(document, func(data string) (*Loaded, error) {
		value, err := schemastore.Load[T](data, document)
		return bind(value, err, adapters())
	})
}

func dynamicDocument[T formats.Document](document string) Case {
	return schemastoreCase(document+"/dynamic", func(data string) (*Loaded, error) {
		return bindDynamic(schemastore.LoadDynamic[T](data, document))
	})
}

func schemastoreCase(name string, load func(data string) (*Loaded, error)) Case {
	document, _, _ := strings.Cut(name, "/")
	return Case{
		Name: "schemastore/" + name, Dataset: "schemastore", Timed: true,
		Fetch: "datasets/schemastore/fetch.sh",
		Load: func(data string) (*Loaded, error) {
			if err := missing(schemastore.File(data, document)); err != nil {
				return nil, err
			}
			return load(data)
		},
	}
}
