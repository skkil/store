package reference

import _ "embed"

// Profile is the conformance mutation profile of the reference product.
//
//go:embed profile.json
var Profile []byte
