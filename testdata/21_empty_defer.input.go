package sample
func emptyDefer() {
 defer func() {}()
 defer func() { /* reserved cleanup */ }()
}
