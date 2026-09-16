package main

import "testing"

func TestRejectWildcard(t *testing.T) {
	if err := rejectWildcard("0.0.0.0:80"); err == nil {
		t.Fatal("expected reject")
	}
	if err := rejectWildcard("127.0.0.1:8740"); err != nil {
		t.Fatal(err)
	}
}
