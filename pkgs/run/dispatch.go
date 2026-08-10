package run

func SetRun(run RunFunc) (cleanup func()) {
	orig := runDispatch
	runDispatch = run
	cleanup = func() { runDispatch = orig }
	return
}

type Wrapper = func(RunFunc) RunFunc

// WrapRun updates the [Run] dispatch function via the supplied wrapper. A cleanup
// function is returned that will cause the dispatch function to be reverted to
// its original value.
func WrapRun(wrapper Wrapper) (cleanup func()) {
	orig := runDispatch
	runDispatch = wrapper(orig)
	cleanup = func() { runDispatch = orig }
	return
}
