package sample
func comments() {
 send(request.TransactionId /* transaction */, request.PipelineId /* pipeline */, request.ActorId)
 response := Response{TransactionId: request.TransactionId /* transaction */, PipelineId: request.PipelineId}
 _ = response
}
