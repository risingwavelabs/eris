package eris

import (
	"fmt"
	"strings"
)

// FormatOptions defines output options like omitting stack traces and inverting the error or stack order.
type FormatOptions struct {
	InvertOutput bool // Flag that inverts the error output (wrap errors shown first).
	WithTrace    bool // Flag that enables stack trace output.
	InvertTrace  bool // Flag that inverts the stack trace output (top of call stack shown first).
	WithExternal bool // Flag that enables external error output.
	// todo: maybe allow users to hide wrap frames if desired
}

// StringFormat defines a string error format.
type StringFormat struct {
	Options      FormatOptions // Format options (e.g. omitting stack trace or inverting the output order).
	MsgStackSep  string        // Separator between error messages and stack frame data.
	PreStackSep  string        // Separator at the beginning of each stack frame.
	StackElemSep string        // Separator between elements of each stack frame.
	ErrorSep     string        // Separator between each error in the chain.
}

// NewDefaultStringFormat returns a default string output format.
func NewDefaultStringFormat(options FormatOptions) StringFormat {
	stringFmt := StringFormat{
		Options: options,
	}
	if options.WithTrace {
		stringFmt.MsgStackSep = "\n"
		stringFmt.PreStackSep = "\t"
		stringFmt.StackElemSep = ":"
		stringFmt.ErrorSep = "\n"
	} else {
		stringFmt.ErrorSep = ": "
	}
	return stringFmt
}

// ToString returns a default formatted string for a given error.
//
// An error without trace will be formatted as follows:
//
//	<Wrap error msg>: <Root error msg>
//
// An error with trace will be formatted as follows:
//
//	<Wrap error msg>
//	  <Method2>:<File2>:<Line2>
//	<Root error msg>
//	  <Method2>:<File2>:<Line2>
//	  <Method1>:<File1>:<Line1>
//
// Example:
//
//	rootCause := errors.New("external error")
//	caller1 := eris.Wrap(rootCause, "no good").WithCode(eris.CodeDataLoss).WithProperty("foo", true).WithProperty("bar", 42)
//	caller2 := eris.Wrap(caller1, "even more context")
//
// Without trace:
//
//	code(internal) even more context: code(data loss) KVs(map[bar:42 foo:true]) additional context: external error
func ToString(err error, withTrace bool) string {
	return ToCustomString(err, NewDefaultStringFormat(FormatOptions{
		WithTrace:    withTrace,
		WithExternal: true,
	}))
}

// ToCustomString returns a custom formatted string for a given error.
//
// To declare custom format, the Format object has to be passed as an argument.
// An error without trace will be formatted as follows:
//
//	<Wrap error msg>[Format.ErrorSep]<Root error msg>
//
// An error with trace will be formatted as follows:
//
//	<Wrap error msg>[Format.MsgStackSep]
//	[Format.PreStackSep]<Method2>[Format.StackElemSep]<File2>[Format.StackElemSep]<Line2>[Format.ErrorSep]
//	<Root error msg>[Format.MsgStackSep]
//	[Format.PreStackSep]<Method2>[Format.StackElemSep]<File2>[Format.StackElemSep]<Line2>[Format.ErrorSep]
//	[Format.PreStackSep]<Method1>[Format.StackElemSep]<File1>[Format.StackElemSep]<Line1>[Format.ErrorSep]
//
// Example:
//
//	rootCause := errors.New("external error")
//	caller1 := eris.Wrap(rootCause, "no good").WithCode(eris.CodeDataLoss).WithProperty("foo", true).WithProperty("bar", 42)
//	caller2 := eris.Wrap(caller1, "even more context")
//
// Formatted with
//
//	eris.NewDefaultStringFormat(eris.FormatOptions{WithExternal: true, InvertOutput: true})
//
// will result in
//
//	code(internal) even more context: code(data loss) KVs(map[bar:42 foo:true]) additional context: external error
func ToCustomString(err error, format StringFormat) string {
	upErr := Unpack(err)

	var str string
	if format.Options.InvertOutput {
		errSep := false
		if format.Options.WithExternal && upErr.ErrExternal != nil {
			externalStr := formatExternalStr(upErr.ErrExternal, format.Options.WithTrace)
			str += externalStr
			if strings.Contains(externalStr, "\n") {
				str += "\n"
			} else if (format.Options.WithTrace && len(upErr.ErrRoot.Stack) > 0) || upErr.ErrRoot.Msg != "" {
				errSep = true
				str += format.ErrorSep
			}
		}
		rootErrStr := upErr.ErrRoot.formatStr(format)
		space := ""
		if !errSep && len(rootErrStr) > 0 && len(upErr.ErrChain) == 0 {
			space = " "
		}
		str += space + rootErrStr
		for _, eLink := range upErr.ErrChain {
			str += format.ErrorSep + eLink.formatStr(format)
		}
	} else {
		for i := len(upErr.ErrChain) - 1; i >= 0; i-- {
			str += upErr.ErrChain[i].formatStr(format) + format.ErrorSep
		}
		str += upErr.ErrRoot.formatStr(format)
		if format.Options.WithExternal && upErr.ErrExternal != nil {
			externalStr := formatExternalStr(upErr.ErrExternal, format.Options.WithTrace)
			if strings.Contains(externalStr, "\n") {
				str += "\n"
			} else if (format.Options.WithTrace && len(upErr.ErrRoot.Stack) > 0) || upErr.ErrRoot.Msg != "" {
				str += format.ErrorSep
			}
			str += externalStr
		}
	}

	return str
}

// JSONFormat defines a JSON error format.
type JSONFormat struct {
	Options FormatOptions // Format options (e.g. omitting stack trace or inverting the output order).
	// todo: maybe allow setting of wrap/root keys in the output map as well
	StackElemSep string // Separator between elements of each stack frame.
}

// NewDefaultJSONFormat returns a default JSON output format.
func NewDefaultJSONFormat(options FormatOptions) JSONFormat {
	return JSONFormat{
		Options:      options,
		StackElemSep: ":",
	}
}

// ToJSON returns a JSON formatted map for a given error.
//
// Example error:
//
//	rootCause := errors.New("external error")
//	caller1 := eris.Wrap(rootCause, "no good").WithCode(eris.CodeDataLoss).WithProperty("foo", true).WithProperty("bar", 42)
//	caller2 := eris.Wrap(caller1, "even more context")
//
// The example error above without trace will be formatted as follows:
//
//	{
//	    "external": "external error",
//	    "root": {
//	        "KVs": {
//	            "bar": 42,
//	            "foo": true
//	        },
//	        "code": "data loss",
//	        "message": "no good"
//	    },
//	    "wrap": [
//	        {
//	            "code": "internal",
//	            "message": "even more context"
//	        }
//	    ]
//	}
//
// The example error above with trace will be formatted as follows:
//
//	{
//	    "external": "external error",
//	    "root": {
//	        "KVs": {
//	            "bar": 42,
//	            "foo": true
//	        },
//	        "code": "data loss",
//	        "message": "additional context",
//	        "stack": [
//	            "<Method1>:<File1>:<Line1>",
//	            "<Method2>:<File2>:<Line2>"
//	        ]
//	    },
//	    "wrap": [
//	        {
//	            "code": "internal",
//	            "message": "even more context",
//	            "stack": "<Method3>:<File3>:<Line3>"
//	        }
//	    ]
//	}
func ToJSON(err error, withTrace bool) map[string]any {
	return ToCustomJSON(err, NewDefaultJSONFormat(FormatOptions{
		WithTrace:    withTrace,
		WithExternal: true,
	}))
}

// ToCustomJSON returns a JSON formatted map for a given error.
//
// To declare custom format, the Format object has to be passed as an argument.
// An error without trace will be formatted as follows:
//
//	{
//	  "root": {
//	    "code": "unknown",
//	    "message": "Root error msg",
//	  },
//	  "wrap": [
//	    {
//	      "message": "Wrap error msg'",
//	      "code": "internal",
//	    }
//	  ]
//	}
//
// An error with trace will be formatted as follows:
//
//	{
//	  "root": {
//	    "code": "unknown",
//	    "message": "Root error msg",
//	    "stack": [
//	      "<Method2>[Format.StackElemSep]<File2>[Format.StackElemSep]<Line2>",
//	      "<Method1>[Format.StackElemSep]<File1>[Format.StackElemSep]<Line1>"
//	    ]
//	  }
//	  "wrap": [
//	    {
//	      "code": "internal",
//	      "message": "Wrap error msg",
//	      "stack": "<Method2>[Format.StackElemSep]<File2>[Format.StackElemSep]<Line2>"
//	    }
//	  ]
//	}
func ToCustomJSON(err error, format JSONFormat) map[string]any {
	upErr := Unpack(err)

	jsonMap := make(map[string]any)
	if format.Options.WithExternal && upErr.ErrExternal != nil {

		join, ok := upErr.ErrExternal.(joinError)
		if !ok {
			jsonMap["external"] = formatExternalStr(upErr.ErrExternal, format.Options.WithTrace)
		} else {
			var externals []map[string]any
			for _, e := range join.Unwrap() {
				externals = append(externals, ToCustomJSON(e, format))
			}
			jsonMap["externals"] = externals
		}
	}

	if upErr.ErrRoot.Msg != "" || len(upErr.ErrRoot.Stack) > 0 {
		jsonMap["root"] = upErr.ErrRoot.formatJSON(format)
	}
	if len(upErr.ErrChain) > 0 {
		var wrapArr []map[string]any
		for _, eLink := range upErr.ErrChain {
			wrapMap := eLink.formatJSON(format)
			if format.Options.InvertOutput {
				wrapArr = append(wrapArr, wrapMap)
			} else {
				wrapArr = append([]map[string]any{wrapMap}, wrapArr...)
			}
		}
		jsonMap["wrap"] = wrapArr
	}

	return jsonMap
}

// Unpack returns a human-readable UnpackedError type for a given error.
func Unpack(err error) UnpackedError {
	var upErr UnpackedError
	for err != nil {
		switch err := err.(type) {
		case *rootError:
			upErr.ErrRoot.Msg = err.msg
			upErr.ErrRoot.Stack = err.stack.get()
			upErr.ErrRoot.code = err.code
			upErr.ErrRoot.kvs = err.kvs
		case *wrapError:
			// prepend links in stack trace order
			link := ErrLink{Msg: err.msg}
			link.Frame = err.frame.get()
			link.code = err.code
			link.kvs = err.kvs
			upErr.ErrChain = append([]ErrLink{link}, upErr.ErrChain...)
		default:
			upErr.ErrExternal = err
			return upErr
		}
		err = Unwrap(err)
	}
	return upErr
}

// UnpackedError represents complete information about an error.
//
// This type can be used for custom error logging and parsing. Use `eris.Unpack` to build an UnpackedError
// from any error type. The ErrChain and ErrRoot fields correspond to `wrapError` and `rootError` types,
// respectively. If any other error type is unpacked, it will appear in the ExternalErr field.
type UnpackedError struct {
	ErrExternal error
	ErrRoot     ErrRoot
	ErrChain    []ErrLink
}

// String formatter for external errors.
func formatExternalStr(err error, withTrace bool) string {
	type joinError interface {
		Unwrap() []error
	}

	format := "%v"
	if withTrace {
		format = "%+v"
	}
	join, ok := err.(joinError)
	if !ok {
		return fmt.Sprintf(format, err)
	}

	var strs []string
	for i, e := range join.Unwrap() {
		lines := strings.Split(fmt.Sprintf(format, e), "\n")
		for no, line := range lines {
			lines[no] = fmt.Sprintf("\t%s", line)
		}
		strs = append(strs, fmt.Sprintf("%d>", i)+strings.Join(lines, "\n"))
	}
	return strings.Join(strs, "\n")
}

// ErrRoot represents an error stack and the accompanying message.
type ErrRoot struct {
	Msg   string
	Stack Stack
	code  Code
	kvs   map[string]any
}

// Code returns the error code.
func (err *ErrRoot) Code() Code {
	return err.code
}

// HasKVs returns true if the error has key-value pairs.
func (err *ErrRoot) HasKVs() bool {
	return err.kvs != nil && len(err.kvs) > 0
}

// String formatter for root errors.
func (err *ErrRoot) formatStr(format StringFormat) string {

	kvs := ""
	if len(err.kvs) > 0 {
		kvs = fmt.Sprintf(" KVs(%v)", err.kvs)
	}

	// Do not print default errors
	if kvs == "" && err.code == DEFAULT_ERROR_CODE_NEW && err.Msg == "" {
		return ""
	}

	str := fmt.Sprintf("code(%s)%s %s%s", err.code.String(), kvs, err.Msg, format.MsgStackSep)
	if format.Options.WithTrace {
		stackArr := err.Stack.format(format.StackElemSep, format.Options.InvertTrace)
		for i, frame := range stackArr {
			str += format.PreStackSep + frame
			if i < len(stackArr)-1 {
				str += format.ErrorSep
			}
		}
	}
	return str
}

// JSON formatter for root errors.
func (err *ErrRoot) formatJSON(format JSONFormat) map[string]any {
	rootMap := make(map[string]any)
	rootMap["code"] = err.code.String()
	rootMap["message"] = err.Msg
	if err.HasKVs() {
		rootMap["KVs"] = err.kvs // TODO: debugging notes we lost the object at this point
	}
	if format.Options.WithTrace {
		rootMap["stack"] = err.Stack.format(format.StackElemSep, format.Options.InvertTrace)
	}
	return rootMap
}

// ErrLink represents a single error frame and the accompanying information.
type ErrLink struct {
	Msg   string
	Frame StackFrame
	code  Code
	kvs   map[string]any
}

// Code returns the error code.
func (eLink *ErrLink) Code() Code {
	return eLink.code
}

// HasKVs returns true if the error has key-value pairs.
func (eLink *ErrLink) HasKVs() bool {
	return eLink.kvs != nil && len(eLink.kvs) > 0
}

// String formatter for wrap errors chains.
func (eLink *ErrLink) formatStr(format StringFormat) string {
	kvs := ""
	if len(eLink.kvs) > 0 {
		kvs = fmt.Sprintf(" KVs(%v)", eLink.kvs)
	}
	str := fmt.Sprintf("code(%s)%s %s%s", eLink.code.String(), kvs, eLink.Msg, format.MsgStackSep)
	if format.Options.WithTrace {
		str += format.PreStackSep + eLink.Frame.format(format.StackElemSep)
	}
	return str
}

// JSON formatter for wrap error chains.
func (eLink *ErrLink) formatJSON(format JSONFormat) map[string]any {
	wrapMap := make(map[string]any)
	wrapMap["code"] = eLink.code.String()
	wrapMap["message"] = fmt.Sprint(eLink.Msg)
	if eLink.HasKVs() {
		wrapMap["KVs"] = eLink.kvs
	}
	if format.Options.WithTrace {
		wrapMap["stack"] = eLink.Frame.format(format.StackElemSep)
	}
	return wrapMap
}
