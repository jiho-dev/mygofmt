package sample

func emit(ok bool) {
	if ok {
		log.Info("ready")
	}

	for ok {
		logger.Debug("waiting")

		break
	}

	defer func() {
		fl.EndError("err", err)
	}()

	log.Infof("finished")
}
