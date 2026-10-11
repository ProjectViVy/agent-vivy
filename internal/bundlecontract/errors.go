package bundlecontract

// Error exposes a bounded diagnostic without including private content.
type Error struct {
	Code  string `json:"code"`
	Field string `json:"field,omitempty"`
	Cause error  `json:"-"`
}

func (e *Error) Error() string   { return "toolkits: " + e.Code + " (" + e.Field + ")" }
func (e *Error) Unwrap() error   { return e.Cause }
func Invalid(field string) error { return &Error{Code: "invalid_manifest", Field: field} }
