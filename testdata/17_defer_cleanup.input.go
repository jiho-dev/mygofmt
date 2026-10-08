package sample
func cleanup() {
 defer func() { cleanupResources() }()
 prepare()
}
