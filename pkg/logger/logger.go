package logger

import (
	"github.com/sirupsen/logrus"
)

// Init initializes the structured logger.
func Init() {
	logrus.SetFormatter(&logrus.JSONFormatter{})
	logrus.SetLevel(logrus.InfoLevel)
	logrus.Info("Logger initialized")
}