package controller

import (
	"os"
	"reflect"
	"testing"
)

func TestCabCallCache_FlushThenRestore(t *testing.T) {
	requests := [][3]bool{{true, false, false}, {false, true, true}, {true, true, true}}
	flushRequests(requests)
	result := restoreRequests(3)
	expected := [][3]bool{{false, false, false}, {false, false, true}, {false, false, true}}

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Restored requests not as expected.\nExpected: %+v\nWas: %+v", expected, result)
	}
}

func TestCabCallCache_RestoreOnly(t *testing.T) {
	file, _ := os.OpenFile(hallCallStatePath, os.O_CREATE|os.O_WRONLY, 0644)
	defer file.Close()

	hallCalls := [...]bool{true, false, true}

	for _, val := range hallCalls {
		if val {
			file.WriteString("1")
		} else {
			file.WriteString("0")
		}
	}

	result := restoreRequests(3)
	expected := [][3]bool{{false, false, true}, {false, false, false}, {false, false, true}}

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Restored requests not as expected.\nExpected: %+v\nWas: %+v", expected, result)
	}
}
