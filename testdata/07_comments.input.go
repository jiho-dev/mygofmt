package sample
// -----------------------------------------------------------------------------
// Shared helpers
// -----------------------------------------------------------------------------
// emit preserves explanatory comments.
func emit() {
 prepare() // prepare the request
 // The next event describes completion.
 log.Info("done") // keep this attached
 // This comment describes the return.
 return
}
