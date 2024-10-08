package eris_test

import (
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/risingwavelabs/eris"
)

var (
	globalErr          = eris.New("global error").WithCode(eris.CodeUnknown)
	formattedGlobalErr = eris.Errorf("%v global error", "formatted").WithCode(eris.CodeUnknown)
)

type withMessage struct {
	msg string
}

func (e withMessage) Error() string { return e.msg }
func (e withMessage) Is(target error) bool {
	if err, ok := target.(withMessage); ok {
		return e.msg == err.msg
	}
	return e.msg == target.Error()
}

type withLayer struct {
	err error
	msg string
}

func (e withLayer) Error() string { return e.msg + ": " + e.err.Error() }
func (e withLayer) Unwrap() error { return e.err }
func (e withLayer) Is(target error) bool {
	if err, ok := target.(withLayer); ok {
		return e.msg == err.msg
	}
	return e.msg == target.Error()
}

type withEmptyLayer struct {
	err error
}

func (e withEmptyLayer) Error() string { return e.err.Error() }
func (e withEmptyLayer) Unwrap() error { return e.err }

func setupTestCase(wrapf bool, cause error, input []string) error {
	err := cause
	for _, str := range input {
		if wrapf {
			err = eris.WithCode(eris.Wrapf(err, "%v", str), eris.CodeUnknown)
		} else {
			err = eris.WithCode(eris.Wrap(err, str), eris.CodeUnknown)
		}
	}
	return err
}

func TestDefaultCodes(t *testing.T) {
	newErr := eris.New("some error").WithCode(eris.CodeUnknown)
	errCode := eris.GetCode(newErr)
	if errCode != eris.CodeUnknown {
		t.Errorf("New errors supposed to default to code 'unknown', but defaulted to %s", errCode)
	}
	wrapErr := eris.WithCode(eris.Wrap(newErr, "wrap err"), eris.CodeInternal)
	errCode = eris.GetCode(wrapErr)
	if errCode != eris.CodeInternal {
		t.Errorf("Wrap errors supposed to default to code 'internal', but defaulted to %s", errCode)
	}
}

func TestKVs(t *testing.T) {
	type KVer interface {
		KVs() map[string]any
	}
	tests := map[string]struct {
		cause KVer           // root error
		kvs   map[string]any // expected output
	}{
		"empty kvs": {
			cause: eris.New("error message"),
			kvs:   map[string]any{},
		},
		"1 kv": {
			cause: eris.New("error message").WithProperty("key1", "val1"),
			kvs:   map[string]any{"key1": "val1"},
		},
		"2 kvs": {
			cause: eris.New("error message").WithProperty("key1", "val1").WithProperty("key2", 2),
			kvs:   map[string]any{"key1": "val1", "key2": 2},
		},
	}
	for desc, tc := range tests {
		t.Run(desc, func(t *testing.T) {
			err := tc.cause
			if err == nil {
				t.Errorf("%v: wrapping nil errors should return nil but got { %v }", desc, err)
			} else if !reflect.DeepEqual(err.KVs(), tc.kvs) {
				t.Errorf("%v: expected { %v } got { %v }", desc, tc.kvs, err.KVs())
			}
		})
	}
}

func TestExternalKVs(t *testing.T) {
	tests := map[string]struct {
		cause error          // root error
		kvs   map[string]any // expected output
	}{
		"nil kvs": {
			cause: fmt.Errorf("external error"),
			kvs:   nil,
		},
		"empty kvs": {
			cause: eris.New("error message"),
			kvs:   map[string]any{},
		},
		"1 kv": {
			cause: eris.New("error message").WithProperty("key1", "val1"),
			kvs:   map[string]any{"key1": "val1"},
		},
		"2 kvs": {
			cause: eris.New("error message").WithProperty("key1", "val1").WithProperty("key2", 2),
			kvs:   map[string]any{"key1": "val1", "key2": 2},
		},
		"wrapped kv": {
			cause: eris.WithProperty(eris.Wrap(fmt.Errorf("external error"), "wrap"), "key1", "val1"),
			kvs:   map[string]any{"key1": "val1"},
		},
	}
	for desc, tc := range tests {
		t.Run(desc, func(t *testing.T) {
			err := tc.cause
			if err == nil {
				t.Errorf("%v: wrapping nil errors should return nil but got { %v }", desc, err)
			} else if !reflect.DeepEqual(eris.GetKVs(err), tc.kvs) {
				t.Errorf("%v: expected { %v } got { %v }", desc, tc.kvs, eris.GetKVs(err))
			}
		})
	}
}

func TestProperty(t *testing.T) {
	tests := map[string]struct {
		cause    error
		key      string
		exist    bool
		property string // expected output
	}{
		"no property": {
			cause: eris.New("error message"),
			key:   "key1",
			exist: false,
		},
		"property not found": {
			cause: eris.New("error message").WithProperty("key2", "val2"),
			key:   "key1",
			exist: false,
		},
		"property type mismatch": {
			cause: eris.New("error message").WithProperty("key1", 1234).WithProperty("key2", "val2"),
			key:   "key1",
			exist: false,
		},
		"error new property found": {
			cause:    eris.New("error message").WithProperty("key1", "val1").WithProperty("key2", 2),
			key:      "key1",
			exist:    true,
			property: "val1",
		},
		"error wrap property found": {
			cause: eris.WithProperty(eris.WithProperty(
				eris.Wrap(fmt.Errorf("external error"), "wrap"),
				"key1", "val1",
			), "key2", 2),
			key:      "key1",
			exist:    true,
			property: "val1",
		},
	}
	for desc, tc := range tests {
		t.Run(desc, func(t *testing.T) {
			v1, ok := eris.GetProperty[string](tc.cause, tc.key)
			v2 := eris.GetPropertyP[string](tc.cause, tc.key)
			if tc.exist {
				if !ok {
					t.Errorf("%v: expected ok { %v } got { %v }", desc, true, ok)
				}
				if v1 != tc.property {
					t.Errorf("%v: expected { %v } got { %v }", desc, tc.property, v1)
				}
				if *v2 != tc.property {
					t.Errorf("%v: expected { %v } got { %v }", desc, tc.property, v2)
				}
			} else {
				if ok {
					t.Errorf("%v: expected ok { %v } got { %v }", desc, false, ok)
				}
				if v2 != nil {
					t.Errorf("%v: expected { %v } got { %v }", desc, nil, v2)
				}
			}
		})
	}
}

func TestErrorWrapping(t *testing.T) {
	tests := map[string]struct {
		cause  error    // root error
		input  []string // input for error wrapping
		output string   // expected output
	}{
		"nil root error": {
			cause: nil,
			input: []string{"additional context"},
		},
		"standard error wrapping with a global root cause": {
			cause:  globalErr,
			input:  []string{"additional context", "even more context"},
			output: "code(unknown) even more context: code(unknown) additional context: code(unknown) global error",
		},
		"formatted error wrapping with a global root cause": {
			cause:  formattedGlobalErr,
			input:  []string{"additional context", "even more context"},
			output: "code(unknown) even more context: code(unknown) additional context: code(unknown) formatted global error",
		},
		"standard error wrapping with a local root cause": {
			cause:  eris.New("root error").WithCode(eris.CodeUnknown),
			input:  []string{"additional context", "even more context"},
			output: "code(unknown) even more context: code(unknown) additional context: code(unknown) root error",
		},
		"standard error wrapping with a local root cause (eris.Errorf)": {
			cause:  eris.Errorf("%v root error", "formatted").WithCode(eris.CodeUnknown),
			input:  []string{"additional context", "even more context"},
			output: "code(unknown) even more context: code(unknown) additional context: code(unknown) formatted root error",
		},
		"no error wrapping with a local root cause (eris.Errorf)": {
			cause:  eris.Errorf("%v root error", "formatted").WithCode(eris.CodeUnknown),
			output: "code(unknown) formatted root error",
		},
	}

	for desc, tc := range tests {
		t.Run(desc, func(t *testing.T) {
			err := setupTestCase(false, tc.cause, tc.input)
			if err != nil && tc.cause == nil {
				t.Errorf("%v: wrapping nil errors should return nil but got { %v }", desc, err)
			} else if err != nil && tc.output != err.Error() {
				t.Errorf("%v: expected { %v } got { %v }", desc, tc.output, err)
			}
		})
	}
}

func TestExternalErrorWrapping(t *testing.T) {
	tests := map[string]struct {
		cause  error    // root error
		input  []string // input for error wrapping
		output []string // expected output
	}{
		"no error wrapping with a third-party root cause (errors.New)": {
			cause: errors.New("external error"),
			output: []string{
				"external error",
			},
		},
		"standard error wrapping with a third-party root cause (errors.New)": {
			cause: errors.New("external error"),
			input: []string{"additional context", "even more context"},
			output: []string{
				"code(unknown) even more context: code(unknown) additional context: external error",
				"code(unknown) additional context: external error",
				"external error",
			},
		},
		// TODO: fix test case
		// "wrapping a wrapped third-party root cause (errors.New and fmt.Errorf)": {
		// 	cause: fmt.Errorf("additional context: %w", errors.New("external error")),
		// 	input: []string{"even more context"},
		// 	output: []string{
		// 		"code(unknown) even more context: code(unknown) additional context: external error",
		// 		"code(unknown) additional context: external error",
		// 		"external error",
		// 	},
		// },
		// TODO: fix test case
		// "wrapping a wrapped third-party root cause (multiple layers)": {
		// 	cause: fmt.Errorf("even more context: %w", fmt.Errorf("additional context: %w", errors.New("external error"))),
		// 	input: []string{"way too much context"},
		// 	output: []string{
		// 		"code(unknown) way too much context: code(unknown) even more context: code(unknown) additional context: external error",
		// 		"code(unknown) even more context: code(unknown) additional context: external error",
		// 		"code(unknown) additional context: external error",
		// 		"external error",
		// 	},
		// },
		// TODO: fix test case
		// 		"wrapping a wrapped third-party root cause that contains an empty layer": {
		// 			cause: fmt.Errorf(": %w", errors.New("external error")),
		// 			input: []string{"even more context"},
		// 			output: []string{
		// 				"code(unknown) even more context: external error",
		// 				"external error",
		// 				"external error",
		// 			},
		// 		},
		"wrapping a wrapped third-party root cause that contains an empty layer without a delimiter": {
			cause: fmt.Errorf("%w", errors.New("external error")),
			input: []string{"even more context"},
			output: []string{
				"code(unknown) even more context: external error",
				"external error",
				"external error",
			},
		},
		// TODO: fix this test case
		//	"wrapping a pkg/errors style error (contains layers without messages)": {
		//		cause: &withLayer{ // var to mimic wrapping a pkg/errors style error
		//			msg: "additional context",
		//			err: &withEmptyLayer{
		//				err: &withMessage{
		//					msg: "external error",
		//				},
		//			},
		//		},
		//		input: []string{"even more context"},
		//		output: []string{
		//			"code(unknown) even more context: code(unknown) additional context: external error",
		//			"code(unknown) additional context: external error",
		//			"external error",
		//			"external error",
		//		},
		//	},
		"implicate wrap when add field to external error": {
			cause: eris.With(
				errors.New("external error"),
				eris.Codes(eris.CodeCanceled), eris.KVs("key", "value"),
			),
			input: []string{"even more context"},
			output: []string{
				"code(unknown) even more context: code(canceled) KVs(map[key:value]) with property: external error",
				"code(canceled) KVs(map[key:value]) with property: external error",
				"external error",
			},
		},
	}

	for desc, tc := range tests {
		t.Run(desc, func(t *testing.T) {
			err := setupTestCase(false, tc.cause, tc.input)

			// unwrap to make sure external errors are actually wrapped properly
			var inputErr []string
			for err != nil {
				inputErr = append(inputErr, err.Error())
				err = eris.Unwrap(err)
			}

			// compare each layer of the actual and expected output
			if len(inputErr) != len(tc.output) {
				t.Fatalf("%v: expected output to have '%v' layers but got '%v': { %#v } got { %#v }", desc, len(tc.output), len(inputErr), tc.output, inputErr)
			}
			for i := 0; i < len(inputErr); i++ {
				if inputErr[i] != tc.output[i] {
					t.Errorf("%v: got { %#v } expected { %#v }", desc, inputErr[i], tc.output[i])
				}
			}
		})
	}
}

func TestErrorPassThrough(t *testing.T) {
	tests := map[string]struct {
		cause  error    // root error
		input  []string // input for error wrapping
		output string   // expected output
	}{
		"nil root error": {
			cause: nil,
			input: []string{"additional context"},
		},
		"external error passing": {
			cause:  fmt.Errorf("external error"),
			input:  []string{"additional context"},
			output: "code(internal) additional context: external error",
		},
		"standard error passing with a global root cause": {
			cause:  globalErr,
			input:  []string{"additional context", "even more context"},
			output: "code(internal) even more context: code(internal) additional context: code(unknown) global error",
		},
		"standard error passing with a local root cause": {
			cause:  eris.New("root error").WithCode(eris.CodeInvalidArgument),
			input:  []string{"additional context", "even more context"},
			output: "code(invalid argument) even more context: code(invalid argument) additional context: code(invalid argument) root error",
		},
		"standard error passing with a local root cause (eris.Errorf)": {
			cause:  eris.Errorf("%v root error", "formatted").WithCode(eris.CodeInvalidArgument),
			input:  []string{"additional context", "even more context"},
			output: "code(invalid argument) even more context: code(invalid argument) additional context: code(invalid argument) formatted root error",
		},
		"standard error passing with a local root cause property": {
			cause:  eris.New("root error").WithProperty("key1", "val1"),
			input:  []string{"additional context", "even more context"},
			output: "code(internal) KVs(map[key1:val1]) even more context: code(internal) KVs(map[key1:val1]) additional context: code(unknown) KVs(map[key1:val1]) root error",
		},
		"standard error passing with a local root cause property (eris.Errorf)": {
			cause:  eris.Errorf("%v root error", "formatted").WithProperty("key1", "val1"),
			input:  []string{"additional context", "even more context"},
			output: "code(internal) KVs(map[key1:val1]) even more context: code(internal) KVs(map[key1:val1]) additional context: code(unknown) KVs(map[key1:val1]) formatted root error",
		},
		"no error passing with a local root cause (eris.Errorf)": {
			cause:  eris.Errorf("%v root error", "formatted").WithCode(eris.CodeUnknown),
			output: "code(unknown) formatted root error",
		},
	}

	for desc, tc := range tests {
		t.Run(desc, func(t *testing.T) {
			err := tc.cause
			for _, str := range tc.input {
				err = eris.PassThrough(err, str)
			}
			if err != nil && tc.cause == nil {
				t.Errorf("%v: wrapping nil errors should return nil but got { %v }", desc, err)
			} else if err != nil && tc.output != err.Error() {
				t.Errorf("%v: expected { %v } got { %v }", desc, tc.output, err)
			}
		})
	}
}

func TestErrorUnwrap(t *testing.T) {
	tests := map[string]struct {
		cause  error    // root error
		input  []string // input for error wrapping
		output []string // expected output
	}{
		"unwrapping error with internal root cause (eris.New)": {
			cause: eris.New("root error").WithCode(eris.CodeUnknown),
			input: []string{"additional context", "even more context"},
			output: []string{
				"code(unknown) even more context: code(unknown) additional context: code(unknown) root error",
				"code(unknown) additional context: code(unknown) root error",
				"code(unknown) root error",
			},
		},
		"unwrapping error with external root cause (errors.New)": {
			cause: errors.New("external error"),
			input: []string{"additional context", "even more context"},
			output: []string{
				"code(unknown) even more context: code(unknown) additional context: external error",
				"code(unknown) additional context: external error",
				"external error",
			},
		},
		"unwrapping error with external root cause (custom type)": {
			cause: &withMessage{
				msg: "external error",
			},
			input: []string{"additional context", "even more context"},
			output: []string{
				"code(unknown) even more context: code(unknown) additional context: external error",
				"code(unknown) additional context: external error",
				"external error",
			},
		},
	}

	for desc, tc := range tests {
		t.Run(desc, func(t *testing.T) {
			err := setupTestCase(true, tc.cause, tc.input)
			for _, out := range tc.output {
				if err == nil {
					t.Errorf("%v: unwrapping error returned nil but expected { %v }", desc, out)
				} else if out != err.Error() {
					t.Errorf("%v: expected { %v } got { %v }", desc, out, err)
				}
				err = eris.Unwrap(err)
			}
		})
	}
}

func TestErrorIs(t *testing.T) {
	rootErr := eris.New("root error")
	externalErr := errors.New("external error")
	customErr := withLayer{
		msg: "additional context",
		err: withEmptyLayer{
			err: withMessage{
				msg: "external error",
			},
		},
	}

	tests := map[string]struct {
		cause   error    // root error
		input   []string // input for error wrapping
		compare error    // errors for comparison
		output  bool     // expected comparison result
	}{
		"root error (internal)": {
			cause:   eris.New("root error").WithCode(eris.CodeUnknown),
			input:   []string{"additional context", "even more context"},
			compare: eris.New("root error").WithCode(eris.CodeUnknown),
			output:  true,
		},
		"error not in chain": {
			cause:   eris.New("root error").WithCode(eris.CodeUnknown),
			compare: eris.New("other error").WithCode(eris.CodeUnknown),
			output:  false,
		},
		"middle of chain (internal)": {
			cause:   eris.New("root error").WithCode(eris.CodeUnknown),
			input:   []string{"additional context", "even more context"},
			compare: eris.New("additional context").WithCode(eris.CodeUnknown),
			output:  true,
		},
		"another in middle of chain (internal)": {
			cause:   eris.New("root error").WithCode(eris.CodeUnknown),
			input:   []string{"additional context", "even more context"},
			compare: eris.New("even more context").WithCode(eris.CodeUnknown),
			output:  true,
		},
		"root error (external)": {
			cause:   externalErr,
			input:   []string{"additional context", "even more context"},
			compare: externalErr,
			output:  true,
		},
		"wrapped error from global root error": {
			cause:   globalErr,
			input:   []string{"additional context", "even more context"},
			compare: eris.WithCode(eris.Wrap(globalErr, "additional context"), eris.CodeUnknown),
			output:  true,
		},
		"comparing against external error": {
			cause:   externalErr,
			input:   []string{"additional context", "even more context"},
			compare: externalErr,
			output:  true,
		},
		"comparing against different external error": {
			cause:   errors.New("external error 1"),
			compare: errors.New("external error 2"),
			output:  false,
		},
		"comparing against custom error type": {
			cause:   customErr,
			input:   []string{"even more context"},
			compare: customErr,
			output:  true,
		},
		"comparing against custom error type (copied error)": {
			cause: customErr,
			input: []string{"even more context"},
			compare: &withMessage{
				msg: "external error",
			},
			output: true,
		},
		"comparing against nil error": {
			cause:   eris.New("root error").WithCode(eris.CodeUnknown),
			compare: nil,
			output:  false,
		},
		"comparing error against itself": {
			cause:   globalErr,
			compare: globalErr,
			output:  true,
		},
		"comparing two nil errors": {
			cause:   nil,
			compare: nil,
			output:  true,
		},
		"join error (external)": {
			cause:   eris.Join(externalErr, rootErr),
			compare: externalErr,
			output:  true,
		},
		"join error (root)": {
			cause:   eris.Join(externalErr, rootErr),
			compare: rootErr,
			output:  true,
		},
		"join error (nil)": {
			cause:   eris.Join(nil, nil),
			compare: nil,
			output:  true,
		},
		"join error (wrap)": {
			cause:   eris.Join(externalErr, eris.Wrap(rootErr, "eris wrap error")),
			compare: eris.New("eris wrap error").WithCode(eris.CodeInternal),
			output:  true,
		},
		"join error not found (code don't match)": {
			cause:   eris.Join(externalErr, eris.Wrap(rootErr, "eris wrap error")),
			compare: eris.New("eris wrap error"),
			output:  false,
		},
		"join error not found (message don't match)": {
			cause:   eris.Join(externalErr, eris.Wrap(rootErr, "eris wrap error")),
			compare: eris.New("eris root error message wrong"),
			output:  false,
		},
		"join error not found (external don't match)": {
			cause:   eris.Join(externalErr, rootErr),
			compare: errors.New("external error not match"),
			output:  false,
		},
		"wrapped join error (match join errors)": {
			cause:   eris.Join(externalErr, rootErr),
			input:   []string{"additional context"},
			compare: rootErr,
			output:  true,
		},
		"wrapped join error (match wrap)": {
			cause:   eris.Join(externalErr, rootErr),
			input:   []string{"additional context"},
			compare: eris.New("additional context").WithCode(eris.CodeUnknown),
			output:  true,
		},
	}

	for desc, tc := range tests {
		t.Run(desc, func(t *testing.T) {
			err := setupTestCase(false, tc.cause, tc.input)
			if tc.output && !eris.Is(err, tc.compare) {
				t.Errorf("%v: expected eris.Is('%v', '%v') to return true but got false", desc, err, tc.compare)
			} else if !tc.output && eris.Is(err, tc.compare) {
				t.Errorf("%v: expected eris.Is('%v', '%v') to return false but got true", desc, err, tc.compare)
			}
		})
	}
}

func TestErrorAs(t *testing.T) {
	externalError := errors.New("external error")
	rootErr := eris.New("root error").WithCode(eris.CodeUnknown)
	anotherRootErr := eris.New("another root error").WithCode(eris.CodeUnknown)
	wrappedErr := eris.WithCode(eris.Wrap(rootErr, "additional context"), eris.CodeUnknown)
	customErr := withLayer{
		msg: "additional context",
		err: withEmptyLayer{
			err: withMessage{
				msg: "external error",
			},
		},
	}
	tests := map[string]struct {
		cause  error // root error
		target any   // errors for comparison
		match  bool  // expected comparison result
		output any   // value of target on match
	}{
		"nil error against nil target": {
			cause:  nil,
			target: nil,
			match:  false,
			output: nil,
		},
		"error against nil target": {
			cause:  rootErr,
			target: nil,
			match:  false,
			output: nil,
		},
		"error against non pointer interface": {
			cause:  rootErr,
			target: rootErr,
			match:  false,
			output: nil,
		},
		"error against non error interface": {
			cause:  rootErr,
			target: &withMessage{"test"},
			match:  false,
			output: nil,
		},
		"error against non pointer": {
			cause:  rootErr,
			target: "test",
			match:  false,
			output: nil,
		},
		"nil error against external target": {
			cause:  nil,
			target: &externalError,
			match:  false,
			output: nil,
		},
		"root error against external target": {
			cause:  eris.New("root error").WithCode(eris.CodeUnknown),
			target: &externalError,
			match:  false,
			output: nil,
		},
		"wrapped error against external target": {
			cause:  wrappedErr,
			target: &externalError,
			match:  false,
			output: nil,
		},
		"nil wrapped error against root error target": {
			cause:  eris.WithCode(eris.Wrap(nil, "additional context"), eris.CodeUnknown),
			target: &rootErr,
			match:  false,
			output: nil,
		},
		"wrapped error against its own root error target": {
			cause:  wrappedErr,
			target: &rootErr,
			match:  true,
			output: rootErr,
		},
		"root error against itself": {
			cause:  rootErr,
			target: &rootErr,
			match:  true,
			output: rootErr,
		},
		"root error against different root error": {
			cause:  eris.New("other root error").WithCode(eris.CodeUnknown),
			target: &rootErr,
			match:  false,
			output: nil,
		},
		"wrapped error against same wrapped error": {
			cause:  wrappedErr,
			target: &wrappedErr,
			match:  true,
			output: wrappedErr,
		},
		"wrapped error against different wrapped error": {
			cause:  eris.WithCode(eris.Wrap(nil, "some other error"), eris.CodeUnknown),
			target: &wrappedErr,
			match:  false,
			output: nil,
		},
		"custom error against similar type error": {
			cause:  customErr,
			target: &withLayer{msg: "additional layer", err: externalError},
			match:  true,
			output: customErr,
		},
		"join error (external)": {
			cause:  eris.Join(externalError, rootErr),
			target: &externalError,
			match:  true,
			output: externalError,
		},
		"join error (root)": {
			cause:  eris.Join(externalError, rootErr),
			target: &rootErr,
			match:  true,
			output: rootErr,
		},
		"join error (custom)": {
			cause:  eris.Join(externalError, withMessage{"test"}),
			target: &withMessage{""},
			match:  true,
			output: withMessage{"test"},
		},
		"join error not found (message don't match)": {
			cause:  eris.Join(externalError, rootErr),
			target: &anotherRootErr,
			match:  false,
		},
	}

	for desc, tc := range tests {
		rtarget := reflect.ValueOf(tc.target)
		t.Run(desc, func(t *testing.T) {
			match := eris.As(tc.cause, tc.target)
			if tc.match != match {
				t.Fatalf("%v: expected eris.As('%v', '%v') to return %v but got %v", desc, tc.cause, reflect.ValueOf(tc.target).Elem(), tc.match, match)
			}
			if !match {
				return
			}
			if got := rtarget.Elem().Interface(); got != tc.output {
				t.Fatalf("%v: expected eris.As('%v', '%v') target interface value to be %#v, but got %#v", desc, tc.cause, reflect.ValueOf(tc.target).Elem(), tc.output, got)
			}
		})
	}
}

func TestErrorCause(t *testing.T) {
	globalErr := eris.New("global error").WithCode(eris.CodeUnknown)
	extErr := errors.New("external error")
	customErr := withMessage{
		msg: "external error",
	}

	tests := map[string]struct {
		cause  error    // root error
		input  []string // input for error wrapping
		output error    // expected output
	}{
		"internal root error": {
			cause:  globalErr,
			input:  []string{"additional context", "even more context"},
			output: globalErr,
		},
		"external error": {
			cause:  extErr,
			input:  []string{"additional context", "even more context"},
			output: extErr,
		},
		"external error (custom type)": {
			cause:  customErr,
			input:  []string{"additional context", "even more context"},
			output: customErr,
		},
		"nil error": {
			cause:  nil,
			output: nil,
		},
	}

	for desc, tc := range tests {
		t.Run(desc, func(t *testing.T) {
			err := setupTestCase(false, tc.cause, tc.input)
			cause := eris.Cause(err)
			if tc.output != eris.Cause(err) {
				t.Errorf("%v: expected { %v } got { %v }", desc, tc.output, cause)
			}
		})
	}
}

func TestExternalErrorAs(t *testing.T) {
	cause := withMessage{
		msg: "external error",
	}
	empty := withEmptyLayer{
		err: cause,
	}
	layer := withLayer{
		msg: "additional context",
		err: empty,
	}

	tests := map[string]struct {
		cause   error    // root error
		input   []string // input for error wrapping
		results []bool   // comparison results
		targets []error  // output results
	}{
		"external cause": {
			cause:   cause,
			input:   []string{"even more context"},
			results: []bool{true, false, false},
			targets: []error{cause, nil, nil},
		},
		"external error with empty layer": {
			cause:   empty,
			input:   []string{"even more context"},
			results: []bool{true, true, false},
			targets: []error{cause, empty, nil},
		},
		"external error with multiple layers": {
			cause:   layer,
			input:   []string{"even more context"},
			results: []bool{true, true, true},
			targets: []error{cause, empty, layer},
		},
	}

	for desc, tc := range tests {
		t.Run(desc, func(t *testing.T) {
			err := setupTestCase(false, tc.cause, tc.input)

			msgTarget := withMessage{}
			msgResult := errors.As(err, &msgTarget)
			if tc.results[0] != msgResult {
				t.Errorf("%v: expected errors.As('%v', &withMessage{}) to return {'%v', '%v'} but got {'%v', '%v'}",
					desc, err, tc.results[0], tc.targets[0], msgResult, msgTarget)
			} else if msgResult == true && tc.targets[0] != msgTarget {
				t.Errorf("%v: expected errors.As('%v', &withMessage{}) to return {'%v', '%v'} but got {'%v', '%v'}",
					desc, err, tc.results[0], tc.targets[0], msgResult, msgTarget)
			}

			emptyTarget := withEmptyLayer{}
			emptyResult := errors.As(err, &emptyTarget)
			if tc.results[1] != emptyResult {
				t.Errorf("%v: expected errors.As('%v', &withEmptyLayer{}) to return {'%v', '%v'} but got {'%v', '%v'}",
					desc, err, tc.results[1], tc.targets[1], emptyResult, emptyTarget)
			} else if emptyResult == true && tc.targets[1] != emptyTarget {
				t.Errorf("%v: expected errors.As('%v', &withEmptyLayer{}) to return {'%v', '%v'} but got {'%v', '%v'}",
					desc, err, tc.results[1], tc.targets[1], emptyResult, emptyTarget)
			}

			layerTarget := withLayer{}
			layerResult := errors.As(err, &layerTarget)
			if tc.results[2] != layerResult {
				t.Errorf("%v: expected errors.As('%v', &withLayer{}) to return {'%v', '%v'} but got {'%v', '%v'}",
					desc, err, tc.results[2], tc.targets[2], layerResult, layerTarget)
			} else if layerResult == true && tc.targets[2] != layerTarget {
				t.Errorf("%v: expected errors.As('%v', &withLayer{}) to return {'%v', '%v'} but got {'%v', '%v'}",
					desc, err, tc.results[2], tc.targets[2], layerResult, layerTarget)
			}
		})
	}
}

type CustomErr struct{}

func (CustomErr) Error() string {
	return "custom error"
}

func TestCustomErrorAs(t *testing.T) {
	original := CustomErr{}
	wrap1 := eris.WithCode(eris.Wrap(original, "wrap1"), eris.CodeUnknown)
	wrap2 := eris.WithCode(eris.Wrap(wrap1, "wrap2"), eris.CodeUnknown)

	var customErr CustomErr
	if !eris.As(wrap1, &customErr) {
		t.Errorf("expected errors.As(wrap1, &customErr) to return true but got false")
	}
	if !eris.As(wrap2, &customErr) {
		t.Errorf("expected errors.As(wrap2, &customErr) to return true but got false")
	}
}

func TestErrorFormatting(t *testing.T) {
	tests := map[string]struct {
		cause  error    // root error
		input  []string // input for error wrapping
		output string   // expected output
	}{
		"standard error wrapping with internal root cause (eris.New)": {
			cause:  eris.New("root error").WithCode(eris.CodeUnknown),
			input:  []string{"additional context", "even more context"},
			output: "code(unknown) even more context: code(unknown) additional context: code(unknown) root error",
		},
		"standard error wrapping with external root cause (errors.New)": {
			cause:  errors.New("external error"),
			input:  []string{"additional context", "even more context"},
			output: "code(unknown) even more context: code(unknown) additional context: external error",
		},
	}

	for desc, tc := range tests {
		t.Run(desc, func(t *testing.T) {
			err := setupTestCase(false, tc.cause, tc.input)
			if err != nil && tc.cause == nil {
				t.Errorf("%v: wrapping nil errors should return nil but got { %v }", desc, err)
			} else if err != nil && tc.output != err.Error() {
				t.Errorf("%v: expected { %v } got { %v }", desc, tc.output, err)
			}

			_ = fmt.Sprintf("error formatting results (%v):\n", desc)
			_ = fmt.Sprintf("%v\n", err)
			_ = fmt.Sprintf("%+v", err)
		})
	}
}

func getFrames(pc []uintptr) []eris.StackFrame {
	var stackFrames []eris.StackFrame
	if len(pc) == 0 {
		return stackFrames
	}

	frames := runtime.CallersFrames(pc)
	for {
		frame, more := frames.Next()
		i := strings.LastIndex(frame.Function, "/")
		name := frame.Function[i+1:]
		stackFrames = append(stackFrames, eris.StackFrame{
			Name: name,
			File: frame.File,
			Line: frame.Line,
		})
		if !more {
			break
		}
	}

	return stackFrames
}

func getFrameFromLink(link eris.ErrLink) eris.Stack {
	var stackFrames []eris.StackFrame
	stackFrames = append(stackFrames, link.Frame)
	return eris.Stack(stackFrames)
}

func TestStackFrames(t *testing.T) {
	tests := map[string]struct {
		cause     error    // root error
		input     []string // input for error wrapping
		isWrapErr bool     // flag for wrap error
	}{
		"root error": {
			cause:     eris.New("root error").WithCode(eris.CodeUnknown),
			isWrapErr: false,
		},
		"wrapped error": {
			cause:     eris.New("root error").WithCode(eris.CodeUnknown),
			input:     []string{"additional context", "even more context"},
			isWrapErr: true,
		},
		"external error": {
			cause:     errors.New("external error"),
			isWrapErr: false,
		},
		"wrapped external error": {
			cause:     errors.New("external error"),
			input:     []string{"additional context", "even more context"},
			isWrapErr: true,
		},
		"global root error": {
			cause:     globalErr,
			isWrapErr: false,
		},
		"wrapped error from global root error": {
			cause:     globalErr,
			input:     []string{"additional context", "even more context"},
			isWrapErr: true,
		},
		"nil error": {
			cause:     nil,
			isWrapErr: false,
		},
	}

	for desc, tc := range tests {
		t.Run(desc, func(t *testing.T) {
			err := setupTestCase(false, tc.cause, tc.input)
			uErr := eris.Unpack(err)
			sFrames := eris.Stack(getFrames(eris.StackFrames(err)))
			if !tc.isWrapErr && !reflect.DeepEqual(uErr.ErrRoot.Stack, sFrames) {
				t.Errorf("%v: expected { %v } got { %v }", desc, uErr.ErrRoot.Stack, sFrames)
			}
			if tc.isWrapErr && !reflect.DeepEqual(getFrameFromLink(uErr.ErrChain[0]), sFrames) {
				t.Errorf("%v: expected { %v } got { %v }", desc, getFrameFromLink(uErr.ErrChain[0]), sFrames)
			}
		})
	}
}

// TODO fix this test
//func TestOkCode(t *testing.T) {
//	err := eris.New("everything went fine").WithCodeGrpc(grpc.OK)
//	if err != nil {
//		t.Errorf("expected nil error if grpc status is OK, but error was %v", err)
//	}
//
//	err = eris.New("everything went fine again").WithCodeHttp(http.StatusOK)
//	if err != nil {
//		t.Errorf("expected nil error if grpc status is OK, but error was %v", err)
//	}
//}

func TestWrapType(t *testing.T) {
	var err error = nil
	var erisErr = eris.Wrapf(err, "test error")

	if erisErr != nil {
		t.Errorf("expected nil error if wrap nil error, but error was %v", erisErr)
	}
}

func TestJoinError(t *testing.T) {
	err := eris.Join(nil, nil)
	if err != nil {
		t.Errorf("join nil should be nil")
	}
	err = eris.Join(nil, fmt.Errorf("external error"))
	if err == nil {
		t.Errorf("join error should be error")
	}
	err = eris.Join(fmt.Errorf("err1"), nil, fmt.Errorf("err2"))
	if err == nil {
		t.Errorf("join errors should be error")
	}
	type joinError interface {
		Unwrap() []error
	}
	if joinErr, ok := eris.Unwrap(err).(joinError); !ok {
		if len(joinErr.Unwrap()) != 2 {
			t.Errorf("join 2 errors should be 2 errors")
		}
	}
}
