package sample

func invoke() {
	assignTask(
		request.TransactionId,
		request.PipelineId,
		request.ActorId,
		processor.Configuration(),
	)

	logger.Infof(
		"allocation complete: transaction=%s pipeline=%s actor=%s",
		request.TransactionId,
		request.PipelineId,
		request.ActorId,
	)

}
