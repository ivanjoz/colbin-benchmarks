package schemastore

// One root type per document, each a transliteration of the protobuf schema
// Viotti and Kinderkhedia wrote for it (benchmark/<doc>/protobuf/schema.proto
// in their repository): the same messages, fields, nesting and scalar types,
// as Go structs. They are hand-written, as theirs were.
//
// The `doc` tag is the document's own key for a field, read by load.go only;
// no format sees it, so every format encodes the same struct. `,omitempty`
// marks a key the document leaves out when the value is zero. A struct whose
// one field is tagged `doc:"[]"` is that field's array in the document (the
// wrappers protobuf needs for nested arrays), and a struct tagged
// `doc:",oneof"` on its first field is a protobuf oneof: one non-nil pointer,
// whichever the document's value fits first.
//
// protobuf to Go:
//
//	uint32, sint32, double, float   uint32, int32 (`pb:"sint"`), float64, float32
//	string, bool                    string, bool
//	repeated T, message             []T, the struct by value
//	google.protobuf.NullValue       *string, always nil: the document's null
//	oneof                           a struct of pointers, one set

// CircleCIBlank is circleciblank: CircleCI Definition (Blank).
type CircleCIBlank struct {
	Version float32 `cb:"1" doc:"version"`
}

// CircleCIMatrix is circlecimatrix: CircleCI Matrix Definition.
type CircleCIMatrix struct {
	Version   float64                 `cb:"1" doc:"version"`
	Workflows CircleCIMatrixWorkflows `cb:"2" doc:"workflows"`
}

type CircleCIMatrixWorkflows struct {
	Test CircleCIMatrixTestWorkflow `cb:"1" doc:"test"`
}

type CircleCIMatrixTestWorkflow struct {
	Jobs []CircleCIMatrixJob `cb:"1" doc:"jobs"`
}

type CircleCIMatrixJob struct {
	M1 CircleCIMatrixM1 `cb:"1" doc:"m1"`
}

type CircleCIMatrixM1 struct {
	Matrix CircleCIMatrixMatrix `cb:"1" doc:"matrix"`
}

type CircleCIMatrixMatrix struct {
	Parameters CircleCIMatrixParameters `cb:"1" doc:"parameters"`
}

type CircleCIMatrixParameters struct {
	A []uint32 `cb:"1" doc:"a"`
}

// CommitLint is commitlint: CommitLint Configuration.
type CommitLint struct {
	Rules CommitLintRules `cb:"1" doc:"rules"`
}

type CommitLintRules struct {
	ScopeCase   []CommitLintRule `cb:"1" doc:"scope-case"`
	SubjectCase []CommitLintRule `cb:"2" doc:"subject-case"`
}

type CommitLintRule struct {
	Level   *uint32            `cb:"1" doc:",oneof"`
	When    *string            `cb:"2"`
	Options *CommitLintOptions `cb:"3"`
}

type CommitLintOptions struct {
	Values []string `cb:"1" doc:"[]"`
}

// CommitLintBasic is commitlintbasic: CommitLint Configuration (Basic).
type CommitLintBasic struct {
	DefaultIgnores bool `cb:"1" doc:"defaultIgnores"`
}

// EPR is epr: Entry Point Regulation Manifest.
type EPR struct {
	Site               string    `cb:"1" doc:"site"`
	MaxAge             uint32    `cb:"2" doc:"maxAge"`
	ReportURL          string    `cb:"3" doc:"reportUrl"`
	DefaultNavBehavior string    `cb:"4" doc:"defaultNavBehavior"`
	DefaultResBehavior string    `cb:"5" doc:"defaultResBehavior"`
	Rules              []EPRRule `cb:"6" doc:"rules"`
}

type EPRRule struct {
	Path      string   `cb:"1" doc:"path,omitempty"`
	Regex     string   `cb:"2" doc:"regex,omitempty"`
	Types     []string `cb:"3" doc:"types"`
	AllowData bool     `cb:"4" doc:"allowData"`
}

// ESLintRC is eslintrc: ESLint Configuration Document.
type ESLintRC struct {
	Extension    string               `cb:"1" doc:"extends"`
	Parser       string               `cb:"2" doc:"parser"`
	EcmaFeatures ESLintRCEcmaFeatures `cb:"3" doc:"ecmaFeatures"`
	Plugins      []string             `cb:"4" doc:"plugins"`
	Env          ESLintRCEnvironment  `cb:"5" doc:"env"`
	Rules        ESLintRCRules        `cb:"6" doc:"rules"`
}

type ESLintRCEcmaFeatures struct {
	JSX bool `cb:"1" doc:"jsx"`
}

type ESLintRCEnvironment struct {
	Browser bool `cb:"1" doc:"browser"`
	Node    bool `cb:"2" doc:"node"`
	ES6     bool `cb:"3" doc:"es6"`
}

type ESLintRCJSDocOptions struct {
	RequireReturn bool `cb:"1" doc:"requireReturn"`
}

type ESLintRCJSDocRule struct {
	Numeric *uint32               `cb:"1" doc:",oneof"`
	Options *ESLintRCJSDocOptions `cb:"2"`
}

type ESLintRCRules struct {
	Eqeqeq                         uint32              `cb:"1" doc:"eqeqeq"`
	CommaDangle                    uint32              `cb:"2" doc:"comma-dangle"`
	NoConsole                      uint32              `cb:"3" doc:"no-console"`
	NoDebugger                     uint32              `cb:"4" doc:"no-debugger"`
	NoExtraSemi                    uint32              `cb:"5" doc:"no-extra-semi"`
	NoExtraParens                  uint32              `cb:"6" doc:"no-extra-parens"`
	NoIrregularWhitespace          uint32              `cb:"7" doc:"no-irregular-whitespace"`
	NoUndef                        uint32              `cb:"8" doc:"no-undef"`
	NoUnusedVars                   uint32              `cb:"9" doc:"no-unused-vars"`
	Semi                           uint32              `cb:"10" doc:"semi"`
	SemiSpacing                    uint32              `cb:"11" doc:"semi-spacing"`
	ValidJsdoc                     []ESLintRCJSDocRule `cb:"12" doc:"valid-jsdoc"`
	ReactDisplayName               uint32              `cb:"13" doc:"react/display-name"`
	ReactForbidPropTypes           uint32              `cb:"14" doc:"react/forbid-prop-types"`
	ReactJsxBooleanValue           uint32              `cb:"15" doc:"react/jsx-boolean-value"`
	ReactJsxClosingBracketLocation uint32              `cb:"16" doc:"react/jsx-closing-bracket-location"`
	ReactJsxCurlySpacing           uint32              `cb:"17" doc:"react/jsx-curly-spacing"`
	ReactJsxIndentProps            uint32              `cb:"18" doc:"react/jsx-indent-props"`
	ReactJsxMaxPropsPerLine        uint32              `cb:"19" doc:"react/jsx-max-props-per-line"`
	ReactJsxNoDuplicateProps       uint32              `cb:"20" doc:"react/jsx-no-duplicate-props"`
	ReactJsxNoLiterals             uint32              `cb:"21" doc:"react/jsx-no-literals"`
	ReactJsxNoUndef                uint32              `cb:"22" doc:"react/jsx-no-undef"`
	ReactJsxSortPropTypes          uint32              `cb:"23" doc:"react/jsx-sort-prop-types"`
	ReactJsxSortProps              uint32              `cb:"24" doc:"react/jsx-sort-props"`
	ReactJsxUsesReact              uint32              `cb:"25" doc:"react/jsx-uses-react"`
	ReactJsxUsesVars               uint32              `cb:"26" doc:"react/jsx-uses-vars"`
	ReactNoDanger                  uint32              `cb:"27" doc:"react/no-danger"`
	ReactNoDidMountSetState        uint32              `cb:"28" doc:"react/no-did-mount-set-state"`
	ReactNoDidUpdateSetState       uint32              `cb:"29" doc:"react/no-did-update-set-state"`
	ReactNoDirectMutationState     uint32              `cb:"30" doc:"react/no-direct-mutation-state"`
	ReactNoMultiComp               uint32              `cb:"31" doc:"react/no-multi-comp"`
	ReactNoSetState                uint32              `cb:"32" doc:"react/no-set-state"`
	ReactNoUnknownProperty         uint32              `cb:"33" doc:"react/no-unknown-property"`
	ReactPropTypes                 uint32              `cb:"34" doc:"react/prop-types"`
	ReactReactInJsxScope           uint32              `cb:"35" doc:"react/react-in-jsx-scope"`
	ReactRequireExtension          uint32              `cb:"36" doc:"react/require-extension"`
	ReactSelfClosingComp           uint32              `cb:"37" doc:"react/self-closing-comp"`
	ReactSortComp                  uint32              `cb:"38" doc:"react/sort-comp"`
	ReactWrapMultilines            uint32              `cb:"39" doc:"react/wrap-multilines"`
}

// ESMRC is esmrc: ECMAScript Module Loader Definition.
type ESMRC struct {
	CJS        bool     `cb:"1" doc:"cjs"`
	MainFields []string `cb:"2" doc:"mainFields"`
	Mode       string   `cb:"3" doc:"mode"`
	Force      bool     `cb:"4" doc:"force"`
	Cache      bool     `cb:"5" doc:"cache"`
	SourceMap  bool     `cb:"6" doc:"sourceMap"`
}

// GeoJSON is geojson: GeoJSON Example Document, a MultiPolygon.
type GeoJSON struct {
	Type        string           `cb:"1" doc:"type"`
	Coordinates []GeoJSONPolygon `cb:"2" doc:"coordinates"`
}

type GeoJSONPolygon struct {
	Rings []GeoJSONRing `cb:"1" doc:"[]"`
}

type GeoJSONRing struct {
	Positions []GeoJSONPosition `cb:"1" doc:"[]"`
}

type GeoJSONPosition struct {
	Coordinates []float64 `cb:"1" doc:"[]"`
}

// GitHubFundingBlank is githubfundingblank: GitHub FUNDING Sponsorship
// Definition (Empty).
type GitHubFundingBlank struct {
	GitHub          string  `cb:"1" doc:"github"`
	Patreon         *string `cb:"2" doc:"patreon"`
	OpenCollective  *string `cb:"3" doc:"open_collective"`
	KoFi            *string `cb:"4" doc:"ko_fi"`
	Tidelift        *string `cb:"5" doc:"tidelift"`
	CommunityBridge *string `cb:"6" doc:"community_bridge"`
	Liberapay       *string `cb:"7" doc:"liberapay"`
	IssueHunt       *string `cb:"8" doc:"issuehunt"`
	Otechie         *string `cb:"9" doc:"otechie"`
	Custom          *string `cb:"10" doc:"custom"`
}

// GitHubWorkflow is githubworkflow: GitHub Workflow Definition.
type GitHubWorkflow struct {
	Name string             `cb:"1" doc:"name"`
	On   []string           `cb:"2" doc:"on"`
	Jobs GitHubWorkflowJobs `cb:"3" doc:"jobs"`
}

type GitHubWorkflowJobs struct {
	Build GitHubWorkflowBuild `cb:"1" doc:"build"`
}

type GitHubWorkflowBuild struct {
	RunsOn string                    `cb:"1" doc:"runs-on"`
	Env    GitHubWorkflowEnvironment `cb:"2" doc:"env"`
	Steps  []GitHubWorkflowStep      `cb:"3" doc:"steps"`
}

type GitHubWorkflowEnvironment struct {
	BuildSuiteDir string `cb:"1" doc:"build-suite-dir"`
}

type GitHubWorkflowStep struct {
	Uses             string              `cb:"1" doc:"uses,omitempty"`
	Run              string              `cb:"2" doc:"run,omitempty"`
	WorkingDirectory string              `cb:"3" doc:"working-directory,omitempty"`
	With             *GitHubWorkflowWith `cb:"4" doc:"with,omitempty"`
	Name             string              `cb:"5" doc:"name,omitempty"`
}

type GitHubWorkflowWith struct {
	NodeVersion string `cb:"1" doc:"node-version"`
}

// GruntContribClean is gruntcontribclean: Grunt.js Clean Task Definition.
type GruntContribClean struct {
	Foo     []string                     `cb:"1" doc:"foo"`
	Main    GruntContribCleanMainOptions `cb:"2" doc:"main"`
	Options GruntContribCleanOptions     `cb:"3" doc:"options"`
}

type GruntContribCleanOptions struct {
	Force   bool `cb:"1" doc:"force"`
	NoWrite bool `cb:"2" doc:"no-write"`
}

type GruntContribCleanFiles struct{}

type GruntContribCleanMainOptions struct {
	Files GruntContribCleanFiles `cb:"1" doc:"files"`
	Src   []string               `cb:"2" doc:"src"`
}

// ImageOptimizerWebJob is imageoptimizerwebjob: ImageOptimizer Azure Webjob
// Configuration.
type ImageOptimizerWebJob struct {
	Optimizations []ImageOptimizerWebJobOptimization `cb:"1" doc:"optimizations"`
}

type ImageOptimizerWebJobOptimization struct {
	Includes []string `cb:"1" doc:"includes"`
	Excludes []string `cb:"2" doc:"excludes"`
	Lossy    bool     `cb:"3" doc:"lossy"`
}

// JSONEReverseSort is jsonereversesort: JSON-e Templating Engine Reverse Sort
// Example.
type JSONEReverseSort struct {
	Let JSONEReverseSortLet `cb:"1" doc:"$let"`
	In  JSONEReverseSortIn  `cb:"2" doc:"in"`
}

type JSONEReverseSortLet struct {
	X []uint32 `cb:"1" doc:"x"`
}

type JSONEReverseSortSort struct {
	Eval string `cb:"1" doc:"$eval"`
}

type JSONEReverseSortReverse struct {
	Sort JSONEReverseSortSort `cb:"1" doc:"$sort"`
	ByX  string               `cb:"2" doc:"by(x)"`
}

type JSONEReverseSortIn struct {
	Reverse JSONEReverseSortReverse `cb:"1" doc:"$reverse"`
}

// JSONESort is jsonesort: JSON-e Templating Engine Sort Example.
type JSONESort struct {
	Sort []uint32 `cb:"1" doc:"$sort"`
	ByX  string   `cb:"2" doc:"by(x)"`
}

// JSONFeed is jsonfeed: JSON Feed Example Document.
type JSONFeed struct {
	Version     string         `cb:"1" doc:"version"`
	UserComment string         `cb:"2" doc:"user_comment"`
	Title       string         `cb:"3" doc:"title"`
	HomePageURL string         `cb:"4" doc:"home_page_url"`
	FeedURL     string         `cb:"5" doc:"feed_url"`
	Author      JSONFeedAuthor `cb:"6" doc:"author"`
	Items       []JSONFeedItem `cb:"7" doc:"items"`
}

type JSONFeedAuthor struct {
	Name   string `cb:"1" doc:"name"`
	URL    string `cb:"2" doc:"url"`
	Avatar string `cb:"3" doc:"avatar"`
}

type JSONFeedItem struct {
	ID            string `cb:"1" doc:"id"`
	URL           string `cb:"2" doc:"url"`
	ContentText   string `cb:"3" doc:"content_text"`
	DatePublished string `cb:"4" doc:"date_published"`
}

// JSONResume is jsonresume: JSON Resume Example.
type JSONResume struct {
	Basics       JSONResumeBasics        `cb:"1" doc:"basics"`
	Work         []JSONResumeWork        `cb:"2" doc:"work"`
	Volunteer    []JSONResumeVolunteer   `cb:"3" doc:"volunteer"`
	Education    []JSONResumeEducation   `cb:"4" doc:"education"`
	Awards       []JSONResumeAward       `cb:"5" doc:"awards"`
	Publications []JSONResumePublication `cb:"6" doc:"publications"`
	Skills       []JSONResumeSkill       `cb:"7" doc:"skills"`
	Languages    []JSONResumeLanguage    `cb:"8" doc:"languages"`
	Interests    []JSONResumeInterest    `cb:"9" doc:"interests"`
	References   []JSONResumeReference   `cb:"10" doc:"references"`
}

type JSONResumeLocation struct {
	Address     string `cb:"1" doc:"address"`
	PostalCode  string `cb:"2" doc:"postalCode"`
	City        string `cb:"3" doc:"city"`
	CountryCode string `cb:"4" doc:"countryCode"`
	Region      string `cb:"5" doc:"region"`
}

type JSONResumeProfile struct {
	Network  string `cb:"1" doc:"network"`
	Username string `cb:"2" doc:"username"`
	URL      string `cb:"3" doc:"url"`
}

type JSONResumeBasics struct {
	Name     string              `cb:"1" doc:"name"`
	Label    string              `cb:"2" doc:"label"`
	Picture  string              `cb:"3" doc:"picture"`
	Email    string              `cb:"4" doc:"email"`
	Phone    string              `cb:"5" doc:"phone"`
	Website  string              `cb:"6" doc:"website"`
	Summary  string              `cb:"7" doc:"summary"`
	Location JSONResumeLocation  `cb:"8" doc:"location"`
	Profiles []JSONResumeProfile `cb:"9" doc:"profiles"`
}

type JSONResumeWork struct {
	Company    string   `cb:"1" doc:"company"`
	Position   string   `cb:"2" doc:"position"`
	Website    string   `cb:"3" doc:"website"`
	StartDate  string   `cb:"4" doc:"startDate"`
	EndDate    string   `cb:"5" doc:"endDate"`
	Summary    string   `cb:"6" doc:"summary"`
	Highlights []string `cb:"7" doc:"highlights"`
}

type JSONResumeVolunteer struct {
	Organization string   `cb:"1" doc:"organization"`
	Position     string   `cb:"2" doc:"position"`
	Website      string   `cb:"3" doc:"website"`
	StartDate    string   `cb:"4" doc:"startDate"`
	EndDate      string   `cb:"5" doc:"endDate"`
	Summary      string   `cb:"6" doc:"summary"`
	Highlights   []string `cb:"7" doc:"highlights"`
}

type JSONResumeEducation struct {
	Institution string   `cb:"1" doc:"institution"`
	Area        string   `cb:"2" doc:"area"`
	StudyType   string   `cb:"3" doc:"studyType"`
	StartDate   string   `cb:"4" doc:"startDate"`
	EndDate     string   `cb:"5" doc:"endDate"`
	GPA         string   `cb:"6" doc:"gpa"`
	Courses     []string `cb:"7" doc:"courses"`
}

type JSONResumeAward struct {
	Title   string `cb:"1" doc:"title"`
	Date    string `cb:"2" doc:"date"`
	Awarder string `cb:"3" doc:"awarder"`
	Summary string `cb:"4" doc:"summary"`
}

type JSONResumePublication struct {
	Name        string `cb:"1" doc:"name"`
	Publisher   string `cb:"2" doc:"publisher"`
	ReleaseDate string `cb:"3" doc:"releaseDate"`
	Website     string `cb:"4" doc:"website"`
	Summary     string `cb:"5" doc:"summary"`
}

type JSONResumeSkill struct {
	Name     string   `cb:"1" doc:"name"`
	Level    string   `cb:"2" doc:"level"`
	Keywords []string `cb:"3" doc:"keywords"`
}

type JSONResumeLanguage struct {
	Language string `cb:"1" doc:"language"`
	Fluency  string `cb:"2" doc:"fluency"`
}

type JSONResumeInterest struct {
	Name     string   `cb:"1" doc:"name"`
	Keywords []string `cb:"2" doc:"keywords"`
}

type JSONResumeReference struct {
	Name      string `cb:"1" doc:"name"`
	Reference string `cb:"2" doc:"reference"`
}

// NetCoreProject is netcoreproject: .NET Core Project.
type NetCoreProject struct {
	Version            string                           `cb:"1" doc:"version"`
	CompilationOptions NetCoreProjectCompilationOptions `cb:"2" doc:"compilationOptions"`
	Dependencies       NetCoreProjectDependencies       `cb:"3" doc:"dependencies"`
	Code               string                           `cb:"4" doc:"code"`
	Frameworks         NetCoreProjectFrameworks         `cb:"5" doc:"frameworks"`
}

type NetCoreProjectCompilationOptions struct {
	WarningsAsErrors bool `cb:"1" doc:"warningsAsErrors"`
}

type NetCoreProjectDependencies struct {
	MicrosoftBclImmutable              string `cb:"1" doc:"Microsoft.Bcl.Immutable"`
	MicrosoftAspNetConfigurationModel  string `cb:"2" doc:"Microsoft.AspNet.ConfigurationModel"`
	MicrosoftAspNetDependencyInjection string `cb:"3" doc:"Microsoft.AspNet.DependencyInjection"`
	MicrosoftAspNetLogging             string `cb:"4" doc:"Microsoft.AspNet.Logging"`
	SystemDataCommon                   string `cb:"5" doc:"System.Data.Common"`
}

type NetCoreProjectNet45Dependencies struct {
	SystemRuntime     string `cb:"1" doc:"System.Runtime"`
	SystemCollections string `cb:"2" doc:"System.Collections"`
}

type NetCoreProjectK10Dependencies struct {
	SystemCollections              string `cb:"1" doc:"System.Collections"`
	SystemCollectionsConcurrent    string `cb:"2" doc:"System.Collections.Concurrent"`
	SystemComponentModel           string `cb:"3" doc:"System.ComponentModel"`
	SystemConsole                  string `cb:"4" doc:"System.Console"`
	SystemDiagnosticsContracts     string `cb:"5" doc:"System.Diagnostics.Contracts"`
	SystemDiagnosticsDebug         string `cb:"6" doc:"System.Diagnostics.Debug"`
	SystemGlobalization            string `cb:"7" doc:"System.Globalization"`
	SystemLinq                     string `cb:"8" doc:"System.Linq"`
	SystemLinqExpressions          string `cb:"9" doc:"System.Linq.Expressions"`
	SystemLinqQueryable            string `cb:"10" doc:"System.Linq.Queryable"`
	SystemReflection               string `cb:"11" doc:"System.Reflection"`
	SystemReflectionExtensions     string `cb:"12" doc:"System.Reflection.Extensions"`
	SystemResourcesResourceManager string `cb:"13" doc:"System.Resources.ResourceManager"`
	SystemRuntime                  string `cb:"14" doc:"System.Runtime"`
	SystemRuntimeExtensions        string `cb:"15" doc:"System.Runtime.Extensions"`
	SystemThreading                string `cb:"16" doc:"System.Threading"`
	SystemThreadingTasks           string `cb:"17" doc:"System.Threading.Tasks"`
}

type NetCoreProjectNet45 struct {
	Dependencies NetCoreProjectNet45Dependencies `cb:"1" doc:"dependencies"`
}

type NetCoreProjectK10 struct {
	Dependencies NetCoreProjectK10Dependencies `cb:"1" doc:"dependencies"`
}

type NetCoreProjectFrameworks struct {
	Net45 NetCoreProjectNet45 `cb:"1" doc:"net45"`
	K10   NetCoreProjectK10   `cb:"2" doc:"k10"`
}

// Nightwatch is nightwatch: Nightwatch.js Test Framework Configuration.
type Nightwatch struct {
	CustomCommandsPath      *string                       `cb:"1" doc:"custom_commands_path"`
	CustomAssertionsPath    *string                       `cb:"2" doc:"custom_assertions_path"`
	PageObjectsPath         *string                       `cb:"3" doc:"page_objects_path"`
	GlobalsPath             *string                       `cb:"4" doc:"globals_path"`
	Globals                 NightwatchGlobals             `cb:"5" doc:"globals"`
	Dotenv                  NightwatchEmpty               `cb:"6" doc:"dotenv"`
	PersistGlobals          bool                          `cb:"7" doc:"persist_globals"`
	OutputFolder            string                        `cb:"8" doc:"output_folder"`
	SrcFolders              *string                       `cb:"9" doc:"src_folders"`
	LiveOutput              bool                          `cb:"10" doc:"live_output"`
	DisableColors           bool                          `cb:"11" doc:"disable_colors"`
	ParallelProcessDelay    uint32                        `cb:"12" doc:"parallel_process_delay"`
	Selenium                NightwatchSelenium            `cb:"13" doc:"selenium"`
	StartSession            bool                          `cb:"14" doc:"start_session"`
	EndSessionOnFail        bool                          `cb:"15" doc:"end_session_on_fail"`
	TestWorkers             bool                          `cb:"16" doc:"test_workers"`
	TestRunner              string                        `cb:"17" doc:"test_runner"`
	WebDriver               NightwatchWebDriver           `cb:"18" doc:"webdriver"`
	TestSettings            NightwatchEmpty               `cb:"19" doc:"test_settings"`
	LaunchURL               string                        `cb:"20" doc:"launch_url"`
	Silent                  bool                          `cb:"21" doc:"silent"`
	Output                  bool                          `cb:"22" doc:"output"`
	DetailedOutput          bool                          `cb:"23" doc:"detailed_output"`
	OutputTimestamp         bool                          `cb:"24" doc:"output_timestamp"`
	DisableErrorLog         bool                          `cb:"25" doc:"disable_error_log"`
	Screenshots             bool                          `cb:"26" doc:"screenshots"`
	LogScreenshotData       bool                          `cb:"27" doc:"log_screenshot_data"`
	DesiredCapabilities     NightwatchDesiredCapabilities `cb:"28" doc:"desiredCapabilities"`
	Exclude                 *string                       `cb:"29" doc:"exclude"`
	Filter                  *string                       `cb:"30" doc:"filter"`
	SkipGroup               string                        `cb:"31" doc:"skipgroup"`
	SyncTestNames           bool                          `cb:"32" doc:"sync_test_names"`
	SkipTags                string                        `cb:"33" doc:"skiptags"`
	UseXPath                bool                          `cb:"34" doc:"use_xpath"`
	ParallelMode            bool                          `cb:"35" doc:"parallel_mode"`
	ReportPrefix            string                        `cb:"36" doc:"report_prefix"`
	UnitTestsMode           bool                          `cb:"37" doc:"unit_tests_mode"`
	DefaultReporter         string                        `cb:"38" doc:"default_reporter"`
}

type NightwatchGlobals struct {
	AbortOnAssertionFailure                    bool   `cb:"1" doc:"abortOnAssertionFailure"`
	AbortOnElementLocateError                  bool   `cb:"2" doc:"abortOnElementLocateError"`
	WaitForConditionPollInterval               uint32 `cb:"3" doc:"waitForConditionPollInterval"`
	WaitForConditionTimeout                    uint32 `cb:"4" doc:"waitForConditionTimeout"`
	ThrowOnMultipleElementsReturned            bool   `cb:"5" doc:"throwOnMultipleElementsReturned"`
	SuppressWarningsOnMultipleElementsReturned bool   `cb:"6" doc:"suppressWarningsOnMultipleElementsReturned"`
	AsyncHookTimeout                           uint32 `cb:"7" doc:"asyncHookTimeout"`
	UnitTestsTimeout                           uint32 `cb:"8" doc:"unitTestsTimeout"`
	CustomReporterCallbackTimeout              uint32 `cb:"9" doc:"customReporterCallbackTimeout"`
	RetryAssertionTimeout                      uint32 `cb:"10" doc:"retryAssertionTimeout"`
}

type NightwatchEmpty struct{}

type NightwatchSelenium struct {
	StartProcess       bool            `cb:"1" doc:"start_process"`
	CLIArgs            NightwatchEmpty `cb:"2" doc:"cli_args"`
	ServerPath         *string         `cb:"3" doc:"server_path"`
	LogPath            string          `cb:"4" doc:"log_path"`
	CheckProcessDelay  uint32          `cb:"5" doc:"check_process_delay"`
	MaxStatusPollTries uint32          `cb:"6" doc:"max_status_poll_tries"`
	StatusPollInterval uint32          `cb:"7" doc:"status_poll_interval"`
}

type NightwatchWebDriver struct {
	StartProcess         bool            `cb:"1" doc:"start_process"`
	CLIArgs              NightwatchEmpty `cb:"2" doc:"cli_args"`
	ServerPath           *string         `cb:"3" doc:"server_path"`
	LogPath              string          `cb:"4" doc:"log_path"`
	CheckProcessDelay    uint32          `cb:"5" doc:"check_process_delay"`
	MaxStatusPollTries   uint32          `cb:"6" doc:"max_status_poll_tries"`
	StatusPollInterval   uint32          `cb:"7" doc:"status_poll_interval"`
	ProcessCreateTimeout uint32          `cb:"8" doc:"process_create_timeout"`
	TimeoutOptions       NightwatchEmpty `cb:"9" doc:"timeout_options"`
}

type NightwatchDesiredCapabilities struct {
	BrowserName string `cb:"1" doc:"browserName"`
}

// OpenWeatherMap is openweathermap: OpenWeatherMap API Example Document.
type OpenWeatherMap struct {
	Coord      OpenWeatherMapCoord     `cb:"1" doc:"coord"`
	Weather    []OpenWeatherMapWeather `cb:"2" doc:"weather"`
	Base       string                  `cb:"3" doc:"base"`
	Main       OpenWeatherMapMain      `cb:"4" doc:"main"`
	Visibility uint32                  `cb:"5" doc:"visibility"`
	Wind       OpenWeatherMapWind      `cb:"6" doc:"wind"`
	Clouds     OpenWeatherMapClouds    `cb:"7" doc:"clouds"`
	Dt         uint32                  `cb:"8" doc:"dt"`
	Sys        OpenWeatherMapSys       `cb:"9" doc:"sys"`
	Timezone   int32                   `cb:"10" doc:"timezone" pb:"sint"`
	ID         uint32                  `cb:"11" doc:"id"`
	Name       string                  `cb:"12" doc:"name"`
	Cod        uint32                  `cb:"13" doc:"cod"`
}

type OpenWeatherMapCoord struct {
	Lon float64 `cb:"1" doc:"lon"`
	Lat float64 `cb:"2" doc:"lat"`
}

type OpenWeatherMapWeather struct {
	ID          uint32 `cb:"1" doc:"id"`
	Main        string `cb:"2" doc:"main"`
	Description string `cb:"3" doc:"description"`
	Icon        string `cb:"4" doc:"icon"`
}

type OpenWeatherMapMain struct {
	Temp      float64 `cb:"1" doc:"temp"`
	FeelsLike float64 `cb:"2" doc:"feels_like"`
	TempMin   float64 `cb:"3" doc:"temp_min"`
	TempMax   float64 `cb:"4" doc:"temp_max"`
	Pressure  uint32  `cb:"5" doc:"pressure"`
	Humidity  uint32  `cb:"6" doc:"humidity"`
}

type OpenWeatherMapWind struct {
	Speed float64 `cb:"1" doc:"speed"`
	Deg   uint32  `cb:"2" doc:"deg"`
}

type OpenWeatherMapClouds struct {
	All uint32 `cb:"1" doc:"all"`
}

type OpenWeatherMapSys struct {
	Type    uint32  `cb:"1" doc:"type"`
	ID      uint32  `cb:"2" doc:"id"`
	Message float64 `cb:"3" doc:"message"`
	Country string  `cb:"4" doc:"country"`
	Sunrise uint32  `cb:"5" doc:"sunrise"`
	Sunset  uint32  `cb:"6" doc:"sunset"`
}

// OpenWeatherRoadRisk is openweatherroadrisk: OpenWeather Road Risk API
// Example. The document is an array; Viotti's schema wraps it in a message.
type OpenWeatherRoadRisk struct {
	Risks []OpenWeatherRoadRiskRisk `cb:"1" doc:"[]"`
}

type OpenWeatherRoadRiskAlert struct {
	SenderName string `cb:"1" doc:"sender_name"`
	Event      string `cb:"2" doc:"event"`
	EventLevel uint32 `cb:"3" doc:"event_level"`
}

type OpenWeatherRoadRiskWeather struct {
	Temp                   float64 `cb:"1" doc:"temp"`
	WindSpeed              float64 `cb:"2" doc:"wind_speed"`
	WindDeg                uint32  `cb:"3" doc:"wind_deg"`
	PrecipitationIntensity float64 `cb:"4" doc:"precipitation_intensity,omitempty"`
	DewPoint               float64 `cb:"5" doc:"dew_point"`
}

type OpenWeatherRoadRiskRisk struct {
	Dt      uint32                     `cb:"1" doc:"dt"`
	Coord   []float64                  `cb:"2" doc:"coord"`
	Weather OpenWeatherRoadRiskWeather `cb:"3" doc:"weather"`
	Alerts  []OpenWeatherRoadRiskAlert `cb:"4" doc:"alerts"`
}

// PackageJSON is packagejson: NPM Package.json Example Manifest.
type PackageJSON struct {
	Name            string                     `cb:"1" doc:"name"`
	Description     string                     `cb:"2" doc:"description"`
	Version         string                     `cb:"3" doc:"version"`
	Author          PackageJSONAuthor          `cb:"4" doc:"author"`
	Homepage        string                     `cb:"5" doc:"homepage"`
	Repository      PackageJSONRepository      `cb:"6" doc:"repository"`
	Bugs            PackageJSONBugs            `cb:"7" doc:"bugs"`
	Licenses        []PackageJSONLicense       `cb:"8" doc:"licenses"`
	Main            string                     `cb:"9" doc:"main"`
	Scripts         PackageJSONScripts         `cb:"10" doc:"scripts"`
	Engines         PackageJSONEngines         `cb:"11" doc:"engines"`
	Keywords        []string                   `cb:"12" doc:"keywords"`
	Dependencies    PackageJSONDependencies    `cb:"13" doc:"dependencies"`
	DevDependencies PackageJSONDevDependencies `cb:"14" doc:"devDependencies"`
	Readme          string                     `cb:"15" doc:"readme"`
	ID              string                     `cb:"16" doc:"_id"`
	From            string                     `cb:"17" doc:"_from"`
}

type PackageJSONAuthor struct {
	Name string `cb:"1" doc:"name"`
	URL  string `cb:"2" doc:"url"`
}

type PackageJSONRepository struct {
	Type string `cb:"1" doc:"type"`
	URL  string `cb:"2" doc:"url"`
}

type PackageJSONBugs struct {
	URL string `cb:"1" doc:"url"`
}

type PackageJSONLicense struct {
	Type string `cb:"1" doc:"type"`
	URL  string `cb:"2" doc:"url"`
}

type PackageJSONScripts struct {
	Test string `cb:"1" doc:"test"`
}

type PackageJSONEngines struct {
	Node string `cb:"1" doc:"node"`
}

type PackageJSONDependencies struct {
	Async            string `cb:"1" doc:"async"`
	CoffeeScript     string `cb:"2" doc:"coffee-script"`
	Colors           string `cb:"3" doc:"colors"`
	Dateformat       string `cb:"4" doc:"dateformat"`
	Eventemitter2    string `cb:"5" doc:"eventemitter2"`
	FindupSync       string `cb:"6" doc:"findup-sync"`
	Glob             string `cb:"7" doc:"glob"`
	Hooker           string `cb:"8" doc:"hooker"`
	IconvLite        string `cb:"9" doc:"iconv-lite"`
	Minimatch        string `cb:"10" doc:"minimatch"`
	Nopt             string `cb:"11" doc:"nopt"`
	Rimraf           string `cb:"12" doc:"rimraf"`
	Lodash           string `cb:"13" doc:"lodash"`
	UnderscoreString string `cb:"14" doc:"underscore.string"`
	Which            string `cb:"15" doc:"which"`
	JSYaml           string `cb:"16" doc:"js-yaml"`
	Exit             string `cb:"17" doc:"exit"`
	Getobject        string `cb:"18" doc:"getobject"`
	GruntLegacyUtil  string `cb:"19" doc:"grunt-legacy-util"`
	GruntLegacyLog   string `cb:"20" doc:"grunt-legacy-log"`
}

type PackageJSONDevDependencies struct {
	Temporary            string `cb:"1" doc:"temporary"`
	GruntContribJshint   string `cb:"2" doc:"grunt-contrib-jshint"`
	GruntContribNodeunit string `cb:"3" doc:"grunt-contrib-nodeunit"`
	GruntContribWatch    string `cb:"4" doc:"grunt-contrib-watch"`
	Difflet              string `cb:"5" doc:"difflet"`
	Semver               string `cb:"6" doc:"semver"`
	Shelljs              string `cb:"7" doc:"shelljs"`
}

// PackageJSONLintRC is packagejsonlintrc: NPM Package.json Linter
// Configuration Manifest.
type PackageJSONLintRC struct {
	Rules PackageJSONLintRCRules `cb:"1" doc:"rules"`
}

type PackageJSONLintRCOptions struct {
	Items []string `cb:"1" doc:"[]"`
}

type PackageJSONLintRCBooleanOptions struct {
	Items []bool `cb:"1" doc:"[]"`
}

type PackageJSONLintRCComplexRule struct {
	Textual        *string                          `cb:"1" doc:",oneof"`
	Options        *PackageJSONLintRCOptions        `cb:"2"`
	BooleanOptions *PackageJSONLintRCBooleanOptions `cb:"3"`
}

type PackageJSONLintRCRules struct {
	RequireAuthor                         string                         `cb:"1" doc:"require-author"`
	RequireDescription                    string                         `cb:"2" doc:"require-description"`
	RequireEngines                        string                         `cb:"3" doc:"require-engines"`
	RequireLicense                        string                         `cb:"4" doc:"require-license"`
	RequireName                           string                         `cb:"5" doc:"require-name"`
	RequireRepository                     string                         `cb:"6" doc:"require-repository"`
	RequireVersion                        string                         `cb:"7" doc:"require-version"`
	RequireBugs                           string                         `cb:"8" doc:"require-bugs"`
	RequireHomepage                       string                         `cb:"9" doc:"require-homepage"`
	RequireKeywords                       string                         `cb:"10" doc:"require-keywords"`
	BinType                               string                         `cb:"11" doc:"bin-type"`
	ConfigType                            string                         `cb:"12" doc:"config-type"`
	DescriptionType                       string                         `cb:"13" doc:"description-type"`
	DevDependenciesType                   string                         `cb:"14" doc:"devDependencies-type"`
	DirectoriesType                       string                         `cb:"15" doc:"directories-type"`
	EnginesType                           string                         `cb:"16" doc:"engines-type"`
	FilesType                             string                         `cb:"17" doc:"files-type"`
	HomepageType                          string                         `cb:"18" doc:"homepage-type"`
	KeywordsType                          string                         `cb:"19" doc:"keywords-type"`
	LicenseType                           string                         `cb:"20" doc:"license-type"`
	MainType                              string                         `cb:"21" doc:"main-type"`
	ManType                               string                         `cb:"22" doc:"man-type"`
	NameType                              string                         `cb:"23" doc:"name-type"`
	PreferGlobalType                      string                         `cb:"24" doc:"preferGlobal-type"`
	PrivateType                           string                         `cb:"25" doc:"private-type"`
	RepositoryType                        string                         `cb:"26" doc:"repository-type"`
	ScriptsType                           string                         `cb:"27" doc:"scripts-type"`
	VersionType                           string                         `cb:"28" doc:"version-type"`
	ValidValuesAuthor                     []PackageJSONLintRCComplexRule `cb:"29" doc:"valid-values-author"`
	ValidValuesPrivate                    []PackageJSONLintRCComplexRule `cb:"30" doc:"valid-values-private"`
	NoRestrictedDependencies              []PackageJSONLintRCComplexRule `cb:"31" doc:"no-restricted-dependencies"`
	NoRestrictedPreReleaseDependencies    []PackageJSONLintRCComplexRule `cb:"32" doc:"no-restricted-pre-release-dependencies"`
	NoRestrictedInvalidDevDependencies    []PackageJSONLintRCComplexRule `cb:"33" doc:"no-restricted-invalid-devDependencies"`
	NoRestrictedPreReleaseDevDependencies []PackageJSONLintRCComplexRule `cb:"34" doc:"no-restricted-pre-release-devDependencies"`
	NameFormat                            string                         `cb:"35" doc:"name-format"`
	VersionFormat                         string                         `cb:"36" doc:"version-format"`
}

// SAPCloudSDKPipeline is sapcloudsdkpipeline: SAP Cloud SDK Continuous
// Delivery Toolkit Configuration.
type SAPCloudSDKPipeline struct {
	General *string `cb:"1" doc:"general"`
	Stages  *string `cb:"2" doc:"stages"`
	Steps   *string `cb:"3" doc:"steps"`
}

// TravisNotifications is travisnotifications: TravisCI Notifications
// Configuration.
type TravisNotifications struct {
	Notifications TravisNotificationsNotifications `cb:"1" doc:"notifications"`
}

type TravisNotificationsNotification struct {
	Secure string `cb:"1" doc:"secure"`
}

type TravisNotificationsNotifications struct {
	Campfire TravisNotificationsNotification `cb:"1" doc:"campfire"`
	IRC      TravisNotificationsNotification `cb:"2" doc:"irc"`
	Flowdock TravisNotificationsNotification `cb:"3" doc:"flowdock"`
	HipChat  TravisNotificationsNotification `cb:"4" doc:"hipchat"`
	Slack    TravisNotificationsNotification `cb:"5" doc:"slack"`
	Webhooks TravisNotificationsNotification `cb:"6" doc:"webhooks"`
	Email    TravisNotificationsNotification `cb:"7" doc:"email"`
}

// TSLintBasic is tslintbasic: TSLint Linter Definition (Basic).
type TSLintBasic struct {
	Rules TSLintBasicRules `cb:"1" doc:"rules"`
}

type TSLintBasicOptions struct {
	GroupedImports bool `cb:"1" doc:"grouped-imports"`
}

type TSLintBasicOrderedImports struct {
	Options TSLintBasicOptions `cb:"1" doc:"options"`
}

type TSLintBasicRules struct {
	OrderedImports TSLintBasicOrderedImports `cb:"1" doc:"ordered-imports"`
}

// TSLintExtend is tslintextend: TSLint Linter Definition (Extends Only).
type TSLintExtend struct {
	Extensions []string `cb:"1" doc:"extends"`
}

// TSLintMulti is tslintmulti: TSLint Linter Definition (Multi-rule).
type TSLintMulti struct {
	Rules TSLintMultiRules `cb:"1" doc:"rules"`
}

type TSLintMultiOptions struct {
	GroupedImports bool `cb:"1" doc:"grouped-imports"`
}

type TSLintMultiOrderedImports struct {
	Options TSLintMultiOptions `cb:"1" doc:"options"`
}

type TSLintMultiRules struct {
	NoAny          []bool                    `cb:"1" doc:"no-any"`
	Radix          []bool                    `cb:"2" doc:"radix"`
	OrderedImports TSLintMultiOrderedImports `cb:"3" doc:"ordered-imports"`
}
