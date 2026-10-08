package sample
func existing() {
 first := 1
 second := 2

 // Separate roles manually.
 third := use(first, second)

 value, err := call(third)
 if err != nil {
  return err
 }

 _ = value
}
