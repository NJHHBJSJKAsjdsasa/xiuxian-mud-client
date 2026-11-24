//go:build windows

package main

import "testing"

func TestProtectSecretRoundTrip(t *testing.T) {
	enc, err := protectSecret("test123456")
	if err != nil {
		t.Fatalf("加密失败：%v", err)
	}
	if enc == "" || enc == "test123456" {
		t.Fatalf("密文不应为空或明文：%q", enc)
	}
	got, err := unprotectSecret(enc)
	if err != nil {
		t.Fatalf("解密失败：%v", err)
	}
	if got != "test123456" {
		t.Fatalf("解密结果 = %q", got)
	}
}

func TestMergeAccountRecentFirst(t *testing.T) {
	old := []savedAccount{
		{Name: "甲", Server: "s1"},
		{Name: "乙", Server: "s1"},
		{Name: "丙", Server: "s1"},
	}
	got := mergeAccount(old, savedAccount{Name: "乙", Server: "s2"})
	if len(got) != 3 || got[0].Name != "乙" || got[0].Server != "s2" {
		t.Fatalf("应把最近账号置顶并去重：%+v", got)
	}
	old = nil
	for i := 0; i < 15; i++ {
		old = append(old, savedAccount{Name: string(rune('A' + i))})
	}
	got = mergeAccount(old, savedAccount{Name: "新"})
	if len(got) != 12 || got[0].Name != "新" {
		t.Fatalf("账号上限应为 12 且新账号置顶：len=%d", len(got))
	}
}
