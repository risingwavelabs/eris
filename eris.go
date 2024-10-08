// Package eris is an error handling library with readable stack traces and flexible formatting support. We also support error codes
package eris

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"

	grpc "google.golang.org/grpc/codes"
)

type statusError interface {
	error
	WithCode(Code) statusError
	WithCodeGrpc(grpc.Code) statusError
	WithCodeHttp(HTTPStatus) statusError
	WithProperty(string, any) statusError
	Code() Code
	HasKVs() bool
	KVs() map[string]any
}

// GetCode returns the error code. Defaults to unknown, if error does not have code.
func GetCode(err error) Code {
	type Coder interface {
		Code() Code
	}
	codeErr, ok := err.(Coder)
	if !ok {
		return DEFAULT_UNKNOWN_CODE
	}
	return codeErr.Code()
}

// GetKVs returns the error code. Returns nil if error doesn't support kvs.
func GetKVs(err error) map[string]any {
	type KVer interface {
		KVs() map[string]any
	}
	kvErr, ok := err.(KVer)
	if !ok {
		return nil
	}
	return kvErr.KVs()
}

// GetProperty returns the property. If the property doesn't exist or type doesn't match, returns T{}, false.
func GetProperty[T any](err error, key string) (T, bool) {
	val, ok := GetKVs(err)[key]
	if !ok {
		var empty T
		return empty, false
	}
	typed, ok := val.(T)
	if !ok {
		var empty T
		return empty, false
	}
	return typed, true
}

// GetPropertyP returns the property pointer. If the property doesn't exist or type doesn't match, returns nil.
func GetPropertyP[T any](err error, key string) *T {
	val, ok := GetKVs(err)[key]
	if !ok {
		return nil
	}
	typed, ok := val.(T)
	if !ok {
		return nil
	}
	return &typed
}

// New creates a new root error with a static message and an error code 'unknown'.
func New(msg string) statusError {
	stack := callers(3) // callers(3) skips this method, stack.callers, and runtime.Callers
	return &rootError{
		global: stack.isGlobal(),
		msg:    msg,
		stack:  stack,
		code:   DEFAULT_ERROR_CODE_NEW,
	}
}

// Errorf creates a new root error with a formatted message and an error code 'unknown'.
func Errorf(format string, args ...any) statusError {
	stack := callers(3)
	return &rootError{
		global: stack.isGlobal(),
		msg:    fmt.Sprintf(format, args...),
		stack:  stack,
		code:   DEFAULT_ERROR_CODE_NEW,
	}
}

type joinError interface {
	Unwrap() []error
}

// Join returns an error that wraps the given errors.
func Join(errs ...error) error {
	internal := errors.Join(errs...)
	if internal == nil {
		return nil
	}
	return wrap(internal, "join error", DEFAULT_ERROR_CODE_NEW)
}

// Wrap adds additional context to all error types while maintaining the type of the original error. Adds a default error code 'internal'
//
// This method behaves differently for each error type. For root errors, the stack trace is reset to the current
// callers which ensures traces are correct when using global/sentinel error values. Wrapped error types are simply
// wrapped with the new context. For external types (i.e. something other than root or wrap errors), this method
// attempts to unwrap them while building a new error chain. If an external type does not implement the unwrap
// interface, it flattens the error and creates a new root error from it before wrapping with the additional
// context.
func Wrap(err error, msg string) error {
	return wrap(err, fmt.Sprint(msg), DEFAULT_ERROR_CODE_WRAP)
}

// Wrapf adds additional context to all error types while maintaining the type of the original error. Adds a default error code 'internal'
//
// This is a convenience method for wrapping errors with formatted messages and is otherwise the same as Wrap.
func Wrapf(err error, format string, args ...any) error {
	return wrap(err, fmt.Sprintf(format, args...), DEFAULT_ERROR_CODE_WRAP)
}

// PassThrough adds additional context to all error types while maintaining the type of the original error.
//
// This method behaves like Wrap but will copy the code and properties from underlying error.
func PassThrough(err error, msg string) error {
	return PassThroughf(err, fmt.Sprint(msg))
}

// PassThroughf adds additional context to all error types while maintaining the type of the original error.
//
// This is a convenience method for wrapping errors with formatted messages and is otherwise the same as PassThrough.
func PassThroughf(err error, format string, args ...any) error {
	if err == nil {
		return nil
	}
	newErr := wrap(err, fmt.Sprintf(format, args...), DEFAULT_ERROR_CODE_WRAP)

	code := GetCode(err)
	if code != CodeUnknown {
		newErr = WithCode(newErr, code)
	}
	kvs := GetKVs(err)
	for k, v := range kvs {
		newErr = WithProperty(newErr, k, v)
	}
	return newErr
}

func wrap(err error, msg string, code Code) error {
	if err == nil {
		return nil
	}

	// callers(4) skips runtime.Callers, stack.callers, this method, and Wrap(f)
	stack := callers(4)
	// caller(3) skips stack.caller, this method, and Wrap(f)
	// caller(skip) has a slightly different meaning which is why it's not 4 as above
	frame := caller(3)
	switch e := err.(type) {
	case *rootError:
		if e.global {
			// create a new root error for global values to make sure nothing interferes with the stack
			err = &rootError{
				global: e.global,
				msg:    e.msg,
				stack:  stack,
				code:   e.code,
			}
		} else {
			// insert the frame into the stack
			e.stack.insertPC(*stack)
		}
	case *wrapError:
		// insert the frame into the stack
		if root, ok := Cause(err).(*rootError); ok {
			root.stack.insertPC(*stack)
		}
	default:
		// return a new root error that wraps the external error
		return &rootError{
			msg:   msg,
			ext:   e,
			stack: stack,
			code:  code,
		}
	}

	return &wrapError{
		msg:   msg,
		err:   err,
		frame: frame,
		code:  code,
	}
}

// Unwrap returns the result of calling the Unwrap method on err, if err's type contains an Unwrap method
// returning error. Otherwise, Unwrap returns nil.
func Unwrap(err error) error {
	u, ok := err.(interface {
		Unwrap() error
	})
	if !ok {
		return nil
	}
	return u.Unwrap()
}

// Is reports whether any error in err's chain matches target.
//
// The chain consists of err itself followed by the sequence of errors obtained by repeatedly calling Unwrap.
//
// An error is considered to match a target if it is equal to that target or if it implements a method
// Is(error) bool such that Is(target) returns true.
func Is(err, target error) bool {
	if target == nil {
		return err == target
	}

	isComparable := reflect.TypeOf(target).Comparable()
	for {
		if isComparable && err == target {
			return true
		}
		if x, ok := err.(interface{ Is(error) bool }); ok && x.Is(target) {
			return true
		}
		if err = Unwrap(err); err == nil {
			return false
		}
	}
}

// As finds the first error in err's chain that matches target. If there's a match, it sets target to that error
// value and returns true. Otherwise, it returns false.
//
// The chain consists of err itself followed by the sequence of errors obtained by repeatedly calling Unwrap.
//
// An error matches target if the error's concrete value is assignable to the value pointed to by target,
// or if the error has a method As(any) bool such that As(target) returns true.
func As(err error, target any) bool {
	if target == nil || err == nil {
		return false
	}
	val := reflect.ValueOf(target)
	typ := val.Type()

	// target must be a non-nil pointer
	if typ.Kind() != reflect.Ptr || val.IsNil() {
		return false
	}

	// *target must be interface or implement error
	if e := typ.Elem(); e.Kind() != reflect.Interface && !e.Implements(reflect.TypeOf((*error)(nil)).Elem()) {
		return false
	}

	for {
		errType := reflect.TypeOf(err)
		if errType != reflect.TypeOf(&wrapError{}) && errType != reflect.TypeOf(&rootError{}) && reflect.TypeOf(err).AssignableTo(typ.Elem()) {
			val.Elem().Set(reflect.ValueOf(err))
			return true
		}
		if x, ok := err.(interface{ As(any) bool }); ok && x.As(target) {
			return true
		}
		if err = Unwrap(err); err == nil {
			return false
		}
	}
}

// Cause returns the root cause of the error, which is defined as the first error in the chain. The original
// error is returned if it does not implement `Unwrap() error` and nil is returned if the error is nil.
func Cause(err error) error {
	for {
		uerr := Unwrap(err)
		if uerr == nil {
			return err
		}
		err = uerr
	}
}

// StackFrames returns the trace of an error in the form of a program counter slice.
// Use this method if you want to pass the eris stack trace to some other error tracing library.
func StackFrames(err error) []uintptr {
	for err != nil {
		switch err := err.(type) {
		case *rootError:
			return err.StackFrames()
		case *wrapError:
			return err.StackFrames()
		default:
			return []uintptr{}
		}
	}
	return []uintptr{}
}

// With attach additional fields for an error.
func With(err error, fields ...Field) error {
	if err == nil {
		return nil
	}

	if root, ok := err.(*rootError); ok {
		for _, field := range fields {
			root = root.WithField(field).(*rootError)
		}
		return root
	} else if wrap, ok := err.(*wrapError); ok {
		for _, field := range fields {
			wrap = wrap.WithField(field).(*wrapError)
		}
		return wrap
	} else {
		return With(Wrap(err, "with property"), fields...)
	}
}

// WithCode attach an error code for an error.
func WithCode(err error, code Code) error {
	return With(err, Codes(code))
}

// WithProperty attach an additional key-value property for an error.
func WithProperty(err error, key string, value any) error {
	return With(err, KVs(key, value))
}

// FieldType type of field.
type FieldType uint8

const (
	// UnknownType unknown, should never be used.
	UnknownType FieldType = iota
	// CodeType the field type is a code.
	CodeType
	// KVType the field type is a key-value.
	KVType
)

// Field is the additional property an error could be attached.
type Field struct {
	Type  FieldType
	Key   string
	Value any
}

// Codes returns a Field of CodeType.
func Codes(code Code) Field {
	return Field{
		Type:  CodeType,
		Value: code,
	}
}

// KVs returns a Field of KVType.
func KVs(key string, value any) Field {
	return Field{
		Type:  KVType,
		Key:   key,
		Value: value,
	}
}

type rootError struct {
	global bool   // flag indicating whether the error was declared globally
	msg    string // root error message
	ext    error  // error type for wrapping external errors
	stack  *stack // root error stack trace
	code   Code
	kvs    map[string]any
}

// KVs returns the key-value pairs associated with the error.
func (e *rootError) KVs() map[string]any {
	if e.kvs == nil {
		return make(map[string]any)
	}
	return e.kvs
}

// WithCode sets the error code.
func (e *rootError) WithCode(code Code) statusError {
	e.code = code
	return e
}

// TODO: also do this for other errors
// add this function to interface

// WithCodeGrpc sets the error code, based on an GRPC error code.
func (e *rootError) WithCodeGrpc(code grpc.Code) statusError {
	if code == grpc.OK {
		return e
	}
	e.code, _ = fromGrpc(code)
	return e
}

// WithCodeHttp sets the error code, based on an HTTP status code.
func (e *rootError) WithCodeHttp(code HTTPStatus) statusError {
	if code == http.StatusOK {
		return e
	}
	e.code, _ = fromHttp(code)
	return e
}

// WithProperty adds a key-value pair to the error.
func (e *rootError) WithProperty(key string, value any) statusError {
	if e.kvs == nil {
		e.kvs = make(map[string]any)
	}
	e.kvs[key] = value
	return e
}

// WithField adds a key-value pair to the error.
func (e *rootError) WithField(field Field) statusError {
	if field.Type == CodeType {
		return e.WithCode(field.Value.(Code))
	} else if field.Type == KVType {
		return e.WithProperty(field.Key, field.Value)
	}
	return e
}

// Code returns the error code.
func (e *rootError) Code() Code {
	return e.code
}

// HasKVs returns true if the error has key-value pairs.
func (e *rootError) HasKVs() bool {
	return e.kvs != nil && len(e.kvs) > 0
}

// GetKVs returns the key-value pairs associated with the error.
func (e *rootError) GetKVs() map[string]any {
	return e.kvs
}

func (e *rootError) Error() string {
	return fmt.Sprint(e)
}

// Format pretty prints the error.
func (e *rootError) Format(s fmt.State, verb rune) {
	printError(e, s, verb)
}

// Is returns true if both errors have the same message and code.
// In case of a joined error, returns true if at least one of the joined errors is equal to target.
// Ignores additional KV pairs.
func (e *rootError) Is(target error) bool {
	if joinErr, ok := e.ext.(joinError); ok {
		for _, err := range joinErr.Unwrap() {
			if Is(err, target) {
				return true
			}
		}
		return false
	}
	if err, ok := target.(*rootError); ok {
		return e.msg == err.msg && e.code == err.code && reflect.DeepEqual(e.kvs, err.kvs)
	}
	if err, ok := target.(*wrapError); ok {
		return e.msg == err.msg && e.code == err.code && reflect.DeepEqual(e.kvs, err.kvs)
	}
	return e.msg == target.Error() && e.code == DEFAULT_UNKNOWN_CODE
}

// As returns true if the error message in the target error is equivalent to the error message in the root error.
func (e *rootError) As(target any) bool {
	if joinErr, ok := e.ext.(joinError); ok {
		for _, err := range joinErr.Unwrap() {
			if As(err, target) {
				return true
			}
		}
		return false
	}
	t := reflect.Indirect(reflect.ValueOf(target)).Interface()
	if err, ok := t.(*rootError); ok {
		if e.msg == err.msg {
			reflect.ValueOf(target).Elem().Set(reflect.ValueOf(err))
			return true
		}
	}
	return false
}

// Unwrap returns the contained error.
func (e *rootError) Unwrap() error {
	return e.ext
}

// StackFrames returns the trace of a root error in the form of a program counter slice.
// This method is currently called by an external error tracing library (Sentry).
func (e *rootError) StackFrames() []uintptr {
	return *e.stack
}

type wrapError struct {
	msg   string // wrap error message
	err   error  // error type representing the next error in the chain
	frame *frame // wrap error stack frame
	code  Code
	kvs   map[string]any
}

// KVs returns the key-value pairs associated with the error.
func (e *wrapError) KVs() map[string]any {
	if e.kvs == nil {
		return make(map[string]any)
	}
	return e.kvs
}

// WithCode sets the error code.
func (e *wrapError) WithCode(code Code) statusError {
	e.code = code
	return e
}

// WithCodeGrpc sets the error code, based on an GRPC error code.
func (e *wrapError) WithCodeGrpc(code grpc.Code) statusError {
	if code == grpc.OK {
		return e
	}
	e.code, _ = fromGrpc(code)
	return e
}

// WithCodeHttp sets the error code, based on an HTTP status code.
func (e *wrapError) WithCodeHttp(code HTTPStatus) statusError {
	if code == http.StatusOK {
		return e
	}
	e.code, _ = fromHttp(code)
	return e
}

// WithProperty adds a key-value pair to the error.
func (e *wrapError) WithProperty(key string, value any) statusError {
	if e.kvs == nil {
		e.kvs = make(map[string]any)
	}
	e.kvs[key] = value
	return e
}

// WithField adds a key-value pair to the error.
func (e *wrapError) WithField(field Field) statusError {
	if field.Type == CodeType {
		return e.WithCode(field.Value.(Code))
	} else if field.Type == KVType {
		return e.WithProperty(field.Key, field.Value)
	}
	return e
}

// Code returns the error code.
func (e *wrapError) Code() Code {
	return e.code
}

// HasKVs returns true if the error has key-value pairs.
func (e *wrapError) HasKVs() bool {
	return e.kvs != nil && len(e.kvs) > 0
}

// Error returns the error message.
func (e *wrapError) Error() string {
	return fmt.Sprint(e)
}

// Format pretty prints the error.
func (e *wrapError) Format(s fmt.State, verb rune) {
	printError(e, s, verb)
}

// Is returns true if error messages in both errors are equivalent.
func (e *wrapError) Is(target error) bool {
	if err, ok := target.(*rootError); ok {
		return e.msg == err.msg && e.code == err.code && reflect.DeepEqual(e.kvs, err.kvs)
	}
	if err, ok := target.(*wrapError); ok {
		return e.msg == err.msg && e.code == err.code && reflect.DeepEqual(e.kvs, err.kvs)
	}
	return e.msg == target.Error()
}

// As returns true if the error message in the target error is equivalent to the error message in the wrap error.
func (e *wrapError) As(target any) bool {
	t := reflect.Indirect(reflect.ValueOf(target)).Interface()
	if err, ok := t.(*wrapError); ok {
		if e.msg == err.msg {
			reflect.ValueOf(target).Elem().Set(reflect.ValueOf(err))
			return true
		}
	}
	return false
}

func (e *wrapError) Unwrap() error {
	return e.err
}

// StackFrames returns the trace of a wrap error in the form of a program counter slice.
// This method is currently called by an external error tracing library (Sentry).
func (e *wrapError) StackFrames() []uintptr {
	return []uintptr{e.frame.pc()}
}

func printError(err error, s fmt.State, verb rune) {
	var withTrace bool
	switch verb {
	case 'v':
		if s.Flag('+') {
			withTrace = true
		}
	}
	str := ToString(err, withTrace)
	_, _ = io.WriteString(s, str)
}
