package sample

func nested() {
	send(
		request.TransactionId,
		build(request.PipelineId, request.ActorId, request.Configuration),
		request.AdditionalArguments...,
	)
}
