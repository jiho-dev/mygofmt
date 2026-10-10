package sample

// Allocate keeps its description.
func Allocate(
	request ExampleServiceRequest,
	processor ProcessorClient,
	configuration ExampleServiceConfiguration,
) (ExampleServiceResponse, error) {

	return processor.Allocate(request, configuration)
}
func short(id string) error { return nil }
