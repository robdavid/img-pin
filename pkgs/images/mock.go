package images

import "time"

// DigestFunc is the type of the internal function responsible for computing image digests.
type DigestFunc = func(image string, opts *ImageOptions) (digested string, digest string, created time.Time, err error)

var digestDispatch DigestFunc = digestImage

type Testable interface {
	Cleanup(func())
}

// MockDigest allows a test to provide a mocked version of the internal digest func
// It should be called with a *testing.T parameter along with the mock function.
// The original function is restored via t.Cleanup.
func MockDigest(t Testable, digestFunc DigestFunc) {
	orig := digestDispatch
	digestDispatch = digestFunc
	t.Cleanup(func() { digestDispatch = orig })
}
