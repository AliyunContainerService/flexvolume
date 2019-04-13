package utils

import (
	"fmt"
	"github.com/Sirupsen/logrus"
)

func Errorf(format string, args ...interface{}){
	logrus.Errorf(format, args)
	fmt.Errorf(format, args)
}

func Warnf(format string, args ...interface{}){
	logrus.Warnf(format, args)
	fmt.Printf(format, args)
}

func Infof(format string, args ...interface{}){
	logrus.Infof(format, args)
	fmt.Printf(format, args)
}

func Printf(format string, args ...interface{}){
	logrus.Printf(format, args)
	fmt.Printf(format, args)
}