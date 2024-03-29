package eris

import (
	"net/http"

	grpc "google.golang.org/grpc/codes"
)

// Code is an error code that indicates the category of error.
type Code int

// These are common impact error codes that are found throughout our services.
const (
	// The operation was cancelled, typically by the caller.
	CodeCanceled Code = iota + 1
	// Unknown error. For example, this error may be returned when a Status value received from another address space belongs to an error space that is not known in this address space. Also errors raised by APIs that do not return enough error information may be converted to this error.
	CodeUnknown
	// The client specified an invalid argument. Note that this differs from FAILED_PRECONDITION. INVALID_ARGUMENT indicates arguments that are problematic regardless of the state of the system (e.g., a malformed file name).
	CodeInvalidArgument
	// The deadline expired before the operation could complete. For operations that change the state of the system, this error may be returned even if the operation has completed successfully. For example, a successful response from a server could have been delayed long.
	CodeDeadlineExceeded
	// Some requested entity (e.g., file or directory) was not found. Note to server developers: if a request is denied for an entire class of users, such as gradual feature rollout or undocumented allowlist, NOT_FOUND may be used. If a request is denied for some users within a class of users, such as user-based access control, PERMISSION_DENIED must be used.
	CodeNotFound
	// The entity that a client attempted to create (e.g., file or directory) already exists.
	CodeAlreadyExists
	// The caller does not have permission to execute the specified operation. PERMISSION_DENIED must not be used for rejections caused by exhausting some resource (use RESOURCE_EXHAUSTED instead for those errors). PERMISSION_DENIED must not be used if the caller can not be identified (use UNAUTHENTICATED instead for those errors). This error code does not imply the request is valid or the requested entity exists or satisfies other pre-conditions.
	CodePermissionDenied
	// Some resource has been exhausted, perhaps a per-user quota, or perhaps the entire file system is out of space.
	CodeResourceExhausted
	// The operation was rejected because the system is not in a state required for the operation's execution. For example, the directory to be deleted is non-empty, an rmdir operation is applied to a non-directory, etc. Service implementors can use the following guidelines to decide between FAILED_PRECONDITION, ABORTED, and UNAVAILABLE: (a) Use UNAVAILABLE if the client can retry just the failing call. (b) Use ABORTED if the client should retry at a higher level (e.g., when a client-specified test-and-set fails, indicating the client should restart a read-modify-write sequence). (c) Use FAILED_PRECONDITION if the client should not retry until the system state has been explicitly fixed. E.g., if an "rmdir" fails because the directory is non-empty, FAILED_PRECONDITION should be returned since the client should not retry unless the files are deleted from the directory.
	CodeFailedPrecondition
	// The operation was aborted, typically due to a concurrency issue such as a sequencer check failure or transaction abort. See the guidelines above for deciding between FAILED_PRECONDITION, ABORTED, and UNAVAILABLE.
	CodeAborted
	// The operation was attempted past the valid range. E.g., seeking or reading past end-of-file. Unlike INVALID_ARGUMENT, this error indicates a problem that may be fixed if the system state changes. For example, a 32-bit file system will generate INVALID_ARGUMENT if asked to read at an offset that is not in the range [0,2^32-1], but it will generate OUT_OF_RANGE if asked to read from an offset past the current file size. There is a fair bit of overlap between FAILED_PRECONDITION and OUT_OF_RANGE. We recommend using OUT_OF_RANGE (the more specific error) when it applies so that callers who are iterating through a space can easily look for an OUT_OF_RANGE error to detect when they are done.
	CodeOutOfRange
	// The operation is not implemented or is not supported/enabled in this service.
	CodeUnimplemented
	// Internal errors. This means that some invariants expected by the underlying system have been broken. This error code is reserved for serious errors.
	CodeInternal
	// The service is currently unavailable. This is most likely a transient condition, which can be corrected by retrying with a backoff. Note that it is not always safe to retry non-idempotent operations.
	CodeUnavailable
	// Unrecoverable data loss or corruption.
	CodeDataLoss
	// The request does not have valid authentication credentials for the operation.
	CodeUnauthenticated
)

func (c Code) String() string {
	if s, ok := defaultErrorCodes[c]; ok {
		return s
	}
	return defaultErrorCodes[DEFAULT_ERROR_CODE_NEW]
}

var defaultErrorCodes = map[Code]string{
	CodeAborted:            "aborted",
	CodeAlreadyExists:      "already exists",
	CodeCanceled:           "canceled",
	CodeDataLoss:           "data loss",
	CodeDeadlineExceeded:   "deadline exceeded",
	CodeFailedPrecondition: "failed precondition",
	CodeInternal:           "internal",
	CodeInvalidArgument:    "invalid argument",
	CodeNotFound:           "not found",
	CodeOutOfRange:         "out of range",
	CodePermissionDenied:   "permission denied",
	CodeResourceExhausted:  "resource exhausted",
	CodeUnauthenticated:    "unauthenticated",
	CodeUnavailable:        "unavailable",
	CodeUnknown:            "unknown",
	CodeUnimplemented:      "unimplemented",
}

const (
	// Default error code assigned when using eris.New.
	DEFAULT_ERROR_CODE_NEW = CodeUnknown
	// Default error code assigned when using eris.Wrap or Wrapf.
	DEFAULT_ERROR_CODE_WRAP = CodeInternal
	// Fallback code when you cannot determine what code it is.
	DEFAULT_UNKNOWN_CODE = CodeUnknown
)

// fromGrpc converts a grpc code to an eris code. Returns false if mapping failed.
func fromGrpc(c grpc.Code) (Code, bool) {
	if c == grpc.OK {
		return DEFAULT_UNKNOWN_CODE, false
	}
	if resultCode, ok := map[grpc.Code]Code{
		grpc.Aborted:            CodeAborted,
		grpc.AlreadyExists:      CodeAlreadyExists,
		grpc.Canceled:           CodeCanceled,
		grpc.DataLoss:           CodeDataLoss,
		grpc.DeadlineExceeded:   CodeDeadlineExceeded,
		grpc.FailedPrecondition: CodeFailedPrecondition,
		grpc.Internal:           CodeInternal,
		grpc.InvalidArgument:    CodeInvalidArgument,
		grpc.NotFound:           CodeNotFound,
		grpc.OutOfRange:         CodeOutOfRange,
		grpc.PermissionDenied:   CodePermissionDenied,
		grpc.ResourceExhausted:  CodeResourceExhausted,
		grpc.Unauthenticated:    CodeUnauthenticated,
		grpc.Unavailable:        CodeUnavailable,
		grpc.Unknown:            CodeUnknown,
		grpc.Unimplemented:      CodeUnimplemented,
	}[c]; ok {
		return resultCode, true
	}
	return DEFAULT_UNKNOWN_CODE, true
}

// ToGrpc converts an eris code to a grpc code.
func (c Code) ToGrpc() grpc.Code {
	if grpcCode, ok := map[Code]grpc.Code{
		CodeAborted:            grpc.Aborted,
		CodeAlreadyExists:      grpc.AlreadyExists,
		CodeCanceled:           grpc.Canceled,
		CodeDataLoss:           grpc.DataLoss,
		CodeDeadlineExceeded:   grpc.DeadlineExceeded,
		CodeFailedPrecondition: grpc.FailedPrecondition,
		CodeInternal:           grpc.Internal,
		CodeInvalidArgument:    grpc.InvalidArgument,
		CodeNotFound:           grpc.NotFound,
		CodeOutOfRange:         grpc.OutOfRange,
		CodePermissionDenied:   grpc.PermissionDenied,
		CodeResourceExhausted:  grpc.ResourceExhausted,
		CodeUnauthenticated:    grpc.Unauthenticated,
		CodeUnavailable:        grpc.Unavailable,
		CodeUnknown:            grpc.Unknown,
		CodeUnimplemented:      grpc.Unimplemented,
	}[c]; ok {
		return grpcCode
	}
	return grpc.Unknown
}

// We do not provide a FromHttp method, since many http codes, would correlate to a grpc code OK
// TODO Write a test that asserts that

// HTTPStatus is http status code.
type HTTPStatus int

// fromHttp converts a http code to an eris code. Returns false if mapping failed.
func fromHttp(code HTTPStatus) (Code, bool) {
	// mapping according to https://github.com/lobocv/simplerr/blob/master/ecosystem/http/translate_error_code.go
	if code == 200 {
		return DEFAULT_UNKNOWN_CODE, false
	}
	if c, ok := map[HTTPStatus]Code{
		http.StatusInternalServerError: CodeUnknown,
		http.StatusNotFound:            CodeNotFound,
		http.StatusRequestTimeout:      CodeDeadlineExceeded,
		http.StatusForbidden:           CodePermissionDenied,
		http.StatusUnauthorized:        CodeUnauthenticated,
		http.StatusNotImplemented:      CodeUnimplemented,
		http.StatusBadRequest:          CodeInvalidArgument,
		http.StatusTooManyRequests:     CodeResourceExhausted,
	}[code]; ok {
		return c, true
	}
	return CodeUnknown, true
}

// ToHttp converts an eris code to a http code.
func (code Code) ToHttp() HTTPStatus {
	// mapping according to https://github.com/lobocv/simplerr/blob/master/ecosystem/http/translate_error_code.go
	if httpCode, ok := map[Code]HTTPStatus{
		CodeUnknown:           http.StatusInternalServerError,
		CodeNotFound:          http.StatusNotFound,
		CodeDeadlineExceeded:  http.StatusRequestTimeout,
		CodePermissionDenied:  http.StatusForbidden,
		CodeUnauthenticated:   http.StatusUnauthorized,
		CodeUnimplemented:     http.StatusNotImplemented,
		CodeInvalidArgument:   http.StatusBadRequest,
		CodeResourceExhausted: http.StatusTooManyRequests,
	}[code]; ok {
		return httpCode
	}

	// Default according to https://chromium.googlesource.com/external/github.com/grpc/grpc/+/refs/tags/v1.21.4-pre1/doc/statuscodes.md
	return http.StatusInternalServerError
}
