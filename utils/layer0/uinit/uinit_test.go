package main

import (
	"fmt"
	"testing"
)

func TestKernArgs(t *testing.T) {
	kernArgs("test_cmdline.txt")
	if len(myID) < 1 {
		t.Error("failed to get myID")
	}
	fmt.Printf("myID: %s\n", myID)
	if len(myIP) < 1 {
		t.Error("failed to get myIP")
	}
	fmt.Printf("myIP: %s\n", myIP)
	if len(myNet) < 1 {
		t.Error("failed to get myNet")
	}
	fmt.Printf("myNet: %s\n", myNet)
	if len(myParent) < 1 {
		t.Error("failed to get myParent")
	}
	fmt.Printf("myParent: %s\n", myParent)
}
