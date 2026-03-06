package handler

import "testing"

func TestBuildOnlyOfficeDocumentKey_UsesOnlyAllowedCharacters(t *testing.T) {
	key := buildOnlyOfficeDocumentKey("doc:01/abc", 2)
	if key != "doc_01_abc-v2" {
		t.Fatalf("expected normalized onlyoffice key, got %q", key)
	}
}

func TestMatchesOnlyOfficeDocumentKey_AcceptsLegacyShape(t *testing.T) {
	documentID := "01h1onlyofficekeytest0000000001"
	contentVersion := 7
	legacyKey := documentID + ":7"
	if !matchesOnlyOfficeDocumentKey(legacyKey, documentID, contentVersion) {
		t.Fatalf("expected legacy key %q still accepted", legacyKey)
	}
}
