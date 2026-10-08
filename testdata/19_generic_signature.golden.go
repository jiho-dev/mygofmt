package sample

func Generic[
	T interface{ ~string | ~int },
](
	request ExampleServiceRequest,
	processor ProcessorClient,
	configuration ExampleServiceConfiguration,
) T {
	return processor.Value(request, configuration)
}
