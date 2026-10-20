package keys

import "encoding/base64"

func decode(s string) ([]byte, error) { return base64.RawURLEncoding.DecodeString(s) }
