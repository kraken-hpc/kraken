package core

import (
	_ "github.com/hpc/kraken/core"
)

/* FIXME: broken test
func TestGenerateKey(t *testing.T) {
	ctx := Context{}
	sse := NewStateSyncEngine(ctx)
	key1 := sse.generateKey()
	key2 := sse.generateKey()
	fmt.Println(hex.Dump(key1))
	fmt.Println(hex.Dump(key2))
	if bytes.Equal(key1, key2) {
		t.Errorf("generated identical keys!")
	}
}
*/
