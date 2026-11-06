package output

// EmailToken is the delivery intent; the codec protects Token at rest.
type EmailToken struct {
	UserID string
	Email  string
	Name   string
	Token  string
}

// EmailTokenCodec owns the durable wire format and secret protection.
type EmailTokenCodec interface {
	Encode(EmailToken) ([]byte, error)
	Decode([]byte) (EmailToken, error)
}
