package main

import (
	"testing"

	"go.uber.org/zap"
)

func Test(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	defer logger.Sync() // flushes buffer, if any
	sugar := logger.Sugar()

	sugar.Infof("Starting Test...")
}
