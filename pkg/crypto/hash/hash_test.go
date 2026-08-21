package hash

import (
	"testing"
)

func TestGeneratePassword(t *testing.T) {
	origin := "abc-123"
	hashPwd, err := GeneratePassword(origin)
	if err != nil {
		t.Error("GeneratePassword Failed: ", err.Error())
	}
	t.Log("test password: ", hashPwd, ",length: ", len(hashPwd))

	if err := CompareHashAndPassword(hashPwd, origin); err != nil {
		t.Error("Unmatched password: ", err.Error())
	}
}
