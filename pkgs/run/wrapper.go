package run

type Wrapper = func(RunFunc) RunFunc

func WrapRun(wrapper Wrapper) (cleanup func()) {
	orig := runDispatch
	runDispatch = wrapper(orig)
	cleanup = func() { runDispatch = orig }
	return
}
