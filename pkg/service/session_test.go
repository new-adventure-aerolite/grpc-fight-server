package service

import (
	"testing"

	"github.com/TianqiuHuang/grpc-fight-app/pkg/module"
)

func TestDelete(t *testing.T) {
	err := sessionStore.Add("1", &module.SessionView{})
	if err != nil {
		t.Errorf(err.Error())
	}
	sessionStore.Remove("1")
	_, err = sessionStore.Get("1")
	if err != ErrorNotFound {
		t.Errorf("want ErrorNotFound, but get: '%v'", err)
	}
}
