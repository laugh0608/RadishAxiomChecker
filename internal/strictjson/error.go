package strictjson

import "radishaxiom.dev/independent-checker-go/internal/rejection"

type Code = rejection.Code

const (
	CodeDuplicateMember   = rejection.DuplicateMember
	CodeInvalidJSON       = rejection.InvalidJSON
	CodeInvalidUTF8       = rejection.InvalidUTF8
	CodeJSONNumberOrNull  = rejection.JSONNumberOrNull
	CodeNoncanonicalJSON  = rejection.NoncanonicalJSON
	CodeNoncanonicalOrder = rejection.NoncanonicalOrder
	CodeResourceLimit     = rejection.ResourceLimit
)

type Error = rejection.Error

func reject(code Code, offset int, detail string) error {
	return rejection.At(code, offset, detail)
}
