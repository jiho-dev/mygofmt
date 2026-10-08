package sample
func clauses() {
 switch phase {
 case "start":
  log.Info("start")
 case "end":
  log.Info("end")
 }
 select {
 case <-done:
  log.Info("done")
 default:
  log.Info("waiting")
 }
}
