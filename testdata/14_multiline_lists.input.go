package sample
func multiline() {
 send(
  request.VeryLongTransactionIdentifier, request.VeryLongPipelineIdentifier, request.VeryLongActorIdentifier,
 )
 response := Response{
  TransactionId: request.VeryLongTransactionIdentifier, PipelineId: request.VeryLongPipelineIdentifier,
 }
 _ = response
}
