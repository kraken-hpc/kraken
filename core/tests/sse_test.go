package core

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"testing"

	. "github.com/hpc/kraken/core"
)

func TestGenerateKey(t *testing.T) {
	sse := NewStateSyncEngine(&QueryEngine{}, make(chan *EventListener))
	key1 := sse.generateKey()
	key2 := sse.generateKey()
	fmt.Println(hex.Dump(key1))
	fmt.Println(hex.Dump(key2))
	if bytes.Equal(key1, key2) {
		t.Errorf("generated identical keys!")
	}
}
