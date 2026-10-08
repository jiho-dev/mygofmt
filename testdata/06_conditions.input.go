package sample
func validate() bool {
 if request.TransactionId != "" && request.PipelineId != "" && request.ActorId != "" && processor.IsReady() {
  return true
 }
 return false
}
