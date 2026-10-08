package sample

func atomic() {
	message := "one indivisible string literal that exceeds the configured line width and should never be split or rewritten by the formatter"

	// A long explanatory comment deliberately exceeds the preferred width and must preserve its complete wording.
	log.Info(message)

	raw := `log.Info("not code")
 if invalid { still raw }
`
	_ = raw
}
