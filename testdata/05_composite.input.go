package sample
func result() Response {
 response := Response{TransactionId: request.TransactionId, PipelineId: request.PipelineId, ActorId: request.ActorId}
 return response
}
