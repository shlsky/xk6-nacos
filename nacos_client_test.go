package xnacos

import (
	"crypto/tls"
	"errors"
	"github.com/dop251/goja"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/js/modulestest"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/fsext"
	"go.k6.io/k6/lib/testutils"
	"gopkg.in/guregu/null.v3"
	"io"
	"net/url"
	"os"
	"runtime"
	"strings"
	"sync"
	"testing"

	"go.k6.io/k6/lib/testutils/httpmultibin"
	"go.k6.io/k6/metrics"
)

const isWindows = runtime.GOOS == "windows"

// codeBlock represents an execution of a k6 script.
type codeBlock struct {
	code       string
	val        interface{}
	err        string
	windowsErr string
	asserts    func(*testing.T, *httpmultibin.HTTPMultiBin, chan metrics.SampleContainer, error)
}

type testcase struct {
	name       string
	setup      func(*httpmultibin.HTTPMultiBin)
	initString codeBlock // runs in the init context
	vuString   codeBlock // runs in the vu context
}

// TestModuleInstancet returns a new instance of the Kafka module for testing.
// nolint: golint,revive
func TestModuleInstance(t *testing.T) {

	tests := []testcase{
		{
			name: "BadTLS",
			setup: func(tb *httpmultibin.HTTPMultiBin) {
				// changing the pointer's value
				// for affecting the lib.State
				// that uses the same pointer
				*tb.TLSClientConfig = tls.Config{
					MinVersion: tls.VersionTLS13,
				}
			},
			initString: codeBlock{
				code: `var client = new nacos.NacosClient({
					ipAddr: "nacos.test.infra.ww5sawfyut0k.bitsvc.io",
					port: 8848,
					username: "nacos",
					password: "nacos",
					namespaceId: "efficiency-test",
			});`,
			},
			vuString: codeBlock{
				code: `
					for (var i=0;i<1000;i++){
						var ip = client.selectOneHealthyInstance("eff-pts-agent");
					}
					
			`,
				err: "",
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ts := newTestState(t)

			// setup necessary environment if needed by a test
			if tt.setup != nil {
				tt.setup(ts.httpBin)
			}

			val, err := ts.Run(tt.initString.code)
			assertResponse(t, tt.initString, err, val, ts)

			ts.ToVUContext()
			val, err = ts.Run(tt.vuString.code)
			assertResponse(t, tt.vuString, err, val, ts)
		})
	}
}

// newTestState creates a new test state.
func newTestState(t *testing.T) testState {
	t.Helper()

	tb := httpmultibin.NewHTTPMultiBin(t)

	samples := make(chan metrics.SampleContainer, 1000)
	testRuntime := modulestest.NewRuntime(t)

	cwd, err := os.Getwd() //nolint:forbidigo
	require.NoError(t, err)
	fs := fsext.NewOsFs()

	if isWindows {
		fs = fsext.NewTrimFilePathSeparatorFs(fs)
	}
	testRuntime.VU.InitEnvField.CWD = &url.URL{Path: cwd}
	testRuntime.VU.InitEnvField.FileSystems = map[string]fsext.Fs{"file": fs}

	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	logger.Out = io.Discard

	hook := testutils.NewLogHook()
	logger.AddHook(hook)

	recorder := &callRecorder{
		calls: make([]string, 0),
	}

	ts := testState{
		Runtime:      testRuntime,
		httpBin:      tb,
		samples:      samples,
		logger:       logger,
		loggerHook:   hook,
		callRecorder: recorder,
	}

	m, ok := New().NewModuleInstance(ts.VU).(*ModuleInstance)
	require.True(t, ok)
	require.NoError(t, ts.VU.Runtime().Set("nacos", m.Exports().Named))
	require.NoError(t, ts.VU.Runtime().Set("call", recorder.Call))

	return ts
}

type testState struct {
	*modulestest.Runtime
	httpBin      *httpmultibin.HTTPMultiBin
	samples      chan metrics.SampleContainer
	logger       logrus.FieldLogger
	loggerHook   *testutils.SimpleLogrusHook
	callRecorder *callRecorder
}

// callRecorder a helper type that records all calls
type callRecorder struct {
	sync.Mutex
	calls []string
}

func (r *callRecorder) Call(text string) {
	r.Lock()
	defer r.Unlock()

	r.calls = append(r.calls, text)
}

// Run replaces the httpbin address and runs the code.
func (ts *testState) Run(code string) (goja.Value, error) {
	return ts.VU.Runtime().RunString(ts.httpBin.Replacer.Replace(code))
}
func assertResponse(t *testing.T, cb codeBlock, err error, val goja.Value, ts testState) {
	if isWindows && cb.windowsErr != "" && err != nil {
		err = errors.New(strings.ReplaceAll(err.Error(), cb.windowsErr, cb.err))
	}
	if cb.err == "" {
		assert.NoError(t, err)
	} else {
		require.Error(t, err)
		assert.Contains(t, err.Error(), cb.err)
	}
	if cb.val != nil {
		require.NotNil(t, val)
		assert.Equal(t, cb.val, val.Export())
	}
	if cb.asserts != nil {
		cb.asserts(t, ts.httpBin, ts.samples, err)
	}
}

// ToInitContext moves the test state to the VU context.
func (ts *testState) ToVUContext() {
	registry := metrics.NewRegistry()
	root, err := lib.NewGroup("", nil)
	if err != nil {
		panic(err)
	}

	state := &lib.State{
		Group:     root,
		Dialer:    ts.httpBin.Dialer,
		TLSConfig: ts.httpBin.TLSClientConfig,
		Samples:   ts.samples,
		Options: lib.Options{
			SystemTags: metrics.NewSystemTagSet(
				metrics.TagName,
				metrics.TagURL,
			),
			UserAgent: null.StringFrom("k6-test"),
		},
		BuiltinMetrics: metrics.RegisterBuiltinMetrics(registry),
		Tags:           lib.NewVUStateTags(registry.RootTagSet()),
		Logger:         ts.logger,
	}

	ts.MoveToVUContext(state)
}
