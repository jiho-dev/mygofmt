package sample
func callError() error {
 result, err := fetch()

 if err != nil { return err }
 _ = result
 return nil
}
func documentedError() error {
 result, err := fetch()

 // Preserve the comment with its guard.
 if err != nil { return err }
 _ = result
 return nil
}
