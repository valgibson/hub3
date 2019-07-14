package namespace

// Namespace errors.
const (
	ErrNameSpaceNotFound       = Error("namespace not found")
	ErrNameSpaceDuplicateEntry = Error("prefix and base stored in different entries")
	ErrNameSpaceNotValid       = Error("prefix or base not valid")
)

// Error represents a Namespace error.
type Error string

// Error returns the error message.
func (e Error) Error() string { return string(e) }
