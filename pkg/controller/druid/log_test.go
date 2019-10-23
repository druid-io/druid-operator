package stub

import (
	"reflect"
	"testing"
)

func TestLogInitialization(t *testing.T) {
	logger := getLogger()
	if reflect.ValueOf(logger).IsNil() {
		t.Error("logger is NIL")
	}

	logger.Info("I am created successfully.")
}
