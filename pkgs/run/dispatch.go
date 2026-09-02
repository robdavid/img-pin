package run

// SetRun allows the implementation of the [Run] method to be replaced by the
// the supplied function. This is intended for mocking purposes. It will return
// a cleanup function that must be called when mocking is complete. If called a
// second time before the first cleanup is called, the second cleanup should be
// called before the first. Calling the cleanup via defer immediately after
// this function is a recommended pattern.
func SetRun(run RunFunc) (cleanup func()) {
	orig := runDispatch
	runDispatch = run
	cleanup = func() { runDispatch = orig }
	return
}

type Wrapper = func(RunFunc) RunFunc

// WrapRun updates the [Run] dispatch function via the supplied wrapper. This is
// intended for mocking purposes. A cleanup function is returned that will cause
// the dispatch function to be reverted to its original value. This should be called
// as soon as mocking is complete. Calling the clean up via defer immediately after
// this function is a recommended pattern.
func WrapRun(wrapper Wrapper) (cleanup func()) {
	orig := runDispatch
	runDispatch = wrapper(orig)
	cleanup = func() { runDispatch = orig }
	return
}
